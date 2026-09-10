#!/bin/sh
# draft's tests. A shell script, because the thing it tests is one.
#
#     sh bin/draft_test.sh
#
# No test here calls a model or reaches the network. The described-new path
# uses the actual Ask executable with an unsupported provider, so it checks
# CLI compatibility and failure handling before any provider can be called.
set -u

HERE=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DRAFT="$HERE/bin/draft"
JOURNAL=.draft-prove-restore
TMP=$(mktemp -d)
STATE="$HERE/.draft-test-state.$$"
# The tally runs on exit rather than at the bottom of the file. It used to
# be a printf on the last line, so a test appended after it ran, passed or
# failed silently, and changed nothing -- the exit status had already been
# decided one line above. Tests get appended to this file constantly; the
# structure has to survive that.
finish() {
	rm -rf "$TMP" "$STATE"
	printf '%d passed, %d failed\n' "$pass" "$fail"
	[ "$fail" -eq 0 ] || exit 1
}
trap finish EXIT
pass=0
fail=0

# `draft check` asks whether a design is buildable, and it cannot answer that
# without the tools that would build it -- so most of this suite needs them
# on PATH. Say that once, here, rather than letting it surface as eleven
# unrelated failures further down. A partial run that reports "21 passed" is
# worse than a refusal, because it reads like success.
missing=
for t in "${ASK:-ask}" "${BRIEF:-brief}" "${PLY:-ply}" "${HONE:-hone}"; do
	command -v "$t" >/dev/null 2>&1 || missing="$missing $t"
done
if [ -n "$missing" ]; then
	cat >&2 <<EOF
draft_test: cannot run: not on PATH:$missing

The suite exercises draft check, which refuses to call a design buildable
when the tools that would build it are absent. Install them, or put an
existing build on PATH:

    PATH="\$(go env GOPATH)/bin:\$PATH" sh bin/draft_test.sh
EOF
	exit 2
fi

ok() { pass=$((pass + 1)); }
no() {
	fail=$((fail + 1))
	printf 'FAIL: %s\n' "$1" >&2
	[ $# -gt 1 ] && printf '  %s\n' "$2" >&2
	return 0
}
want() { if [ "$2" = "$3" ]; then ok; else no "$1" "exit $3, want $2"; fi; }

# --- the contract ----------------------------------------------------------

"$DRAFT" help >"$TMP/help" 2>"$TMP/helperr"
want "help exits 0" 0 $?
[ -s "$TMP/help" ] && ok || no "help goes to stdout"
[ -s "$TMP/helperr" ] && no "help wrote to stderr" || ok

if awk 'length > 80 { bad = 1 } END { exit !bad }' "$TMP/help"; then
	no "help fits eighty columns" "$(awk 'length > 80 {print length": "$0}' "$TMP/help" | head -3)"
else ok; fi

"$DRAFT" nonsense >"$TMP/o" 2>"$TMP/e"
want "misuse is exit 2" 2 $?
[ -s "$TMP/o" ] && no "misuse wrote to stdout" || ok
[ -s "$TMP/e" ] && ok || no "misuse says nothing on stderr"

[ "$("$DRAFT" version | wc -l | tr -d ' ')" = 1 ] && ok || no "version is one line"

# Prove's mutation operators are behavior, not incidental data. If one
# disappears or reverses, the tool silently stops challenging that class of
# boundary bug; pin the complete small set so prove can prove itself.
mutations=$(sed -n "/^MUTATIONS='/,/^.*'$/p" "$DRAFT" |
	sed -e "1s/^MUTATIONS='//" -e "\$s/'\$//")
expected_mutations='>=|>
<=|<
==|!=
!=|==
True|False
False|True'
[ "$mutations" = "$expected_mutations" ] && ok ||
	no "prove mutation operators drifted" "$mutations"

# Every verb the script dispatches appears in help and in the README.
#
# The list used to be written out here by hand, and `prove` was added
# without touching it -- so a whole verb shipped undocumented while this
# test stayed green, which is the exact failure AGENTS.md warns about two
# repositories over. Read the verbs out of the dispatcher instead: a list
# that has to be maintained alongside the thing it checks is a list that
# will disagree with it.
verbs=$(sed -n '/^case /,/^esac/p' "$DRAFT" | sed -n 's/^\([a-z][a-z]*\)).*/\1/p')
[ -n "$verbs" ] && ok || no "could not read the verb list out of the dispatcher"
for v in $verbs; do
	grep -q "^  draft $v" "$TMP/help" || no "verb $v is missing from draft help"
	grep -q "draft $v" "$HERE/README.md" || no "verb $v is missing from README.md"
done
ok

# Line 2 is the catalogue entry ply prints, and the name is already the
# filename. contrib/edit states the rule; draft has to obey it.
case "$(sed -n 2p "$DRAFT")" in
"# draft"*) no "the synopsis repeats the name" "$(sed -n 2p "$DRAFT")" ;;
"# "*) ok ;;
*) no "line 2 is not a synopsis" "$(sed -n 2p "$DRAFT")" ;;
esac

# --- new -------------------------------------------------------------------

"$DRAFT" new "$TMP/a" >"$TMP/o" 2>/dev/null
want "new exits 0" 0 $?
[ -f "$TMP/a/DESIGN.md" ] && ok || no "new wrote no DESIGN.md"
grep -q "DESIGN.md" "$TMP/o" && ok || no "new does not print the path on stdout"

# One source of truth: the template new writes is the file the skill ships,
# not a second copy inside the script that would drift from it.
if cmp -s "$TMP/a/DESIGN.md" "$HERE/skills/draft/references/template.md"; then ok
else no "the template is not the skill's copy"; fi

"$DRAFT" new "$TMP/a" >/dev/null 2>&1
want "new refuses to clobber an existing design" 2 $?

# The real Ask parser must accept Draft's invocation before rejecting this
# deliberately unsupported provider. A fake accepting every argument missed
# the retired -n flag, which broke every model-backed design before a request.
# Provider selection fails before credentials, a session write, or networking.
DRAFT_MODEL=draft-cli-probe/never ASK_VERBOSITY= \
	"$DRAFT" new "$TMP/cli-probe" 'Write a design from this description.' \
	>"$TMP/o" 2>"$TMP/e"
want "described new propagates provider refusal" 2 $?
grep -F 'unknown provider "draft-cli-probe"' "$TMP/e" >/dev/null && ok ||
	no "described new reaches actual Ask provider validation" "$(cat "$TMP/e")"
[ ! -s "$TMP/o" ] && ok || no "failed described new printed a design path"
[ ! -e "$TMP/cli-probe/DESIGN.md" ] && ok || no "failed described new published a design"
[ ! -e "$TMP/cli-probe/DESIGN.md.tmp" ] && ok || no "failed described new left temporary output"
[ ! -e "$TMP/cli-probe/.draft/design.jsonl" ] && ok || no "provider probe wrote a session"

# --- check: the falsifiability gate ---------------------------------------

"$DRAFT" check "$TMP/a" >"$TMP/o" 2>"$TMP/e"
want "check refuses the unfilled template" 1 $?
grep -q "false" "$TMP/e" && ok || no "check does not name the template check" "$(cat "$TMP/e")"
grep -q "checkbox" "$TMP/e" && ok || no "check does not name the empty checkbox"
[ -s "$TMP/o" ] && no "a refused design wrote to stdout" || ok

fill() { # fill <dir> <check-command>
	mkdir -p "$1"
	sed -e 's/^- \[ \]$/- [x] it does the thing/' \
	    -e "s|^false\$|$2|" \
	    "$HERE/skills/draft/references/template.md" >"$1/DESIGN.md"
}

fill "$TMP/b" "go test ./..."
"$DRAFT" check "$TMP/b" >"$TMP/o" 2>/dev/null
want "check accepts a filled design" 0 $?
[ "$(cat "$TMP/o")" = "go test ./..." ] && ok ||
	no "check does not print the command on stdout" "$(cat "$TMP/o")"

# A design missing a required section is refused, and the message names it.
fill "$TMP/c" "true"
grep -v '^## Data' "$TMP/c/DESIGN.md" >"$TMP/c/x" && mv "$TMP/c/x" "$TMP/c/DESIGN.md"
"$DRAFT" check "$TMP/c" >/dev/null 2>"$TMP/e"
want "check refuses a design missing a section" 1 $?
grep -q "## Data" "$TMP/e" && ok || no "the refusal does not name the section"

# An empty Check section is the same refusal as a missing one: a design that
# cannot say how you would know it worked is not a design.
fill "$TMP/d" ""
"$DRAFT" check "$TMP/d" >/dev/null 2>"$TMP/e"
want "check refuses an empty Check section" 1 $?
grep -q "no command" "$TMP/e" && ok || no "the refusal is not specific" "$(cat "$TMP/e")"

# Multi-line checks survive, and only the first fenced block is taken -- a
# design may show a failing example underneath without changing the verdict.
mkdir -p "$TMP/e2"
cat >"$TMP/e2/DESIGN.md" <<'EOF'
# x
## Requirements
- [x] a
## Not doing
- nothing
## The split
| a | b | c |
## Data
raw: x
## Check

```sh
sh -n bin/*
go test ./...
```

Not this one:

```sh
false
```
EOF
"$DRAFT" check "$TMP/e2" >"$TMP/o" 2>/dev/null
want "a multi-line check is accepted" 0 $?
[ "$(wc -l <"$TMP/o" | tr -d ' ')" = 2 ] && ok ||
	no "took more than the first fenced block" "$(cat "$TMP/o")"

"$DRAFT" check "$TMP/nosuch" >/dev/null 2>&1
want "check on a missing design is an error, not a no" 2 $?

# A check that cannot fail is the opposite failure to an unfilled template,
# and the more expensive one: ply drives until exit 0, which is immediately,
# so the design looks finished and certifies nothing.
for v in true : "exit 0" "/bin/true"; do
	fill "$TMP/v" "$v"
	rm -f "$TMP/v/.seen"
	"$DRAFT" check "$TMP/v" >/dev/null 2>"$TMP/e"
	if [ $? = 1 ] && grep -q "cannot fail" "$TMP/e"; then ok
	else no "check accepts the vacuous check '$v'" "$(cat "$TMP/e")"; fi
done

# ...but a real command that merely contains the word true is fine.
fill "$TMP/w" "test -f built.txt && true"
"$DRAFT" check "$TMP/w" >/dev/null 2>&1
want "a real check containing 'true' is accepted" 0 $?

# --- build refuses to run an unbuildable design ---------------------------

# No model is called: the refusal happens before ply is ever reached.
"$DRAFT" build "$TMP/a" >/dev/null 2>"$TMP/e"
want "build refuses an unbuildable design" 1 $?
grep -q "not buildable" "$TMP/e" && ok || no "build does not say why" "$(cat "$TMP/e")"

# --- admit: the worker cannot rewrite its own verdict ---------------------

mkdir -p "$TMP/gates" "$STATE"
REAL_MAY=$(command -v may)
REAL_CAGE=$(command -v cage)
cat >"$TMP/gates/may" <<'EOF'
#!/bin/sh
case ${1:-} in
version|-V|--version|help|-h|--help) exec "$REAL_MAY" "$@" ;;
esac
cat >"$MAY_LOG"
exit "${MAY_EXIT:-0}"
EOF
cat >"$TMP/gates/cage" <<'EOF'
#!/bin/sh
case ${1:-} in
version|-V|--version|help|-h|--help) exec "$REAL_CAGE" "$@" ;;
esac
printf '%s\n' "$@" >"$CAGE_LOG"
printf '%s\n' "${PLY_DIR:-}" >"$CAGE_ENV_LOG"
exit 0
EOF
chmod 755 "$TMP/gates/may" "$TMP/gates/cage"
export MAY_LOG="$TMP/may.log" CAGE_LOG="$TMP/cage.log"
export CAGE_ENV_LOG="$TMP/cage-env.log" REAL_MAY REAL_CAGE

fill "$TMP/admitted" "test -f result"
XDG_STATE_HOME="$STATE" MAY="$TMP/gates/may" \
	"$DRAFT" admit "$TMP/admitted" >"$TMP/receipt" 2>"$TMP/e"
want "admit exits 0 after May approves the exact verifier" 0 $?
receipt=$(cat "$TMP/receipt")
[ -f "$receipt" ] && ok || no "admit wrote no verifier receipt" "$receipt"
grep -q "draft verifier admission v1" "$MAY_LOG" &&
	grep -q "test -f result" "$MAY_LOG" && ok ||
	no "May did not receive the exact admission" "$(cat "$MAY_LOG")"

# Existing operator state is reusable without asking again, but only for the
# same canonical project and command bytes.
MAY_EXIT=3 XDG_STATE_HOME="$STATE" MAY="$TMP/gates/may" \
	"$DRAFT" admit "$TMP/admitted" >"$TMP/receipt2" 2>/dev/null
want "an existing matching admission is reusable" 0 $?
cmp -s "$TMP/receipt" "$TMP/receipt2" && ok || no "admission address changed"

XDG_STATE_HOME="$STATE" CAGE="$TMP/gates/cage" \
	"$DRAFT" build -admitted "$TMP/admitted" >/dev/null 2>"$TMP/e"
want "an admitted build enters Cage" 0 $?
grep -qx -- "-net" "$CAGE_LOG" && grep -qx -- "-w" "$CAGE_LOG" &&
	grep -qx "$(CDPATH= cd -P -- "$TMP/admitted" && pwd)" "$CAGE_LOG" && ok ||
	no "Cage did not receive the project write boundary" "$(cat "$CAGE_LOG")"
grep -Fqx ". '$receipt'" "$CAGE_LOG" && ok ||
	no "Ply did not receive the frozen verifier" "$(cat "$CAGE_LOG")"
admitted_project=$(CDPATH= cd -P -- "$TMP/admitted" && pwd)
[ "$(cat "$CAGE_ENV_LOG")" = "$admitted_project/.draft/build" ] && ok ||
	no "admitted build session is outside the writable project" "$(cat "$CAGE_ENV_LOG")"

fill "$TMP/refused" "test -f another-result"
MAY_EXIT=3 XDG_STATE_HOME="$STATE" MAY="$TMP/gates/may" \
	"$DRAFT" admit "$TMP/refused" >/dev/null 2>/dev/null
want "May refusal propagates and stores nothing" 3 $?

XDG_STATE_HOME="$TMP/unsafe-state" MAY="$TMP/gates/may" \
	"$DRAFT" admit "$TMP/refused" >/dev/null 2>"$TMP/e"
want "admission refuses verifier state inside Cage's temporary write root" 2 $?
grep -q "inside a Cage write root" "$TMP/e" && ok ||
	no "unsafe verifier state was not diagnosed" "$(cat "$TMP/e")"

chmod 700 "$receipt"
printf 'different bytes\n' >"$receipt"
XDG_STATE_HOME="$STATE" CAGE="$TMP/gates/cage" \
	"$DRAFT" build -admitted "$TMP/admitted" >/dev/null 2>"$TMP/e"
want "an altered admitted verifier is a broken gate" 2 $?
grep -q "corrupt" "$TMP/e" && ok || no "corrupt verifier was not diagnosed" "$(cat "$TMP/e")"

# --- the reference --------------------------------------------------------

# The skill must name every file in references/, or progressive disclosure
# silently drops it: no agent reads a directory it was never told about.
for f in "$HERE"/skills/draft/references/*; do
	b=$(basename "$f")
	grep -q "$b" "$HERE/skills/draft/SKILL.md" || no "SKILL.md never mentions references/$b"
done
ok

# The generated reference says it is generated, so nobody edits it by hand.
if [ -f "$HERE/skills/draft/references/tools.md" ]; then
	head -1 "$HERE/skills/draft/references/tools.md" | grep -qi generated && ok ||
		no "the reference does not say it is generated"
else
	ok # not synced in this checkout; draft check reports that itself
fi


# --- prove: does the check have teeth? -------------------------------------
#
# The gap prove exists to close: `draft check` says a check exists, ply says
# it passes, and neither says it is any good. So the test is not "does prove
# run" but "does prove tell a real check apart from a decorative one".

proj() { # proj <dir> <check-command>  -- a tiny buildable project
	mkdir -p "$1"
	fill "$1" "$2"
	cat >"$1/calc.py" <<'PYEOF'
def over(n, limit):
    return n >= limit
PYEOF
}

# A check that actually tests the boundary kills the >= -> > mutant.
proj "$TMP/p1" "python3 -c \"import calc; assert calc.over(5,5) and not calc.over(4,5)\""
"$DRAFT" prove -n 4 "$TMP/p1" >"$TMP/o" 2>"$TMP/e"
want "prove exits 0 when nothing survives" 0 $?
grep -q "killed" "$TMP/e" && ok || no "prove reports no score" "$(cat "$TMP/e")"
[ -s "$TMP/o" ] && no "a clean run listed survivors" "$(cat "$TMP/o")" || ok

# The same code with a check that never exercises the boundary: the mutant
# lives, and prove says which line and which flip.
proj "$TMP/p2" "python3 -c \"import calc; assert calc.over(9,5)\""
"$DRAFT" prove -n 4 "$TMP/p2" >"$TMP/o" 2>"$TMP/e"
want "prove exits 1 when a mutant survives" 1 $?
grep -q "calc.py.*survived" "$TMP/o" && ok ||
	no "prove did not name the surviving mutation" "$(cat "$TMP/o")"

# The source must be exactly as it was, whatever happened in between.
cmp -s "$TMP/p2/calc.py" "$TMP/p1/calc.py" && ok ||
	no "prove did not restore the file it mutated" "$(cat "$TMP/p2/calc.py")"

# A check that is already failing makes every mutation result meaningless,
# so it is refused rather than measured.
proj "$TMP/p3" "false"
sed -i.bak 's|^false$|python3 -c "raise SystemExit(1)"|' "$TMP/p3/DESIGN.md" 2>/dev/null || true
"$DRAFT" prove -n 2 "$TMP/p3" >/dev/null 2>"$TMP/e"
want "prove refuses a check that does not pass clean" 2 $?

# Data files are never mutated: an allowlist, because the first version
# excluded .json and a .jsonl session log walked straight through it.
proj "$TMP/p4" "python3 -c \"import calc; assert calc.over(5,5) and not calc.over(4,5)\""
printf '{"a":1,"b":2}\n' >"$TMP/p4/data.jsonl"
before=$(cat "$TMP/p4/data.jsonl")
"$DRAFT" prove -n 6 "$TMP/p4" >/dev/null 2>&1
[ "$(cat "$TMP/p4/data.jsonl")" = "$before" ] && ok || no "prove mutated a data file"

# Fixtures are never mutated either, and this is the case the data-file rule
# above does NOT cover: a fixture server with a shebang and no extension is
# indistinguishable from source by the extension rule, so it walked straight
# through. Two of six survivors in a real vouch run were mutations of its
# fixture OAuth provider -- noise wearing exactly the clothes of a finding,
# because a survivor in the world the code is tested IN says nothing about
# the code or the check.
proj "$TMP/p5" "python3 -c \"import calc; assert calc.over(5,5) and not calc.over(4,5)\""
mkdir -p "$TMP/p5/fixtures"
printf '#!/bin/sh\n# a stub the check runs against\n[ 1 -ge 1 ] && echo ok\n' >"$TMP/p5/fixtures/stub"
chmod +x "$TMP/p5/fixtures/stub"
before=$(cat "$TMP/p5/fixtures/stub")
"$DRAFT" prove -n 8 "$TMP/p5" >"$TMP/o" 2>/dev/null
[ "$(cat "$TMP/p5/fixtures/stub")" = "$before" ] && ok || no "prove mutated a fixture"
grep -q "fixtures/" "$TMP/o" && no "prove reported a fixture as a survivor" "$(cat "$TMP/o")" || ok

# --- installed via symlink -------------------------------------------------
#
# The README says to install with `ln -s "$PWD/bin/draft" ~/.local/bin/draft`,
# which made $0 the link. Taking dirname of that put the skill directory
# somewhere that does not exist, and the failure surfaced as draft claiming
# its tool reference had drifted -- so the recommended install produced a
# program that misreported why it was broken. Every test above runs draft by
# its real path, which is exactly why none of them saw it.
ln -sfn "$DRAFT" "$TMP/linked-draft"
"$TMP/linked-draft" check "$TMP/b" >"$TMP/o" 2>"$TMP/e"
want "a symlinked draft still works" 0 $?
grep -q "drifted" "$TMP/e" && no "symlinked draft misreports a drifted reference" "$(cat "$TMP/e")" || ok

# ...and through a link to a link, since ~/.local/bin -> elsewhere is common.
ln -sfn "$TMP/linked-draft" "$TMP/twice-linked"
"$TMP/twice-linked" check "$TMP/b" >/dev/null 2>&1
want "a doubly symlinked draft still works" 0 $?

# --- prove must never destroy a source file --------------------------------
#
# It did, once. `sed ... >"$file"` truncates the target as the shell opens
# it, and a SIGKILL inside that window left lib/hn_topics/cli.py at zero
# bytes -- recovered only because it had been committed minutes earlier.
# Writes go through a sibling temporary and a rename now, and the original
# is journalled inside the project so an uninterruptible kill is still
# recoverable on the next run.

proj "$TMP/k1" "python3 -c \"import calc; assert calc.over(5,5) and not calc.over(4,5)\""
mode_before=$(ls -l "$TMP/k1/calc.py" | cut -c1-10)
sum_before=$(cksum <"$TMP/k1/calc.py")

# Kill prove mid-flight, uncatchably, and the source must still be whole.
"$DRAFT" prove -n 40 "$TMP/k1" >/dev/null 2>&1 &
pid=$!
sleep 2
kill -9 $pid 2>/dev/null
wait $pid 2>/dev/null
[ -s "$TMP/k1/calc.py" ] && ok || no "SIGKILL left the source empty"

# The next run puts back anything the kill stranded, before measuring.
"$DRAFT" prove -n 4 "$TMP/k1" >/dev/null 2>&1
[ "$(cksum <"$TMP/k1/calc.py")" = "$sum_before" ] && ok ||
	no "the source did not come back byte-identical" "$(cat "$TMP/k1/calc.py")"
[ "$(ls -l "$TMP/k1/calc.py" | cut -c1-10)" = "$mode_before" ] && ok ||
	no "file mode was not preserved across mutation"
[ -d "$TMP/k1/$JOURNAL" ] && no "prove left its journal behind" || ok

# An executable stays executable: bin/ scripts are mutated too.
proj "$TMP/k2" "sh $TMP/k2/run.sh"
printf '#!/bin/sh\nexit 0\n' >"$TMP/k2/run.sh"; chmod 755 "$TMP/k2/run.sh"
"$DRAFT" prove -n 3 "$TMP/k2" >/dev/null 2>&1
[ -x "$TMP/k2/run.sh" ] && ok || no "prove stripped the executable bit"

# --- a mutant that hangs must not stall the measurement --------------------
#
# Removing a loop's termination condition makes the check run forever, not
# fail. One such mutant stalled a real run for ten minutes with a stray
# interpreter still burning a core after its parent was killed. A hang is a
# difference the check detected, so it counts as killed -- and the bound has
# to reach the whole process group, or the python outlives the shell.
mkdir -p "$TMP/h1"
fill "$TMP/h1" "python3 -c \"import loopy; loopy.walk(3)\""
cat >"$TMP/h1/loopy.py" <<'PYEOF'
def walk(n):
    i = 0
    while i <= n:
        i += 1
    return i
PYEOF
start=$(date +%s)
"$DRAFT" prove -n 2 "$TMP/h1" >"$TMP/o" 2>"$TMP/e"
elapsed=$(( $(date +%s) - start ))
# <= -> < still terminates; the guard is that nothing runs unbounded.
if [ "$elapsed" -lt 90 ]; then ok; else no "a hanging mutant was not bounded" "${elapsed}s"; fi
grep -q "bound" "$TMP/e" && ok || no "prove does not report its bound" "$(cat "$TMP/e")"
pgrep -f "import loopy" >/dev/null 2>&1 && no "a mutant process outlived the run" || ok

# --- the sample must span the codebase, not its first file -----------------
#
# `-n N` used to mean "the first N sites I tripped over", which exhausted
# the alphabetically-first file and never reached the rest. Two runs either
# side of a real fix scored 30% and 26% without touching the changed lines.
mkdir -p "$TMP/s1"
fill "$TMP/s1" "true"
sed -i.bak 's|^true$|python3 -c "import a,b,c"|' "$TMP/s1/DESIGN.md"
for m in a b c; do
	{ printf 'def f%s(n):\n' "$m"
	  for i in 1 2 3 4 5 6; do printf '    if n >= %d: pass\n' "$i"; done
	} >"$TMP/s1/$m.py"
done
"$DRAFT" prove -n 6 "$TMP/s1" >"$TMP/o" 2>"$TMP/e"
grep -q "measuring" "$TMP/e" && ok || no "prove does not say what it measured" "$(cat "$TMP/e")"
spanned=$(grep -oE '^[abc]\.py' "$TMP/o" | sort -u | wc -l | tr -d ' ')
[ "$spanned" -ge 2 ] && ok ||
	no "the sample stayed inside one file" "touched $spanned of 3 files"

# The cap is a target, not a ceiling to undershoot. An integer stride once
# turned "40 of 46" into 23, honouring a cap that was not even binding.
#
# The error file is named rather than numbered: $TMP/e2 is a *directory*
# made by an earlier test, so `2>"$TMP/e2"` failed, the command never ran,
# and the test reported an empty measurement instead of a broken redirect.
mkdir -p "$TMP/cap"; fill "$TMP/cap" "python3 -c \"import many\""
{ printf 'def f(n):\n'; for i in $(seq 1 25); do printf '    if n >= %d: pass\n' "$i"; done; } >"$TMP/cap/many.py"
"$DRAFT" prove -n 20 "$TMP/cap" >/dev/null 2>"$TMP/cap.err"
got=$(sed -n 's/.*measuring \([0-9]*\) spread.*/\1/p' "$TMP/cap.err")
[ "$got" = 20 ] && ok || no "asked for 20 of 25 and measured $got" "$(cat "$TMP/cap.err")"

# Deterministic: the same tree twice gives the same sites, or a score
# cannot be compared with the one before it.
"$DRAFT" prove -n 6 "$TMP/s1" >"$TMP/o2" 2>/dev/null
cmp -s "$TMP/o" "$TMP/o2" && ok || no "two runs sampled different sites"
