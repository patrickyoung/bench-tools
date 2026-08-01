package main

// help is the whole program in one screen, and a test holds it to eighty
// columns: a summary that wraps in the terminal it was written for is a
// summary nobody reads twice.
const help = `ply — work a goal with a toolbox until a check says it is done

  ply [flags] <goal>       work the goal; stdin rides with it, or is it
  ply tools [flags]        print the toolbox exactly as the model sees it
  ply system [flags] [goal] print the system prompt that would be sent
  ply version              print the version (-V, --version)
  ply help                 print this summary (-h, --help)

Anything that is not a command is the goal; -- sends a word that is one.

tools: a tool is a program, and the toolbox is PATH. -t dir makes PATH that
directory alone, so the model reaches the programs you put there and cannot
name one you did not. -sh hands over the whole machine. Build a toolbox the
old way, and add one with ln:

    mkdir tools && ln -s $(which ls grep sed git go) tools/

The toolbox aims the model; it does not sandbox it. sh has builtins, and a
redirect opens a file with no program involved. The security boundary is
the process — its user, its container, its chroot — as it always was.

done: -check cmd runs after the model stops, and the run ends only when it
exits 0; its output goes back to the model and work continues. It also runs
before the first turn, so a goal already met costs nothing and leaves no
session behind. Without -check, exit 0 means only that the model stopped.
The check is yours, not the model's, so it runs with your PATH and the
toolbox merely first on it.

pipes: the answer is stdout, the typescript is stderr (2>/dev/null hides
it), and the exit code says what happened. The conversation is an ask
session — commands in the assistant turns, their output in the user turns —
so ask replay -check on it proves the whole run.

flags:
  -t dir        toolbox: PATH becomes this directory alone ($PLY_TOOLS)
  -sh           full shell: every program on PATH, and -t's first if given
  -check cmd    the goal is done when this shell command exits 0
  -B            work the goal even if the check already passes
  -cycles n     failed checks before giving up (default 5, 0 = unbounded)
  -turns n      model turns before giving up (default 0 = unbounded)
  -timeout d    per-command timeout, e.g. 30s (default 2m; killed is 124)
  -cap n        output kept per command, head and tail (default 16384)
  -C dir        run commands here (default: the current directory)
  -m spec       provider/model, passed to ask ($ASK_MODEL is ask's own)
  -S text       system prompt, replacing the default — ply system prints
                it, so compose with -S "$(ply system; cat house.md)"
  -s name       brief skill to append; repeat for more; -s - picks one
  -f file       session log to write (default: a new one under $PLY_DIR)
  -q            no typescript on stderr

env: PLY_TOOLS (-t) · PLY_DIR (sessions, default ~/.ply/sessions) · ASK
     (the ask binary) · BRIEF (the brief binary) · NO_COLOR
     Models and keys belong to ask; ply passes it -m, -S, -f and -q and
     nothing else. Commands run with $PLY naming this binary and $PLY_DEPTH
     counting the nesting, so a tool can start another ply: a sub-agent is
     a program, not a feature.
exit: 0 done · 1 error · 2 not done — check still failing, a cap tripped,
      or the context window is full · 130 interrupted
`
