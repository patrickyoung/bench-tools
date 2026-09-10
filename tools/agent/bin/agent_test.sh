#!/bin/sh
set -eu

here=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd -P)
agent=$here/bin/agent
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fake_bin=$tmp/bin
capture=$tmp/capture
mkdir -p "$fake_bin" "$capture"

cat >"$fake_bin/brief" <<'EOF'
#!/bin/sh
set -eu
case "${1:-}" in
  lint) exit 0 ;;
  ls) printf '%s\n' 'local-skill  local fixture skill' ;;
  cat) cat "$2/SKILL.md" ;;
  find) printf '%s\n' 'local-skill' ;;
  *) exit 2 ;;
esac
EOF

cat >"$fake_bin/cage" <<'EOF'
#!/bin/sh
exit 99
EOF

cat >"$fake_bin/capture-cage" <<'EOF'
#!/bin/sh
set -eu
: "${AGENT_TEST_CAPTURE:?}"
printf '%s\n' "${TMPDIR:-}" >"$AGENT_TEST_CAPTURE/cage-tmpdir"
printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/cage-argv"
exit 0
EOF

cat >"$fake_bin/hone" <<'EOF'
#!/bin/sh
set -eu
: "${AGENT_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/hone-argv"
printf '%s\n' "${BRIEF_PATH:-}" >"$AGENT_TEST_CAPTURE/hone-brief-path"
printf '%s\n' "${HONE_DIR:-}" >"$AGENT_TEST_CAPTURE/hone-dir"
printf '%s\n' "${ASK:-}" >"$AGENT_TEST_CAPTURE/hone-ask"
printf '%s\n' 'fixture lesson'
exit "${AGENT_TEST_HONE_EXIT:-0}"
EOF

cat >"$fake_bin/trail" <<'EOF'
#!/bin/sh
set -eu
: "${AGENT_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/trail-argv"
printf '%s\n' "${ASK:-}" >"$AGENT_TEST_CAPTURE/trail-ask"
printf '%s\n' '{"kind":"fixture-history"}'
exit "${AGENT_TEST_TRAIL_EXIT:-0}"
EOF

cat >"$fake_bin/ask" <<'EOF'
#!/bin/sh
exit 0
EOF

cat >"$fake_bin/action" <<'EOF'
#!/bin/sh
set -eu
: "${AGENT_TEST_CAPTURE:?}"
case "${1:-}" in
  inspect)
    cat "$2"
    printf '\n'
    ;;
  run)
    printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/action-argv"
    printf '%s\n' "${ACTION_PATH:-}" >"$AGENT_TEST_CAPTURE/action-path"
    printf '%s\n' '{"effect":"fixture"}'
    exit "${AGENT_TEST_ACTION_EXIT:-0}"
    ;;
  *) exit 2 ;;
esac
EOF

cat >"$fake_bin/may" <<'EOF'
#!/bin/sh
set -eu
: "${AGENT_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/may-argv"
cat >"$AGENT_TEST_CAPTURE/may-action"
printf '%s\n' '{"version":1,"job":"fixture","digest":"fixture","action":"bound","verdict":"spent"}'
[ -z "${AGENT_TEST_MAY_MUTATE:-}" ] || printf '%s\n' '# changed after approval' >>"$AGENT_TEST_MAY_MUTATE"
exit "${AGENT_TEST_MAY_EXIT:-0}"
EOF

cat >"$fake_bin/ply" <<'EOF'
#!/bin/sh
set -eu
if [ "${1:-}" = capabilities ]; then
  printf '%s\n' '{"schema":"ply.capabilities/v1","version":"fixture","features":{"action_boundary_receipt":"ply.action-boundary/v1","content_addressed_stdin":true,"goal_file":true,"no_delegate":true}}'
  exit 0
fi
: "${AGENT_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"$AGENT_TEST_CAPTURE/argv"
printf '%s\n' "${BRIEF_PATH:-}" >"$AGENT_TEST_CAPTURE/brief-path"
printf '%s\n' "${PLY_DIR:-}" >"$AGENT_TEST_CAPTURE/ply-dir"
printf '%s\n' "${ASK:-}" >"$AGENT_TEST_CAPTURE/ply-ask"
printf '%s\n' "${AGENT_ACTION_TMP:-}" >"$AGENT_TEST_CAPTURE/action-tmp-env"
printf '%s|%s|%s|%s|%s|%s|%s\n' "${PLY_SHELL-unset}" "${PLY_ACTION_SHELL-unset}" "${PLY_EFFORT-unset}" "${PLY_CONTRACT_ID-unset}" "${PLY_MAY_JOB-unset}" "${PLY_DEPTH-unset}" "${ASK_MODEL-unset}" >"$AGENT_TEST_CAPTURE/composition-env"
printf '%s|%s|%s\n' "${BRIEF_MODEL-unset}" "${BRIEF_EFFORT-unset}" "${BRIEF_DIR-unset}" >"$AGENT_TEST_CAPTURE/brief-runtime"
printf '%s|%s|%s|%s|%s|%s|%s|%s\n' "${RUN_INPUT-unset}" "${RUN_WAKE_OUTPUT-unset}" "${RUN_STDIN_FILE-unset}" "${AGENT_MAY-unset}" "${BENCH_MAY-unset}" "${AGENT_ACTION-unset}" "${AGENT_ACTION_PATH-unset}" "${ACTION_PATH-unset}" >"$AGENT_TEST_CAPTURE/internal-env"
cat >"$AGENT_TEST_CAPTURE/stdin"

previous=
for argument do
	case $previous in
	  -s)
		case "$argument" in
		  -) ;;
		  *) cp "$argument/SKILL.md" "$AGENT_TEST_CAPTURE/context" ;;
		esac
		previous=
		continue
		;;
	  -goal-file)
		cp "$argument" "$AGENT_TEST_CAPTURE/task"
		previous=
		continue
		;;
	esac
	previous=$argument
done

printf '%s\n' 'fixture answer'
exit "${AGENT_TEST_PLY_EXIT:-0}"
EOF

chmod 755 "$fake_bin/action" "$fake_bin/ask" "$fake_bin/brief" "$fake_bin/cage" "$fake_bin/capture-cage" "$fake_bin/hone" "$fake_bin/may" "$fake_bin/ply" "$fake_bin/trail"

failures=0
checks=0

ok() {
  checks=$((checks + 1))
  printf 'ok %d - %s\n' "$checks" "$1"
}

not_ok() {
  checks=$((checks + 1))
  failures=$((failures + 1))
  printf 'not ok %d - %s\n' "$checks" "$1"
}

assert_file() {
  label=$1
  path=$2
  if [ -f "$path" ]; then ok "$label"; else not_ok "$label"; fi
}

assert_dir() {
  label=$1
  path=$2
  if [ -d "$path" ]; then ok "$label"; else not_ok "$label"; fi
}

assert_contains() {
  label=$1
  path=$2
  pattern=$3
  if grep -F -- "$pattern" "$path" >/dev/null 2>&1; then
    ok "$label"
  else
    not_ok "$label"
  fi
}

assert_not_contains() {
  label=$1
  path=$2
  pattern=$3
  if grep -F -- "$pattern" "$path" >/dev/null 2>&1; then
    not_ok "$label"
  else
    ok "$label"
  fi
}

version_output=$($agent version)
if [ "$version_output" = 'agent 0.2.1' ]; then
  ok 'version follows the suite component contract'
else
  not_ok 'version follows the suite component contract'
fi

home=$tmp/support-chief
AGENT_BRIEF="$fake_bin/brief" "$agent" new "$home" 'Own the support queue' >/dev/null
home=$(
  cd -P "$home"
  pwd -P
)
assert_file 'new creates AGENTS.md' "$home/AGENTS.md"
assert_file 'new creates GOAL.md' "$home/GOAL.md"
assert_file 'new creates executable truth' "$home/bin/check"
assert_file 'new creates cheap wake probe' "$home/bin/wake"
assert_dir 'new creates mutable work root' "$home/work"
assert_dir 'new creates mutable state root' "$home/state"
assert_dir 'new creates file-shaped KV state' "$home/state/kv"
assert_dir 'new creates run evidence root' "$home/.agent/runs"
assert_dir 'new creates checkpoint root' "$home/.agent/checkpoints"
assert_dir 'new creates skill-selection evidence root' "$home/.agent/selections"
assert_dir 'new creates learning evidence root' "$home/.agent/learning"
assert_dir 'new creates reviewed learning proposal root' "$home/.agent/learning/proposals"
assert_dir 'new creates definition proposal root' "$home/work/proposals"
assert_dir 'new creates external action proposal root' "$home/work/actions"
assert_dir 'new creates amendment evidence root' "$home/.agent/amendments"

if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  ok 'fresh scaffold validates'
else
  not_ok 'fresh scaffold validates'
fi

show=$tmp/show
AGENT_BRIEF="$fake_bin/brief" "$agent" show "$home" >"$show"
assert_contains 'show emits a compiled hash' "$show" 'compiled-sha256:'
assert_contains 'show emits a definition hash' "$show" 'definition-sha256:'
assert_contains 'show emits a wake hash' "$show" 'wake-sha256:'
assert_contains 'show emits definition byte counts' "$show" 'AGENTS.md` ('
assert_contains 'show includes operating instructions' "$show" '## Operating instructions'
assert_contains 'show keeps goal distinct' "$show" '# Goal input'
assert_contains 'show makes default authority visible' "$show" 'network denied'
assert_contains 'show names checkpoint root' "$show" "$home/.agent/checkpoints"
assert_contains 'show names skill-selection evidence' "$show" "$home/.agent/selections/find"
assert_contains 'show names action proposal root' "$show" "$home/work/actions"

child=$home/agents/researcher
AGENT_BRIEF="$fake_bin/brief" "$agent" new "$child" 'Research one bounded question' >/dev/null
child_show=$tmp/child-show
AGENT_BRIEF="$fake_bin/brief" "$agent" show "$home" >"$child_show"
assert_contains 'show catalogues validated specialist homes' "$child_show" "$child"

bad_child=$home/agents/bad-link
ln -s "$child" "$bad_child"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  not_ok 'check refuses symlinked specialist homes'
else
  ok 'check refuses symlinked specialist homes'
fi
rm "$bad_child"

bad=$tmp/bad
AGENT_BRIEF="$fake_bin/brief" "$agent" new "$bad" 'Bad fixture' >/dev/null
rm "$bad/AGENTS.md"
ln -s "$home/AGENTS.md" "$bad/AGENTS.md"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$bad" >/dev/null 2>&1; then
  not_ok 'check refuses symlinked context'
else
  ok 'check refuses symlinked context'
fi

locked=$tmp/locked
AGENT_BRIEF="$fake_bin/brief" "$agent" new "$locked" 'Locked fixture' >/dev/null
chmod 500 "$locked/state"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$locked" >/dev/null 2>&1; then
  not_ok 'check refuses unwritable mutable state'
else
  ok 'check refuses unwritable mutable state'
fi
chmod 700 "$locked/state"

outside=$tmp/outside-file
printf '%s\n' 'outside' >"$outside"
ln "$outside" "$locked/work/linked-file"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$locked" >/dev/null 2>&1; then
  not_ok 'check refuses hard links in writable roots'
else
  ok 'check refuses hard links in writable roots'
fi

hardlink_stderr=$tmp/hardlink-stderr
locked_action_tmp=$tmp/locked-action-tmp
mkdir "$locked_action_tmp"
if AGENT_WORK="$locked/work" \
   AGENT_STATE="$locked/state" \
   AGENT_ACTION_TMP="$locked_action_tmp" \
   AGENT_FIND="$(command -v find)" \
   AGENT_CAGE="$fake_bin/cage" \
   "$here/bin/agent-action-shell" -c ':' >/dev/null 2>"$hardlink_stderr"; then
  not_ok 'action boundary rechecks hard links'
else
  status=$?
  if [ "$status" -eq 125 ]; then
    ok 'action boundary rechecks hard links'
  else
    not_ok 'action boundary rechecks hard links'
  fi
fi
assert_contains 'hard-link refusal is explained' "$hardlink_stderr" 'multiply-linked regular file'
rm "$locked/work/linked-file"

cat >"$home/tools/find" <<EOF
#!/bin/sh
printf '%s\n' escaped >"$capture/untrusted-find-ran"
exit 0
EOF
chmod 700 "$home/tools/find"
trusted_find=$tmp/trusted-find
cat >"$trusted_find" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod 700 "$trusted_find"
safe_action_tmp=$tmp/safe-action-tmp
mkdir "$safe_action_tmp"
safe_action_tmp=$(
  cd -P "$safe_action_tmp"
  pwd -P
)
rm -f "$capture/untrusted-find-ran" "$capture/cage-tmpdir" "$capture/cage-argv"
if PATH="$home/tools:$PATH" \
   AGENT_WORK="$home/work" \
   AGENT_STATE="$home/state" \
   AGENT_ACTION_TMP="$safe_action_tmp" \
   AGENT_FIND="$trusted_find" \
   AGENT_CAGE="$fake_bin/capture-cage" \
   AGENT_TEST_CAPTURE="$capture" \
   "$here/bin/agent-action-shell" -c ':' >/dev/null 2>&1; then
  ok 'action boundary reaches pinned Cage'
else
  not_ok 'action boundary reaches pinned Cage'
fi
if [ ! -e "$capture/untrusted-find-ran" ]; then
  ok 'toolbox cannot shadow pre-Cage hard-link scanner'
else
  not_ok 'toolbox cannot shadow pre-Cage hard-link scanner'
fi
assert_contains 'action boundary pins Cage temporary writes' "$capture/cage-tmpdir" "$safe_action_tmp"

unsafe_action_tmp=$home/work/action-tmp
mkdir "$unsafe_action_tmp"
rm -f "$capture/cage-tmpdir"
if AGENT_WORK="$home/work" \
   AGENT_STATE="$home/state" \
   AGENT_ACTION_TMP="$unsafe_action_tmp" \
   AGENT_FIND="$trusted_find" \
   AGENT_CAGE="$fake_bin/capture-cage" \
   AGENT_TEST_CAPTURE="$capture" \
   "$here/bin/agent-action-shell" -c ':' >/dev/null 2>&1; then
  not_ok 'action boundary refuses replaceable temporary roots'
else
  status=$?
  if [ "$status" -eq 125 ] && [ ! -e "$capture/cage-tmpdir" ]; then
    ok 'action boundary refuses replaceable temporary roots'
  else
    not_ok 'action boundary refuses replaceable temporary roots'
  fi
fi
rm -rf "$unsafe_action_tmp"
rm "$home/tools/find"

rm -f "$capture/argv"
if AGENT_TEST_CAPTURE="$capture" "$agent" tick "$home" >/dev/null 2>&1 && [ ! -e "$capture/argv" ]; then
  ok 'empty heartbeat makes no Ply call'
else
  not_ok 'empty heartbeat makes no Ply call'
fi

cat >"$home/HEARTBEAT.md" <<'EOF'
# Queue watch

When the wake probe reports a changed ticket, reconcile it in the work tree.
EOF
cat >"$home/bin/wake" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod 700 "$home/bin/wake"
rm -f "$capture/argv"
if AGENT_TEST_CAPTURE="$capture" "$agent" tick "$home" >/dev/null 2>&1 && [ ! -e "$capture/argv" ]; then
  ok 'quiet wake makes no Ply call'
else
  not_ok 'quiet wake makes no Ply call'
fi

cat >"$home/bin/wake" <<'EOF'
#!/bin/sh
printf '%s\n' 'ticket 42 changed'
exit 1
EOF
chmod 700 "$home/bin/wake"
tick_stdout=$tmp/tick-stdout
AGENT_BRIEF="$fake_bin/brief" \
AGENT_PLY="$fake_bin/ply" \
AGENT_CAGE="$fake_bin/cage" \
AGENT_ASK="$fake_bin/ask" \
AGENT_MAY="$fake_bin/may" \
BENCH_MAY="$fake_bin/may" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" tick "$home" >"$tick_stdout" 2>/dev/null
assert_contains 'changed wake starts Ply' "$tick_stdout" 'fixture answer'
assert_contains 'heartbeat replaces the standing goal' "$capture/task" 'Queue watch'
assert_contains 'wake output becomes initial evidence' "$capture/stdin" 'ticket 42 changed'
assert_not_contains 'heartbeat does not silently pursue GOAL.md' "$capture/task" 'Replace this with the durable end state'

cat >"$home/bin/wake" <<'EOF'
#!/bin/sh
exit 7
EOF
chmod 700 "$home/bin/wake"
if "$agent" tick "$home" >/dev/null 2>&1; then
  not_ok 'broken wake status stops before Ply'
else
  status=$?
  if [ "$status" -eq 2 ]; then
    ok 'broken wake status stops before Ply'
  else
    not_ok 'broken wake status stops before Ply'
  fi
fi

cat >"$home/bin/wake" <<'EOF'
#!/bin/sh
awk 'BEGIN { for (i = 0; i < 32769; i++) printf "x" }'
exit 1
EOF
chmod 700 "$home/bin/wake"
if "$agent" tick "$home" >/dev/null 2>&1; then
  not_ok 'oversized wake evidence stops before Ply'
else
  status=$?
  if [ "$status" -eq 2 ]; then
    ok 'oversized wake evidence stops before Ply'
  else
    not_ok 'oversized wake evidence stops before Ply'
  fi
fi

mkdir -p "$home/skills/local-skill"
cat >"$home/skills/local-skill/SKILL.md" <<'EOF'
---
name: local-skill
description: local fixture skill
---
Use the fixture safely.
EOF

specialist_stdout=$tmp/specialist-stdout
specialist_stderr=$tmp/specialist-stderr
set +e
AGENT_BRIEF="$fake_bin/brief" \
AGENT_PLY="$fake_bin/ply" \
AGENT_CAGE="$fake_bin/cage" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" specialist "$home" researcher -m specialist-model -- 'investigate one bounded claim' \
>"$specialist_stdout" 2>"$specialist_stderr"
specialist_status=$?
set -e
if [ "$specialist_status" -eq 0 ]; then
  ok 'specialist run succeeds'
else
  not_ok 'specialist run succeeds'
  sed 's/^/# /' "$specialist_stderr"
fi
assert_contains 'specialist preserves child stdout' "$specialist_stdout" 'fixture answer'
assert_contains 'specialist receives bounded invocation task' "$capture/task" 'investigate one bounded claim'
assert_contains 'specialist loads only child instructions' "$capture/context" 'Research one bounded question'
assert_not_contains 'specialist does not inherit parent instructions' "$capture/context" 'Own the support queue'
assert_contains 'specialist stores separate replay evidence' "$capture/ply-dir" "$child/.agent/runs"
assert_contains 'specialist model choice is caller-owned' "$capture/argv" 'specialist-model'

if "$agent" specialist "$home" '../researcher' >/dev/null 2>&1; then
  not_ok 'specialist rejects path-shaped names'
else
  ok 'specialist rejects path-shaped names'
fi

recovery=$home/.agent/runs/recovery.jsonl
printf '%s\n' '{"fixture":"verified recovery"}' >"$recovery"
learn_stdout=$tmp/learn-stdout
AGENT_BRIEF="$fake_bin/brief" \
AGENT_HONE="$fake_bin/hone" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" learn -into local-skill -m lesson-model -n 2 -N "$home" recovery \
>"$learn_stdout" 2>/dev/null
assert_contains 'learn preserves Hone stdout' "$learn_stdout" 'fixture lesson'
assert_contains 'learn forwards an explicit destination skill' "$capture/hone-argv" 'local-skill'
assert_contains 'learn forwards the exact home session' "$capture/hone-argv" "$recovery"
assert_contains 'learn scopes writes to home skills' "$capture/hone-brief-path" "$home/skills"
assert_contains 'learn stores wording evidence under controller state' "$capture/hone-dir" "$home/.agent/learning"
assert_contains 'learn pins Ask for replay and wording' "$capture/hone-ask" "$fake_bin/ask"
assert_contains 'learn forwards dry-run mode' "$capture/hone-argv" '-N'

AGENT_BRIEF="$fake_bin/brief" \
AGENT_HONE="$fake_bin/hone" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" learn -into local-skill -why "$home" recovery >/dev/null 2>/dev/null
assert_contains 'learn forwards model-free evidence review' "$capture/hone-argv" '-why'

AGENT_BRIEF="$fake_bin/brief" \
AGENT_HONE="$fake_bin/hone" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" learn -into local-skill -prepare recovery.json "$home" recovery >/dev/null 2>/dev/null
learning_proposal=$home/.agent/learning/proposals/recovery.json
assert_contains 'learn scopes exact proposal preparation' "$capture/hone-argv" "$learning_proposal"
assert_contains 'learn forwards Hone exact preparation' "$capture/hone-argv" '-prepare'

printf '%s\n' '{"version":"hone.proposal/v1"}' >"$learning_proposal"
chmod 600 "$learning_proposal"
AGENT_BRIEF="$fake_bin/brief" \
AGENT_HONE="$fake_bin/hone" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" learn -show recovery.json "$home" >/dev/null 2>/dev/null
assert_contains 'learn show invokes Hone without a session' "$capture/hone-argv" 'show'
assert_contains 'learn show reads the scoped proposal' "$capture/hone-argv" "$learning_proposal"
assert_not_contains 'learn show resolves no Ask program' "$capture/hone-ask" "$fake_bin/ask"

AGENT_BRIEF="$fake_bin/brief" \
AGENT_HONE="$fake_bin/hone" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" learn -admit recovery.json "$home" >/dev/null 2>/dev/null
assert_contains 'learn admit invokes Hone on reviewed bytes' "$capture/hone-argv" 'admit'
assert_contains 'learn admit pins Ask for provenance replay' "$capture/hone-ask" "$fake_bin/ask"

if AGENT_BRIEF="$fake_bin/brief" AGENT_HONE="$fake_bin/hone" AGENT_ASK="$fake_bin/ask" AGENT_TEST_CAPTURE="$capture" \
   "$agent" learn -prepare recovery.json -into local-skill "$home" recovery >/dev/null 2>&1; then
  not_ok 'learn refuses to overwrite an exact proposal'
else
  ok 'learn refuses to overwrite an exact proposal'
fi

if "$agent" learn -show '../outside.json' "$home" >/dev/null 2>&1; then
  not_ok 'learn refuses path-shaped proposal names'
else
  ok 'learn refuses path-shaped proposal names'
fi

outside_proposal=$tmp/outside-proposal.json
printf '%s\n' '{}' >"$outside_proposal"
ln -s "$outside_proposal" "$home/.agent/learning/proposals/link.json"
if AGENT_BRIEF="$fake_bin/brief" AGENT_HONE="$fake_bin/hone" AGENT_TEST_CAPTURE="$capture" \
   "$agent" learn -show link.json "$home" >/dev/null 2>&1; then
  not_ok 'learn refuses symlinked proposal artifacts'
else
  ok 'learn refuses symlinked proposal artifacts'
fi

ln "$outside_proposal" "$home/.agent/learning/proposals/hard.json"
if AGENT_BRIEF="$fake_bin/brief" AGENT_HONE="$fake_bin/hone" AGENT_TEST_CAPTURE="$capture" \
   "$agent" learn -show hard.json "$home" >/dev/null 2>&1; then
  not_ok 'learn refuses multiply-linked proposal artifacts'
else
  ok 'learn refuses multiply-linked proposal artifacts'
fi

if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_HONE="$fake_bin/hone" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   AGENT_TEST_HONE_EXIT=1 \
   "$agent" learn -into local-skill "$home" recovery >/dev/null 2>&1; then
  not_ok 'learn preserves Hone nothing-to-learn status'
else
  status=$?
  if [ "$status" -eq 1 ]; then
    ok 'learn preserves Hone nothing-to-learn status'
  else
    not_ok 'learn preserves Hone nothing-to-learn status'
  fi
fi

outside_session=$tmp/outside.jsonl
printf '%s\n' '{}' >"$outside_session"
if AGENT_BRIEF="$fake_bin/brief" AGENT_HONE="$fake_bin/hone" AGENT_TEST_CAPTURE="$capture" \
   "$agent" learn -into local-skill "$home" "$outside_session" >/dev/null 2>&1; then
  not_ok 'learn refuses sessions outside home evidence'
else
  ok 'learn refuses sessions outside home evidence'
fi

if "$agent" learn -into '../outside' "$home" recovery >/dev/null 2>&1; then
  not_ok 'learn refuses path-shaped skill destinations'
else
  ok 'learn refuses path-shaped skill destinations'
fi

history_stdout=$tmp/history-stdout
AGENT_BRIEF="$fake_bin/brief" \
AGENT_TRAIL="$fake_bin/trail" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" history "$home" >"$history_stdout" 2>/dev/null
assert_contains 'history preserves Trail JSONL stdout' "$history_stdout" 'fixture-history'
assert_contains 'history defaults to Trail list' "$capture/trail-argv" 'ls'
assert_contains 'history scopes list to home evidence' "$capture/trail-argv" "$home/.agent/runs"

AGENT_BRIEF="$fake_bin/brief" \
AGENT_TRAIL="$fake_bin/trail" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" history "$home" find 'connection reset; $(literal)' >/dev/null 2>/dev/null
assert_contains 'history forwards one literal semantic query' "$capture/trail-argv" 'connection reset; $(literal)'

AGENT_BRIEF="$fake_bin/brief" \
AGENT_TRAIL="$fake_bin/trail" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" history "$home" show recovery >/dev/null 2>/dev/null
assert_contains 'history resolves a session inside home evidence' "$capture/trail-argv" "$recovery"

AGENT_BRIEF="$fake_bin/brief" \
AGENT_TRAIL="$fake_bin/trail" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" history "$home" window -before 2 -after 1 recovery 4 >/dev/null 2>/dev/null
assert_contains 'history forwards bounded windows' "$capture/trail-argv" '-before'
assert_contains 'history window keeps the selected session' "$capture/trail-argv" "$recovery"

AGENT_BRIEF="$fake_bin/brief" \
AGENT_TRAIL="$fake_bin/trail" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" history "$home" check >/dev/null 2>/dev/null
assert_contains 'history check delegates replay to exact Ask' "$capture/trail-ask" "$fake_bin/ask"

if AGENT_BRIEF="$fake_bin/brief" AGENT_TRAIL="$fake_bin/trail" AGENT_TEST_CAPTURE="$capture" \
   "$agent" history "$home" show "$outside_session" >/dev/null 2>&1; then
  not_ok 'history refuses sessions outside home evidence'
else
  ok 'history refuses sessions outside home evidence'
fi

empty_actions=$tmp/empty-actions
AGENT_BRIEF="$fake_bin/brief" AGENT_ACTION="$fake_bin/action" AGENT_TEST_CAPTURE="$capture" \
"$agent" actions "$home" >"$empty_actions" 2>/dev/null
assert_contains 'action review reports an empty catalogue without mutation' "$empty_actions" 'count: 0'

action_proposal=$home/work/actions/publish-ticket.json
printf '%s\n' '{"version":1,"connector":"publish-ticket","input":{"ticket":42}}' >"$action_proposal"
rm -f "$capture/action-argv"
action_review=$tmp/action-review
AGENT_BRIEF="$fake_bin/brief" AGENT_ACTION="$fake_bin/action" AGENT_TEST_CAPTURE="$capture" \
"$agent" actions "$home" publish-ticket.json >"$action_review" 2>/dev/null
assert_contains 'action review exposes the exact proposal bytes' "$action_review" '"connector":"publish-ticket"'
assert_contains 'action review exposes the stable May job' "$action_review" 'may-job: agent-action-'
if [ ! -e "$capture/action-argv" ]; then
  ok 'action review never invokes a connector'
else
  not_ok 'action review never invokes a connector'
fi

set +e
AGENT_BRIEF="$fake_bin/brief" \
AGENT_ACTION="$fake_bin/action" \
AGENT_ACTION_PATH="$tmp/controller-actions" \
AGENT_MAY="$fake_bin/may" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
AGENT_TEST_ACTION_EXIT=75 \
"$agent" act -policy "$fake_bin/policy" "$home" publish-ticket.json recovery >/dev/null 2>/dev/null
act_status=$?
set -e
if [ "$act_status" -eq 75 ]; then
  ok 'act preserves Action parked status'
else
  not_ok 'act preserves Action parked status'
fi
assert_contains 'act passes the proposal without shell parsing' "$capture/action-argv" "$action_proposal"
assert_contains 'act binds events to the selected Ask session' "$capture/action-argv" "$recovery"
assert_contains 'act forwards the deterministic policy literally' "$capture/action-argv" "$fake_bin/policy"
assert_contains 'act selects controller-only connectors' "$capture/action-path" "$tmp/controller-actions"

if AGENT_BRIEF="$fake_bin/brief" AGENT_ACTION="$fake_bin/action" AGENT_ACTION_PATH="$tmp/controller-actions" \
   AGENT_MAY="$fake_bin/may" AGENT_ASK="$fake_bin/ask" AGENT_TEST_CAPTURE="$capture" \
   "$agent" act "$home" "$outside_session" recovery >/dev/null 2>&1; then
  not_ok 'act refuses proposals outside work/actions'
else
  ok 'act refuses proposals outside work/actions'
fi

empty_proposals=$tmp/empty-proposals
AGENT_BRIEF="$fake_bin/brief" "$agent" proposals "$home" >"$empty_proposals" 2>/dev/null
assert_contains 'proposal review reports an empty catalogue without mutation' "$empty_proposals" 'count: 0'

proposal=$home/work/proposals/add-evidence-rule.patch
cat >"$proposal" <<'EOF'
--- a/AGENTS.md
+++ b/AGENTS.md
@@ -6,4 +6,5 @@
 - Keep current progress in state/plan.md; do not rewrite definition files.
 - Put proposed definition changes under work/proposals/ for human review.
 - Put proposed external effects under work/actions/ as strict Action JSON; never invoke a connector directly or claim a proposal happened.
 - Inspect state on demand instead of loading it wholesale.
+- Summarize the evidence that changed the plan before taking action.
EOF

rm -f "$capture/may-action"
proposal_review=$tmp/proposal-review
AGENT_BRIEF="$fake_bin/brief" "$agent" proposals "$home" >"$proposal_review" 2>/dev/null
assert_contains 'proposal review prints a bounded catalogue' "$proposal_review" 'count: 1'
assert_contains 'proposal review exposes the exact target' "$proposal_review" 'target: AGENTS.md'
assert_contains 'proposal review exposes the exact May action' "$proposal_review" 'agent-amend/v1'
assert_contains 'proposal review includes literal patch bytes' "$proposal_review" 'Summarize the evidence that changed the plan'
if [ ! -e "$capture/may-action" ]; then
  ok 'proposal review does not invoke May'
else
  not_ok 'proposal review does not invoke May'
fi

set +e
AGENT_BRIEF="$fake_bin/brief" \
AGENT_MAY="$fake_bin/may" \
AGENT_TEST_CAPTURE="$capture" \
AGENT_TEST_MAY_EXIT=75 \
"$agent" amend "$home" add-evidence-rule.patch >"$tmp/amend-parked" 2>/dev/null
amend_parked_status=$?
set -e
if [ "$amend_parked_status" -eq 75 ]; then
  ok 'amend parks one exact May request before writing definition'
else
  not_ok 'amend parks one exact May request before writing definition'
fi
assert_not_contains 'parked amendment leaves definition unchanged' "$home/AGENTS.md" 'Summarize the evidence that changed the plan'
assert_contains 'amend binds the definition hash' "$capture/may-action" 'definition-sha256:'
assert_contains 'amend binds the proposal hash' "$capture/may-action" 'proposal-sha256:'
assert_contains 'amend uses a stable exact-action job' "$capture/may-argv" 'agent-amend-'

cp "$proposal" "$tmp/proposal-before-race"
set +e
AGENT_BRIEF="$fake_bin/brief" \
AGENT_MAY="$fake_bin/may" \
AGENT_TEST_CAPTURE="$capture" \
AGENT_TEST_MAY_MUTATE="$proposal" \
"$agent" amend "$home" add-evidence-rule.patch >/dev/null 2>"$tmp/amend-stale-stderr"
amend_stale_status=$?
set -e
if [ "$amend_stale_status" -eq 2 ]; then
  ok 'amend refuses a grant after proposal bytes change'
else
  not_ok 'amend refuses a grant after proposal bytes change'
fi
assert_not_contains 'stale grant leaves definition unchanged' "$home/AGENTS.md" 'Summarize the evidence that changed the plan'
cp "$tmp/proposal-before-race" "$proposal"

AGENT_BRIEF="$fake_bin/brief" \
AGENT_MAY="$fake_bin/may" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" amend "$home" add-evidence-rule.patch >"$tmp/amend-spent" 2>/dev/null
assert_contains 'approved amendment applies one reviewed root definition patch' "$home/AGENTS.md" 'Summarize the evidence that changed the plan'
set -- "$home"/.agent/amendments/*.txt
if [ -f "$1" ]; then
  ok 'approved amendment records controller evidence'
else
  not_ok 'approved amendment records controller evidence'
fi
assert_contains 'amendment receipt binds before and after definition' "$1" 'definition-before-sha256:'
assert_contains 'amendment receipt retains the May result' "$1" 'may-result:'

rollback=$home/work/proposals/empty-goal.patch
cat >"$rollback" <<'EOF'
--- a/GOAL.md
+++ b/GOAL.md
@@ -1,12 +0,0 @@
-# Outcome
-
-Replace this with the durable end state.
-
-## Acceptance evidence
-
-Explain what `bin/check` proves.
-
-## Constraints and stop conditions
-
-- Preserve anything that must not regress.
-- Stop and report when human judgment or new authority is required.
EOF
set +e
AGENT_BRIEF="$fake_bin/brief" AGENT_MAY="$fake_bin/may" AGENT_TEST_CAPTURE="$capture" \
"$agent" amend "$home" empty-goal.patch >/dev/null 2>"$tmp/amend-rollback-stderr"
rollback_status=$?
set -e
if [ "$rollback_status" -eq 2 ]; then
  ok 'amend rejects an approved patch that breaks home validation'
else
  not_ok 'amend rejects an approved patch that breaks home validation'
fi
assert_contains 'failed amendment restores the exact definition file' "$home/GOAL.md" '# Outcome'
assert_contains 'failed amendment reports rollback' "$tmp/amend-rollback-stderr" 'was rolled back'

outside_patch=$home/work/proposals/outside.patch
cat >"$outside_patch" <<'EOF'
--- a/bin/check
+++ b/bin/check
@@ -1,1 +1,1 @@
-#!/bin/sh
+#!/bin/false
EOF
if AGENT_BRIEF="$fake_bin/brief" AGENT_MAY="$fake_bin/may" AGENT_TEST_CAPTURE="$capture" \
   "$agent" amend "$home" outside.patch >/dev/null 2>&1; then
  not_ok 'amend refuses non-root-definition targets'
else
  ok 'amend refuses non-root-definition targets'
fi

if AGENT_BRIEF="$fake_bin/brief" AGENT_TRAIL="$fake_bin/trail" AGENT_TEST_CAPTURE="$capture" \
   AGENT_TEST_TRAIL_EXIT=1 "$agent" history "$home" >/dev/null 2>&1; then
  not_ok 'history preserves Trail negative status'
else
  status=$?
  if [ "$status" -eq 1 ]; then
    ok 'history preserves Trail negative status'
  else
    not_ok 'history preserves Trail negative status'
  fi
fi

linked_skills=$tmp/linked-skills
mkdir -p "$linked_skills"
ln -s "$linked_skills" "$home/skills/linked-skill"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  not_ok 'check refuses symlinked learning destinations'
else
  ok 'check refuses symlinked learning destinations'
fi
rm "$home/skills/linked-skill"

symlinked_instructions=$home/skills/symlinked-instructions
mkdir "$symlinked_instructions"
ln -s "$home/GOAL.md" "$symlinked_instructions/SKILL.md"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  not_ok 'check refuses symlinked skill instructions'
else
  ok 'check refuses symlinked skill instructions'
fi
rm "$symlinked_instructions/SKILL.md"
rmdir "$symlinked_instructions"

reserved_skill=$home/skills/agent-context
mkdir "$reserved_skill"
cp "$home/skills/local-skill/SKILL.md" "$reserved_skill/SKILL.md"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  not_ok 'check reserves the private agent-context identity'
else
  ok 'check reserves the private agent-context identity'
fi
rm "$reserved_skill/SKILL.md"
rmdir "$reserved_skill"

mutable_tool=$home/work/mutable-tool
cat >"$mutable_tool" <<'EOF'
#!/bin/sh
# model-rewritable synopsis
exit 0
EOF
chmod 700 "$mutable_tool"
ln -s "$mutable_tool" "$home/tools/mutable-tool"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  not_ok 'check refuses toolbox targets in model-writable roots'
else
  ok 'check refuses toolbox targets in model-writable roots'
fi
rm "$home/tools/mutable-tool" "$mutable_tool"

old_ply=$tmp/old-ply
cat >"$old_ply" <<'EOF'
#!/bin/sh
[ "${1:-}" = capabilities ] && { printf '%s\n' '{"schema":"old","features":{}}'; exit 0; }
exit 99
EOF
chmod 700 "$old_ply"
old_ply_stderr=$tmp/old-ply-stderr
if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$old_ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   "$agent" run "$home" >/dev/null 2>"$old_ply_stderr"; then
  not_ok 'run refuses a Ply without required boundary capabilities'
else
  ok 'run refuses a Ply without required boundary capabilities'
fi
assert_contains 'incompatible Ply names the missing capability' "$old_ply_stderr" 'ply.capabilities/v1'

rm -f "$capture/argv"
home_tmp_stderr=$tmp/home-tmp-stderr
if TMPDIR="$home/work" \
   AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   "$agent" run "$home" >/dev/null 2>"$home_tmp_stderr"; then
  not_ok 'run refuses an ambient temporary root inside home'
else
  ok 'run refuses an ambient temporary root inside home'
fi
assert_contains 'unsafe ambient temporary root is explained' "$home_tmp_stderr" 'temporary directory must be outside agent home'
if [ ! -e "$capture/argv" ]; then
  ok 'unsafe ambient temporary root stops before Ply'
else
  not_ok 'unsafe ambient temporary root stops before Ply'
fi

rm -f "$capture/argv"
oversized_stdin_stderr=$tmp/oversized-stdin-stderr
if dd if=/dev/zero bs=1048576 count=17 2>/dev/null | \
   AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   "$agent" run "$home" >/dev/null 2>"$oversized_stdin_stderr"; then
  not_ok 'oversized piped evidence stops before Ply'
else
  ok 'oversized piped evidence stops before Ply'
fi
assert_contains 'oversized piped evidence names the limit' "$oversized_stdin_stderr" 'the limit is 16777216'
if [ ! -e "$capture/argv" ]; then
  ok 'oversized piped evidence makes no model call'
else
  not_ok 'oversized piped evidence makes no model call'
fi

stdout=$tmp/stdout
stderr=$tmp/stderr
printf '%s' 'piped fixture bytes' | \
  AGENT_BRIEF="$fake_bin/brief" \
  AGENT_PLY="$fake_bin/ply" \
  AGENT_CAGE="$fake_bin/cage" \
  AGENT_ASK="$fake_bin/ask" \
  AGENT_MAY="$fake_bin/may" \
  AGENT_ACTION="$fake_bin/action" \
  AGENT_ACTION_PATH="$tmp/controller-actions" \
  ACTION_PATH="$tmp/ambient-actions" \
  BENCH_MAY="$fake_bin/may" \
  PLY_SHELL=/bin/false \
  PLY_ACTION_SHELL=/bin/false \
  PLY_EFFORT=ambient-effort \
  PLY_CONTRACT_ID=ambient-contract \
  PLY_MAY_JOB=ambient-job \
  PLY_DEPTH=7 \
  ASK_MODEL=ambient-model \
  BRIEF_MODEL=ambient-selector \
  AGENT_TEST_CAPTURE="$capture" \
  "$agent" run -m fixture-model -effort high "$home" -- 'handle ticket 42' \
  >"$stdout" 2>"$stderr"

assert_contains 'run preserves Ply stdout' "$stdout" 'fixture answer'
assert_contains 'goal reaches Ply through a private file' "$capture/task" 'Replace this with the durable end state'
assert_contains 'invocation input reaches the private task file' "$capture/task" 'handle ticket 42'
assert_contains 'piped input reaches Ply stdin' "$capture/stdin" 'piped fixture bytes'
assert_not_contains 'goal text is absent from Ply argv' "$capture/argv" 'Replace this with the durable end state'
assert_not_contains 'invocation text is absent from Ply argv' "$capture/argv" 'handle ticket 42'
assert_not_contains 'piped evidence is absent from Ply argv' "$capture/argv" 'piped fixture bytes'
assert_contains 'private context is delivered through a skill' "$capture/context" '## Operating instructions'
assert_contains 'local skills are scoped through BRIEF_PATH' "$capture/brief-path" "$home/skills"
assert_contains 'run evidence is scoped through PLY_DIR' "$capture/ply-dir" "$home/.agent/runs"
assert_contains 'run pins Ask for Ply model calls' "$capture/ply-ask" "$fake_bin/ask"
assert_contains 'ambient Ply composition variables are scrubbed' "$capture/composition-env" 'unset|unset|unset|unset|unset|unset|unset'
assert_contains 'selector uses run model, effort, and home evidence root' "$capture/brief-runtime" "fixture-model|high|$home/.agent/selections"
assert_contains 'run provides a private action temporary root' "$capture/action-tmp-env" '/agent-run.'
if action_tmp_seen=$(cat "$capture/action-tmp-env") && \
   case $action_tmp_seen in "$home"|"$home"/*) false ;; *) true ;; esac; then
  ok 'private action temporary root stays outside home'
else
  not_ok 'private action temporary root stays outside home'
fi
assert_contains 'private controller inputs are absent from Ply env' "$capture/internal-env" 'unset|unset|unset|unset|unset|unset|unset|unset'
assert_contains 'Ply works in the mutable work root' "$capture/argv" "$home/work"
assert_contains 'model selection is forwarded' "$capture/argv" 'fixture-model'
assert_contains 'effort selection is forwarded' "$capture/argv" 'high'
assert_contains 'confined action shell is forwarded' "$capture/argv" 'agent-action-shell'
assert_contains 'generic nested Ply delegation is disabled' "$capture/argv" '-no-delegate'
selected_skill_line=$(grep -n -x -- '-' "$capture/argv" | head -1 | cut -d: -f1)
context_skill_line=$(grep -n -F -- 'agent-context' "$capture/argv" | head -1 | cut -d: -f1)
if [ -n "$selected_skill_line" ] && [ -n "$context_skill_line" ] && [ "$selected_skill_line" -lt "$context_skill_line" ]; then
  ok 'governing context is composed after the selected skill'
else
  not_ok 'governing context is composed after the selected skill'
fi
assert_not_contains 'context body does not leak into argv' "$capture/argv" 'Own the support queue'

rmdir "$home/.agent/checkpoints"
if AGENT_BRIEF="$fake_bin/brief" "$agent" check "$home" >/dev/null 2>&1; then
  ok 'older home without checkpoint root still validates'
else
  not_ok 'older home without checkpoint root still validates'
fi
AGENT_BRIEF="$fake_bin/brief" \
AGENT_PLY="$fake_bin/ply" \
AGENT_CAGE="$fake_bin/cage" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" run -checkpoint release "$home" >/dev/null 2>/dev/null
assert_dir 'checkpoint run creates controller root lazily' "$home/.agent/checkpoints"
assert_contains 'checkpoint name maps to one home-scoped Ply pointer' "$capture/argv" "$home/.agent/checkpoints/release.current"

rm -f "$capture/argv"
if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   "$agent" run -checkpoint '../outside' "$home" >/dev/null 2>&1; then
  not_ok 'checkpoint rejects path-shaped names'
else
  ok 'checkpoint rejects path-shaped names'
fi
if [ -e "$capture/argv" ]; then
  not_ok 'invalid checkpoint stops before Ply'
else
  ok 'invalid checkpoint stops before Ply'
fi

outside_checkpoint_session=$tmp/outside-checkpoint.jsonl
printf '%s\n' '{}' >"$outside_checkpoint_session"
printf '%s\n' "$outside_checkpoint_session" >"$home/.agent/checkpoints/escape.current"
if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   "$agent" run -checkpoint escape "$home" >/dev/null 2>&1; then
  not_ok 'checkpoint refuses sessions outside home evidence'
else
  ok 'checkpoint refuses sessions outside home evidence'
fi
rm "$home/.agent/checkpoints/escape.current"

link_bin=$tmp/link-bin
mkdir -p "$link_bin"
ln -s "$agent" "$link_bin/agent"
AGENT_BRIEF="$fake_bin/brief" \
AGENT_PLY="$fake_bin/ply" \
AGENT_CAGE="$fake_bin/cage" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$link_bin/agent" run "$home" >/dev/null 2>/dev/null
assert_contains 'installed symlink resolves private action wrapper' "$capture/argv" "$here/bin/agent-action-shell"

if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_CAGE="$fake_bin/cage" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   AGENT_TEST_PLY_EXIT=2 \
   "$agent" run "$home" >/dev/null 2>&1; then
  not_ok 'run preserves Ply incomplete exit status'
else
  status=$?
  if [ "$status" -eq 2 ]; then
    ok 'run preserves Ply incomplete exit status'
  else
    not_ok 'run preserves Ply incomplete exit status'
  fi
fi

no_cage_stderr=$tmp/no-cage-stderr
AGENT_BRIEF="$fake_bin/brief" \
AGENT_PLY="$fake_bin/ply" \
AGENT_ASK="$fake_bin/ask" \
AGENT_TEST_CAPTURE="$capture" \
"$agent" run -no-cage "$home" >/dev/null 2>"$no_cage_stderr"
assert_contains 'no-cage mode is explicit' "$no_cage_stderr" 'Cage is disabled'
assert_not_contains 'no-cage omits action shell' "$capture/argv" 'agent-action-shell'

if AGENT_BRIEF="$fake_bin/brief" \
   AGENT_PLY="$fake_bin/ply" \
   AGENT_ASK="$fake_bin/ask" \
   AGENT_TEST_CAPTURE="$capture" \
   "$agent" run -no-cage -net "$home" >/dev/null 2>&1; then
  not_ok 'network flag requires confinement'
else
  ok 'network flag requires confinement'
fi

printf '1..%d\n' "$checks"
[ "$failures" -eq 0 ]
