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

shell: commands and checks use /bin/sh -c by default. -shell names one other
executable that accepts -c. Ply resolves it before calling the model and says
exactly which interpreter it chose. The login-shell variable $SHELL is ignored.

loop: one model turn consumes one shell block or a report with no block. Ply
runs the first complete action and returns its result before asking again;
later blocks and claims are deferred. Empty or unfinished first blocks run none.
With -require-action, a report before any command is corrected, then exit 2.

done: -check cmd is a verifier. Before work it receives empty stdin; after
the model stops it receives the candidate report. Exit 0 accepts, 1 rejects
and sends its output back, and any other status means the verifier broke.
Interpreter startup failure or output beyond -cap is broken too: verifier
evidence is never silently truncated.
A passing pre-check costs no model turn or session. Without -check, exit 0
means only that the model stopped. The check is yours, not the model's, so it
runs with your PATH and the toolbox merely first on it.

pipes: the answer is stdout, the typescript is stderr (2>/dev/null hides
it), and the exit code says what happened. The conversation is an ask
session — commands in the assistant turns, their output in the user turns —
and each verifier run is a sealed structured receipt, so ask replay -check
detects changes, gaps, reordering, or an unsealed record in a retained prefix.

flags:
  -t dir        toolbox: PATH becomes this directory alone ($PLY_TOOLS)
  -sh           full shell: every program on PATH, and -t's first if given
  -shell path   command interpreter ($PLY_SHELL; default /bin/sh)
  -check cmd    verifier: candidate stdin; 0 accepts, 1 rejects, other breaks
  -B            work the goal even if the check already passes
  -require-action  refuse a final report until at least one command runs
  -cycles n     rejected candidates before giving up (default 5, 0 = unbounded)
  -compact      when the context window fills, carry on: ask compact writes
                a handoff note and the run continues in a fresh session
  -compactions n  compactions before giving up (default 3, 0 = unbounded)
  -turns n      model turns before giving up (default 50, 0 = unbounded)
  -steer file   read appended UTF-8 lines between model turns
  -may-job job  require exact May approval before every model action
  -cage         confine approved actions; needs -may-job and -contract-id
  -timeout d    per-command timeout, e.g. 30s (default 2m; killed is 124)
  -cap n        output kept per command, head and tail (default 16384)
  -C dir        run commands here (default: the current directory)
  -m spec       provider/model, passed to ask ($ASK_MODEL is ask's own)
  -effort e     reasoning effort, passed literally to ask ($PLY_EFFORT)
  -S text       system prompt, replacing the default — ply system prints
                it, so compose with -S "$(ply system; cat house.md)"
  -s name       brief skill to compose; repeat for more; -s - picks one
  -f file       session log to write (default: a new one under $PLY_DIR)
  -session-out file  atomically write the current session path here
  -contract-id digest  bind verifier receipts to an admitted intent contract
  -q            no typescript on stderr

env: PLY_TOOLS (-t) · PLY_SHELL (-shell) · PLY_EFFORT (-effort) · PLY_DIR
     (sessions, default ~/.ply/sessions) · ASK (the ask binary) · BRIEF
     (the brief binary) · MAY (the may binary) · CAGE (the cage binary)
     · PLY_MAY_JOB · NO_COLOR
     Models and keys belong to ask; ply passes it -m, -effort, -S, -f and
     -q and nothing else. Commands run with $PLY naming this binary and
     $PLY_DEPTH
     counting the nesting, so a tool can start another ply: a sub-agent is
     a program, not a feature.
exit: 0 done · 1 error (including broken verifier) · 2 not done — rejected,
      bound, protocol, or context · 3 approval declined · 75 approval required
      · 125 confinement failed · 130 interrupted
`
