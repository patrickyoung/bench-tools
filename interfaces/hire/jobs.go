package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const jobLogLimit = 8 << 20
const jobLogTail = 32 << 10
const jobCancelGrace = 3 * time.Second

var jobIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Job describes one public command invocation, not a scheduler or Agent session.
type Job struct {
	ID       string
	Kind     string
	Title    string
	Dir      string
	Args     []string
	Started  time.Time
	Finished time.Time
	State    string
	ExitCode *int
	Error    string
}

func (j Job) Active() bool { return j.State == "running" || j.State == "cancelling" }

type activeJob struct {
	id         string
	cmd        *exec.Cmd
	done       chan struct{}
	cancelling bool
	forced     bool
}

type jobManager struct {
	mu        sync.Mutex
	root      *os.Root
	owner     *os.Root
	lock      *os.File
	jobs      map[string]Job
	active    *activeJob
	closed    bool
	closeOnce sync.Once
	closeErr  error
	// A shorter grace is useful for deterministic subprocess tests.
	grace time.Duration
}

func newJobManager(root string) (*jobManager, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	base, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	success := false
	var lock *os.File
	defer func() {
		if !success {
			if lock != nil {
				lock.Close()
			}
			base.Close()
		}
	}()
	info, err := base.Stat(".")
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("job data must be a private directory (0700)")
	}
	// Keep one owner across recovery and execution. Never unlink this file:
	// replacing its inode would let another server acquire a different lock.
	lock, err = base.OpenFile("hire-ui.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("open interface lock: %w", err)
	}
	lockInfo, err := lock.Stat()
	if err != nil || !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm()&0077 != 0 {
		return nil, errors.New("interface lock must be a private regular file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, fmt.Errorf("data directory is already in use by another interface: %w", err)
	}
	if err := base.Mkdir("jobs", 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err = base.Lstat("jobs")
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("job storage must be a private real directory (0700)")
	}
	dir, err := base.OpenRoot("jobs")
	if err != nil {
		return nil, err
	}
	m := &jobManager{root: dir, owner: base, lock: lock, jobs: make(map[string]Job), grace: jobCancelGrace}
	if err := m.recover(); err != nil {
		dir.Close()
		return nil, err
	}
	success = true
	return m, nil
}

func (m *jobManager) recover() error {
	directory, err := m.root.Open(".")
	if err != nil {
		return err
	}
	entries, err := directory.ReadDir(10001)
	directory.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) > 10000 {
		return errors.New("job history exceeds 10000 entries")
	}
	for _, entry := range entries {
		if !entry.IsDir() || !jobIDPattern.MatchString(entry.Name()) {
			continue
		}
		directoryInfo, err := m.root.Lstat(entry.Name())
		if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("job %s must have a private real directory (0700)", entry.Name())
		}
		for _, stream := range []string{"stdout", "stderr"} {
			info, err := m.root.Lstat(filepath.Join(entry.Name(), stream))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
				return fmt.Errorf("job %s %s must be a private regular log", entry.Name(), stream)
			}
		}
		name := filepath.Join(entry.Name(), "job.json")
		info, err := m.root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 256<<10 {
			continue
		}
		if info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("job %s metadata must be private (0600)", entry.Name())
		}
		data, err := m.root.ReadFile(name)
		if err != nil {
			return err
		}
		var job Job
		if json.Unmarshal(data, &job) != nil || job.ID != entry.Name() {
			continue
		}
		if job.Active() {
			job.State = "unknown"
			job.Finished = time.Now().UTC()
			job.ExitCode = nil
			job.Error = "The interface stopped before the command outcome was recorded. Inspect its evidence before starting another build."
			if err := m.save(job); err != nil {
				return err
			}
		}
		m.jobs[job.ID] = job
	}
	return nil
}

func (m *jobManager) save(job Job) error {
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	name := filepath.Join(job.ID, "job.json")
	temp := name + ".tmp"
	f, err := m.root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		// A server interrupted between writing and renaming can leave this file.
		if err = m.root.Remove(temp); err == nil {
			f, err = m.root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		}
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		m.root.Remove(temp)
		return writeErr
	}
	if closeErr != nil {
		m.root.Remove(temp)
		return closeErr
	}
	return m.root.Rename(temp, name)
}

func copyJob(job Job) Job {
	job.Args = append([]string(nil), job.Args...)
	if job.ExitCode != nil {
		code := *job.ExitCode
		job.ExitCode = &code
	}
	return job
}

// Start executes literal argv once. Inputs are supplied by the HTTP adapter,
// which chooses permitted commands and the workspace before calling Start.
func (m *jobManager) Start(kind, title, dir string, argv []string, stdin string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Job{}, errors.New("job manager is closed")
	}
	if m.active != nil {
		return Job{}, errors.New("another command is already running")
	}
	if len(argv) == 0 || argv[0] == "" {
		return Job{}, errors.New("command is missing")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return Job{}, err
	}
	job := Job{ID: hex.EncodeToString(random), Kind: kind, Title: title, Dir: dir, Args: append([]string(nil), argv...), Started: time.Now().UTC(), State: "running"}
	if err := m.root.Mkdir(job.ID, 0700); err != nil {
		return Job{}, err
	}
	stdout, err := m.root.OpenFile(filepath.Join(job.ID, "stdout"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Job{}, err
	}
	stderr, err := m.root.OpenFile(filepath.Join(job.ID, "stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		stdout.Close()
		return Job{}, err
	}
	out := &cappedJobLog{file: stdout, remaining: jobLogLimit}
	errout := &cappedJobLog{file: stderr, remaining: jobLogLimit}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, strings.NewReader(stdin), out, errout
	// An independent process group permits bounded shutdown without touching
	// the interface's own process group. First cancellation only signals Hire,
	// allowing its existing public cancellation contract to reach Agent.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = m.grace
	if err := m.save(job); err != nil {
		stdout.Close()
		stderr.Close()
		return Job{}, err
	}
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		job.State, job.Error, job.Finished = "failed", err.Error(), time.Now().UTC()
		m.jobs[job.ID] = job
		if persistErr := m.save(job); persistErr != nil {
			return copyJob(job), errors.Join(err, persistErr)
		}
		return copyJob(job), err
	}
	active := &activeJob{id: job.ID, cmd: cmd, done: make(chan struct{})}
	m.active, m.jobs[job.ID] = active, job
	go m.wait(active, out, errout)
	return copyJob(job), nil
}

func (m *jobManager) wait(active *activeJob, stdout, stderr *cappedJobLog) {
	err := active.cmd.Wait()
	stdout.file.Close()
	stderr.file.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[active.id]
	job.Finished = time.Now().UTC()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
			if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				code = 128 + int(status.Signal())
			}
		} else {
			job.Error = err.Error()
			code = -1
		}
	}
	if code >= 0 {
		job.ExitCode = &code
	}
	switch {
	case active.forced || code < 0:
		job.State = "unknown"
		if active.forced {
			job.Error = "The command did not stop within the cancellation grace period and was killed. Inspect its evidence before retrying."
		}
	case code == 0:
		job.State = "completed"
	case code == 2:
		job.State = "unfinished"
	case code == 130 || code == 143:
		job.State = "cancelled"
	default:
		job.State = "failed"
	}
	if stdout.err != nil || stderr.err != nil {
		job.Error = strings.TrimSpace(job.Error + " Log recording failed: " + errors.Join(stdout.err, stderr.err).Error())
	}
	if err := m.save(job); err != nil {
		job.Error = strings.TrimSpace(job.Error + " Could not save outcome: " + err.Error())
	}
	m.jobs[job.ID] = job
	m.active = nil
	close(active.done)
}

func (m *jobManager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	return copyJob(job), ok
}

func (m *jobManager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, copyJob(job))
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Started.After(jobs[j].Started) })
	return jobs
}

func (m *jobManager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !jobIDPattern.MatchString(id) {
		return errors.New("invalid command ID")
	}
	active := m.active
	if active == nil || active.id != id {
		return errors.New("command is not running")
	}
	if active.cancelling {
		return nil
	}
	if err := active.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	active.cancelling = true
	job := m.jobs[id]
	job.State = "cancelling"
	m.jobs[id] = job
	saveErr := m.save(job)
	go func() {
		timer := time.NewTimer(m.grace)
		defer timer.Stop()
		select {
		case <-active.done:
			return
		case <-timer.C:
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.active == active {
				active.forced = true
				_ = syscall.Kill(-active.cmd.Process.Pid, syscall.SIGKILL)
			}
		}
	}()
	return saveErr
}

func (m *jobManager) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		active := m.active
		m.mu.Unlock()
		var err error
		if active != nil {
			err = m.Cancel(active.id)
			<-active.done
			// The child may have completed between the snapshot and Cancel.
			m.mu.Lock()
			if !active.cancelling {
				err = nil
			}
			m.mu.Unlock()
		}
		m.closeErr = errors.Join(err, m.root.Close(), m.lock.Close(), m.owner.Close())
	})
	return m.closeErr
}

// Log returns a bounded escaped-at-render-time tail. Only fixed stream names
// and server-generated IDs are accepted; Root prevents directory escape.
func (m *jobManager) Log(id, stream string) string {
	if !jobIDPattern.MatchString(id) || (stream != "stdout" && stream != "stderr") {
		return ""
	}
	if _, ok := m.Get(id); !ok {
		return ""
	}
	name := filepath.Join(id, stream)
	info, err := m.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return ""
	}
	file, err := m.root.Open(name)
	if err != nil {
		return ""
	}
	defer file.Close()
	prefix := ""
	if info.Size() > jobLogTail {
		if _, err := file.Seek(info.Size()-jobLogTail, io.SeekStart); err != nil {
			return ""
		}
		prefix = "[Earlier output omitted from this preview.]\n"
	}
	data, err := io.ReadAll(io.LimitReader(file, jobLogTail))
	if err != nil {
		return ""
	}
	return prefix + string(data)
}

type cappedJobLog struct {
	file      *os.File
	remaining int
	truncated bool
	err       error
}

func (w *cappedJobLog) Write(p []byte) (int, error) {
	original := len(p)
	if w.err != nil {
		return original, nil
	}
	if len(p) > w.remaining {
		if w.remaining > 0 {
			_, w.err = w.file.Write(p[:w.remaining])
			w.remaining = 0
		}
		if !w.truncated && w.err == nil {
			_, w.err = io.WriteString(w.file, "\n[Log limit reached; further output discarded.]\n")
			w.truncated = true
		}
	} else {
		_, w.err = w.file.Write(p)
		w.remaining -= len(p)
	}
	// Draining output after the cap preserves the child's stream and exit
	// contracts instead of failing a successful build with a broken pipe.
	return original, nil
}
