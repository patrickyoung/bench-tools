package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerbosityPolicyReachesAskAndNestedPly(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		set  bool
		args []string
		want string
	}{
		{name: "default", want: "low"},
		{name: "environment", env: "high", set: true, want: "high"},
		{name: "flag_overrides_environment", env: "high", set: true, args: []string{"-verbosity", "medium"}, want: "medium"},
		{name: "empty_flag_omits", env: "high", set: true, args: []string{"-verbosity", ""}},
		{name: "empty_environment_omits", set: true},
		{name: "literal_policy", args: []string{"-verbosity", "future-provider-level"}, want: "future-provider-level"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PLY_VERBOSITY", tc.env)
			if !tc.set {
				if err := os.Unsetenv("PLY_VERBOSITY"); err != nil {
					t.Fatal(err)
				}
			}
			work, _, askdir := sandbox(t, "Done; check passed.")
			args := append([]string{"-sh", "-C", work}, tc.args...)
			code, _, stderr := runPly(t, append(args, "goal")...)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			argv := read(t, filepath.Join(askdir, "argv.log"))
			if tc.want == "" {
				if strings.Contains(argv, "-verbosity") {
					t.Fatalf("empty policy reached Ask: %s", argv)
				}
			} else if !strings.Contains(argv, "-verbosity "+tc.want) {
				t.Fatalf("policy missing from Ask: %s", argv)
			}
			o := newOpts("ply")
			if err := o.fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			runner := o.runner(&Box{Shell: true}, "/bin/ply", 0, defaultShell, defaultShell, nil)
			if !strings.Contains("\n"+strings.Join(runner.Env, "\n")+"\n", "\nPLY_VERBOSITY="+tc.want+"\n") {
				t.Fatalf("nested policy not preserved: %q", runner.Env)
			}
		})
	}
}

func TestCompactionKeepsVerbosityPolicy(t *testing.T) {
	next := filepath.Join(t.TempDir(), "next.jsonl")
	write(t, next, "session fixture", 0o600)
	ask, dir := fakeAsk(t, next)
	model := Model{Bin: ask, Session: filepath.Join(dir, "source.jsonl"), Verbosity: "low"}
	if got, err := model.Compact(context.Background()); err != nil || got != next {
		t.Fatalf("compact=%q err=%v", got, err)
	}
	if argv := read(t, filepath.Join(dir, "argv.log")); !strings.Contains(argv, "compact -q -verbosity low "+model.Session) {
		t.Fatalf("summary lost verbosity: %s", argv)
	}
}
