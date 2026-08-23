package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// view writes the typescript on stderr. It is a display, not a record: the
// record is the ask session, which holds every one of these bytes already.
// 2>/dev/null costs nothing but the watching.
type view struct {
	w     io.Writer
	color bool
	quiet bool
	// dup is set when stdout and stderr are the same terminal, so the final
	// answer is not printed twice to one screen.
	dup bool
}

func newView(w io.Writer, quiet bool) *view {
	f, isFile := w.(*os.File)
	return &view{
		w:     w,
		quiet: quiet,
		color: isFile && os.Getenv("NO_COLOR") == "" && isTTY(f),
		dup:   isFile && sameFile(os.Stdout, f),
	}
}

func (v *view) sgr(code, s string) string {
	if !v.color || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (v *view) dim(s string) string  { return v.sgr("2", s) }
func (v *view) bold(s string) string { return v.sgr("1", s) }
func (v *view) red(s string) string  { return v.sgr("31", s) }

// Note is ply speaking for itself: which session, which model, what it
// decided. Always prefixed, so a typescript can be read apart from the
// program that produced it.
func (v *view) Note(format string, args ...any) {
	if v.quiet {
		return
	}
	fmt.Fprintln(v.w, v.dim("ply: "+fmt.Sprintf(format, args...)))
}

// Reply prints what the model said, in the order it said it: the prose
// belongs above the commands it introduces, and a reply the check later
// rejects belongs above that verdict.
func (v *view) Reply(text string) {
	if v.quiet || strings.TrimSpace(text) == "" {
		return
	}
	fmt.Fprintln(v.w, strings.TrimRight(text, "\n"))
}

// Shown reports that the answer has already reached the screen stdout is
// pointed at, which is the one case where printing it again would show it
// twice. Same terminal, same file, same pipe; a redirected stderr does not
// count, and neither does -q, under which nothing was shown at all.
func (v *view) Shown() bool { return !v.quiet && v.dup }

// Result prints one command the way a terminal would, with the prompt in
// bold so the eye finds what ran in a screen of what it printed.
func (v *view) Result(r Result) {
	if v.quiet {
		return
	}
	var s strings.Builder
	for i, line := range strings.Split(r.Cmd, "\n") {
		p := "$ "
		if i > 0 {
			p = "> "
		}
		s.WriteString(v.bold(p+line) + "\n")
	}
	s.WriteString(r.Output)
	if r.Output != "" && !strings.HasSuffix(r.Output, "\n") {
		s.WriteString("\n")
	}
	switch {
	case r.Killed:
		s.WriteString(v.red(fmt.Sprintf("[ply: killed after %s] exit %d", r.Timeout, r.Code)) + "\n")
	case r.Code != 0:
		s.WriteString(v.red(fmt.Sprintf("exit %d", r.Code)) + "\n")
	}
	if r.Elided > 0 {
		s.WriteString(v.dim(fmt.Sprintf("[ply: %d bytes, %d elided]", r.Total, r.Elided)) + "\n")
	}
	fmt.Fprint(v.w, s.String())
}

// Approval renders a proposed action without borrowing the shell prompt used
// for actions that actually ran. The sealed receipt, not this view, is the
// durable authority record.
func (v *view) Approval(script string, receipt approvalReceipt) {
	if v.quiet {
		return
	}
	state := strings.ToUpper(receipt.Verdict)
	if receipt.Verdict == "spent" {
		fmt.Fprintln(v.w, v.dim("ply: approval spent "+receipt.Digest+"; executing exact action"))
		return
	}
	fmt.Fprintln(v.w, v.red("ply: approval "+state+" "+receipt.Digest+" · NOT EXECUTED"))
	for i, line := range strings.Split(script, "\n") {
		prefix := "? "
		if i > 0 {
			prefix = ": "
		}
		fmt.Fprintln(v.w, v.bold(prefix+line))
	}
}

// Check prints the verdict. It is the one line worth finding in a long
// typescript, so it says the word.
func (v *view) Check(r Result) {
	if v.quiet {
		return
	}
	if r.Code == 0 {
		fmt.Fprintln(v.w, v.dim("ply: check passed: ")+v.bold(oneline(r.Cmd)))
		return
	}
	if r.Code != 1 {
		fmt.Fprintln(v.w, v.red("ply: check broken: ")+v.bold(oneline(r.Cmd)))
		v.Result(r)
		return
	}
	fmt.Fprintln(v.w, v.red("ply: check failed: ")+v.bold(oneline(r.Cmd)))
	v.Result(r)
}

// oneline fits a command on a line of a terminal. A check can be a
// paragraph of shell, and the header that names it is a label, not the
// record — the record is argv and the log, and both keep it whole.
func oneline(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 64 {
		s = strings.TrimRight(s[:64], " ") + "..."
	}
	return s
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func sameFile(a, b *os.File) bool {
	fa, err := a.Stat()
	if err != nil {
		return false
	}
	fb, err := b.Stat()
	return err == nil && os.SameFile(fa, fb)
}
