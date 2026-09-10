#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd -P)
agent=$root/bin/agent
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-eval.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
cp -R "$root/eval/corpus" "$tmp/corpus"

ply_program=${AGENT_PLY:-ply}
for program in brief "$ply_program" cage trail ask; do
	command -v "$program" >/dev/null 2>&1 || {
		printf 'agent eval: %s is required\n' "$program" >&2
		exit 2
	}
done

fake_ask=$tmp/fail-if-called-ask
cat >"$fake_ask" <<'EOF'
#!/bin/sh
: "${AGENT_EVAL_MODEL_MARKER:?}"
printf 'called\n' >"$AGENT_EVAL_MODEL_MARKER"
exit 99
EOF
chmod 700 "$fake_ask"
model_marker=$tmp/model-called

definition_bytes() {
	sed -n 's/.*(\([0-9][0-9]*\) bytes).*/\1/p' "$1" |
		awk '{ total += $1 } END { print total + 0 }'
}

show_metrics() {
	name=$1
	home=$2
	show=$tmp/$name.show
	AGENT_BRIEF=brief "$agent" show "$home" >"$show" 2>"$tmp/$name.show.stderr"
	printf '%s\t%s\t%s\t' "$name" "$(definition_bytes "$show")" "$(wc -c <"$show" | tr -d '[:space:]')"
}

printf 'case\tdefinition_bytes\tshow_bytes\tcheck\ttick_or_run\thistory\tmodel_calls\n'

done_home=$tmp/corpus/done
show_metrics done "$done_home"
AGENT_BRIEF=brief "$agent" check "$done_home" >/dev/null 2>"$tmp/done.check.stderr"
rm -f "$model_marker"
ASK=$fake_ask AGENT_EVAL_MODEL_MARKER=$model_marker AGENT_BRIEF=brief AGENT_PLY="$ply_program" AGENT_CAGE=cage \
	"$agent" run "$done_home" >/dev/null 2>"$tmp/done.run.stderr"
[ ! -e "$model_marker" ] || {
	printf 'agent eval: already-done re-entry called Ask\n' >&2
	exit 1
}
AGENT_BRIEF=brief AGENT_TRAIL=trail AGENT_ASK=ask \
	"$agent" history "$done_home" check >/dev/null 2>"$tmp/done.history.stderr"
printf '0\t0\t0\t0\n'

quiet_home=$tmp/corpus/quiet-watch
show_metrics quiet-watch "$quiet_home"
AGENT_BRIEF=brief "$agent" check "$quiet_home" >/dev/null 2>"$tmp/quiet.check.stderr"
rm -f "$model_marker"
ASK=$fake_ask AGENT_EVAL_MODEL_MARKER=$model_marker AGENT_PLY=$tmp/missing-ply \
	"$agent" tick "$quiet_home" >/dev/null 2>"$tmp/quiet.tick.stderr"
[ ! -e "$model_marker" ] || {
	printf 'agent eval: quiet tick called Ask\n' >&2
	exit 1
}
printf '0\t0\tn/a\t0\n'

broken_home=$tmp/corpus/broken-watch
show_metrics broken-watch "$broken_home"
AGENT_BRIEF=brief "$agent" check "$broken_home" >/dev/null 2>"$tmp/broken.check.stderr"
set +e
"$agent" tick "$broken_home" >/dev/null 2>"$tmp/broken.tick.stderr"
broken_status=$?
set -e
[ "$broken_status" -eq 2 ] || {
	printf 'agent eval: broken wake exited %s, want 2\n' "$broken_status" >&2
	exit 1
}
printf '0\t2\tn/a\t0\n'

delegator_home=$tmp/corpus/delegator
show_metrics delegator "$delegator_home"
AGENT_BRIEF=brief "$agent" check "$delegator_home" >/dev/null 2>"$tmp/delegator.check.stderr"
AGENT_BRIEF=brief "$agent" check "$delegator_home/agents/researcher" >/dev/null 2>"$tmp/researcher.check.stderr"
printf '0\tn/a\tn/a\t0\n'
