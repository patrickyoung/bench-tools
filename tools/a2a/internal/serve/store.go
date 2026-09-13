package serve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"github.com/patrickyoung/a2a/internal/boundary"
)

// Store adds atomic disk records and context ownership to the SDK's task store.
// The SDK retains ownership of task copying, updates, filtering, and pagination.
// Versions are local CAS tokens and are rebuilt when the exclusive owner starts.
type Store struct {
	mu       sync.Mutex
	inner    *taskstore.InMemory
	root     string
	lock     *os.File
	contexts map[string]string
	limit    int64
	fault    error
}

type diskTask struct {
	Owner string    `json:"owner"`
	Task  *a2a.Task `json:"task"`
}

func OpenStore(path string, limit int64) (_ *Store, err error) {
	if path == "" || limit <= 0 {
		return nil, errors.New("an explicit private state directory and positive record limit are required")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("state directory must be mode 0700 or stricter")
	}
	for _, name := range []string{"records", "workspaces", "evidence"} {
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		i, err := os.Lstat(p)
		if err != nil || !i.IsDir() || i.Mode().Perm()&0077 != 0 {
			return nil, errors.New("state subdirectories must be private real directories")
		}
	}
	lockPath := filepath.Join(root, "service.lock")
	if info, err := os.Lstat(lockPath); err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
		return nil, errors.New("invalid state lock")
	}
	lock, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			lock.Close()
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.New("state directory is already served by another process")
	}
	s := &Store{inner: taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: principal}), root: root, lock: lock, contexts: map[string]string{}, limit: limit}
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, errors.New("task records must be regular files")
		}
		b, err := boundary.PrivateFile(filepath.Join(root, "records", entry.Name()), limit)
		if err != nil {
			return nil, err
		}
		var record diskTask
		if boundary.CheckJSON(b) != nil || json.Unmarshal(b, &record) != nil || record.Owner == "" || record.Task == nil || record.Task.ID == "" || record.Task.ContextID == "" || taskFile(record.Task.ID) != entry.Name() {
			return nil, errors.New("invalid persisted task record")
		}
		if owner := s.contexts[record.Task.ContextID]; owner != "" && owner != record.Owner {
			return nil, errors.New("context has conflicting persisted owners")
		}
		s.contexts[record.Task.ContextID] = record.Owner
		if record.Task.Status.State == a2a.TaskStateWorking || record.Task.Status.State == a2a.TaskStateSubmitted {
			markUnknown(record.Task, "Listener stopped before recording an execution outcome. Inspect local evidence before submitting new work.")
			if err := s.persist(&record); err != nil {
				return nil, err
			}
		}
		if _, err := s.inner.Create(ownedContext(context.Background(), record.Owner), record.Task); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func taskFile(id a2a.TaskID) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:]) + ".json"
}

func markUnknown(task *a2a.Task, message string) {
	if task.Metadata == nil {
		task.Metadata = map[string]any{}
	}
	task.Metadata["bench/outcome"] = "unknown"
	now := time.Now().UTC()
	task.Status = a2a.TaskStatus{State: a2a.TaskStateFailed, Timestamp: &now, Message: a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(message))}
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fault = os.ErrClosed // prevent a late SDK callback writing after lock release
	return s.lock.Close()
}
func (s *Store) Root() string { return s.root }

func (s *Store) persist(record *diskTask) error {
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if int64(len(b)) > s.limit {
		return errors.New("task record exceeds byte limit")
	}
	dir := filepath.Join(s.root, "records")
	f, err := os.CreateTemp(dir, ".record-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, taskFile(record.Task.ID))); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (s *Store) storageError(err error) error {
	s.fault = err
	return a2a.NewError(a2a.ErrInternalError, "Task storage failed; execution outcome requires inspection.").WithDetails(map[string]any{"bench/outcome": "unknown"})
}

func (s *Store) Create(ctx context.Context, task *a2a.Task) (taskstore.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return 0, s.storageError(s.fault)
	}
	owner, err := principal(ctx)
	if err != nil {
		return 0, err
	}
	if task == nil || task.ID == "" || task.ContextID == "" {
		return 0, a2a.ErrInvalidParams
	}
	if existing := s.contexts[task.ContextID]; existing != "" && existing != owner {
		return 0, a2a.ErrTaskNotFound
	}
	version, err := s.inner.Create(ctx, task)
	if err != nil {
		return 0, err
	}
	if err = s.persist(&diskTask{Owner: owner, Task: task}); err != nil {
		return 0, s.storageError(err)
	}
	s.contexts[task.ContextID] = owner
	return version, nil
}

func (s *Store) Update(ctx context.Context, update *taskstore.UpdateRequest) (taskstore.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return 0, s.storageError(s.fault)
	}
	if update == nil || update.Task == nil {
		return 0, a2a.ErrInvalidParams
	}
	old, err := s.inner.Get(ctx, update.Task.ID)
	if err != nil {
		return 0, err
	}
	if old.Task.ContextID != update.Task.ContextID {
		return 0, a2a.ErrInvalidParams
	}
	version, err := s.inner.Update(ctx, update)
	if err != nil {
		return 0, err
	}
	if err = s.persist(&diskTask{Owner: old.User, Task: update.Task}); err != nil {
		return 0, s.storageError(err)
	}
	return version, nil
}

func (s *Store) Get(ctx context.Context, id a2a.TaskID) (*taskstore.StoredTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return nil, s.storageError(s.fault)
	}
	return s.inner.Get(ctx, id)
}
func (s *Store) List(ctx context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return nil, s.storageError(s.fault)
	}
	return s.inner.List(ctx, req)
}
func (s *Store) CheckContext(ctx context.Context, id string) error {
	owner, err := principal(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.contexts[id]; existing != "" && existing != owner {
		return a2a.ErrTaskNotFound
	}
	return nil
}

func (s *Store) String() string { return fmt.Sprintf("A2A task records in %s", s.root) }
