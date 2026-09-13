#!/bin/sh
# Extracted home authoring and controller maintenance. Agent owns validation;
# public Hone, Trail, Action and May commands own their respective mechanisms.

set -eu

ME=hire
CONTEXT_MAX=65536
FILE_MAX=32768
PROPOSAL_MAX=16

say() {
	printf '%s: %s\n' "$ME" "$*" >&2
}

die() {
	say "$*"
	exit 2
}

tool() {
	case $1 in
		AGENT_PLY) value=${AGENT_PLY:-} ;;
		AGENT_BRIEF) value=${AGENT_BRIEF:-} ;;
		AGENT_CAGE) value=${AGENT_CAGE:-} ;;
		AGENT_HONE) value=${AGENT_HONE:-} ;;
		AGENT_TRAIL) value=${AGENT_TRAIL:-} ;;
		AGENT_ASK) value=${AGENT_ASK:-} ;;
		AGENT_MAY) value=${AGENT_MAY:-} ;;
		AGENT_ACTION) value=${AGENT_ACTION:-} ;;
		*) die "internal unknown tool variable: $1" ;;
	esac
	fallback=$2
	if [ -n "$value" ]; then
		command -v "$value" >/dev/null 2>&1 || die "$1 executable not found: $value"
		command -v "$value"
		return
	fi
	command -v "$fallback" >/dev/null 2>&1 || die "$fallback is required"
	command -v "$fallback"
}

physical_dir() {
	[ -d "$1" ] || return 1
	(
		cd -P "$1" 2>/dev/null || exit 1
		pwd -P
	)
}

resolve_program() {
	program=$1
	case $program in
		*/*) ;;
		*) program=$(command -v "$program") || return 1 ;;
	esac
	case $program in
		/*) ;;
		*) program=$PWD/$program ;;
	esac
	hops=0
	while [ -L "$program" ]; do
		hops=$((hops + 1))
		[ "$hops" -le 32 ] || return 1
		program_dir=$(physical_dir "$(dirname "$program")") || return 1
		target=$(readlink "$program") || return 1
		case $target in
			/*) program=$target ;;
			*) program=$program_dir/$target ;;
		esac
	done
	program_dir=$(physical_dir "$(dirname "$program")") || return 1
	printf '%s/%s\n' "$program_dir" "$(basename "$program")"
}

bytes() {
	wc -c <"$1" | tr -d '[:space:]'
}

nonempty_markdown() {
	# Blank lines and one-line Markdown comments do not make an optional file
	# worth injecting. This is a loading choice, never Markdown interpretation.
	sed '/^[[:space:]]*$/d; /^[[:space:]]*<!--[[:space:][:print:]]*-->[[:space:]]*$/d' "$1" | grep -q .
}

safe_file() {
	path=$1
	label=$2
	[ ! -L "$path" ] || { say "$label may not be a symlink: $path"; return 1; }
	[ -f "$path" ] || { say "$label is not a regular file: $path"; return 1; }
	[ -r "$path" ] || { say "$label is not readable: $path"; return 1; }
	return 0
}

safe_dir() {
	path=$1
	label=$2
	[ ! -L "$path" ] || { say "$label may not be a symlink: $path"; return 1; }
	[ -d "$path" ] || { say "$label is not a directory: $path"; return 1; }
	return 0
}

definition_files() {
	for name in AGENTS.md GOAL.md SOUL.md PLAN.md HEARTBEAT.md MEMORY.md; do
		[ -f "$HOME_DIR/$name" ] && printf '%s\n' "$name"
	done
}

hash_stream() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 | awk '{print $1}'
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 | awk '{print $NF}'
	else
		die 'show needs sha256sum, shasum, or openssl'
	fi
}

hash_file() {
	hash_stream <"$1"
}

definition_hash() {
	definition_files | while IFS= read -r name; do
		printf '%s\000' "$name"
		cat "$HOME_DIR/$name"
		printf '\000'
	done | hash_stream
}

check_home() {
	[ -n "${AGENT_RUNTIME:-}" ] || die 'select the installed Agent runner through HIRE_AGENT'
	"$AGENT_RUNTIME" check "$HOME_DIR"
}

new_home() {
	definition_only=0
	if [ "${1:-}" = -definition ]; then
		definition_only=1
		shift
	fi
	dir=${1:-}
	[ -n "$dir" ] || die 'new needs DIR'
	shift
	if [ -e "$dir" ]; then
		[ -d "$dir" ] || die "not a directory: $dir"
		[ -z "$(find "$dir" -mindepth 1 -maxdepth 1 -print -quit)" ] || die "directory is not empty: $dir"
	else
		mkdir -p "$dir" || die "cannot create $dir"
	fi
	desc=$*
	[ -n "$desc" ] || desc='Describe this agent'

	mkdir -p "$dir/bin" "$dir/skills" "$dir/agents" "$dir/tools"
	if [ "$definition_only" -eq 0 ]; then
		mkdir -p "$dir/work/proposals" "$dir/work/actions" "$dir/state/kv" "$dir/.agent/runs" "$dir/.agent/learning/proposals" "$dir/.agent/amendments" "$dir/.agent/checkpoints" "$dir/.agent/selections"
	fi
	cat >"$dir/AGENTS.md" <<EOF
# Operating instructions

$desc.

- Work toward the selected invocation goal and respect its constraints.
- Keep current progress in \$AGENT_STATE/plan.md; do not rewrite definition files.
- Put proposed definition changes under \$AGENT_WORK/proposals/ for human review.
- Put proposed external effects under \$AGENT_WORK/actions/ as strict Action JSON; never invoke a connector directly or claim a proposal happened.
- Inspect state on demand instead of loading it wholesale.
EOF
	cat >"$dir/GOAL.md" <<'EOF'
# Outcome

Replace this with the durable end state.

## Acceptance evidence

Explain what `bin/check` proves.

## Constraints and stop conditions

- Preserve anything that must not regress.
- Stop and report when human judgment or new authority is required.
EOF
	cat >"$dir/SOUL.md" <<'EOF'
# Character

Be calm, direct, curious, and evidence-led.
EOF
	cat >"$dir/PLAN.md" <<'EOF'
# Standing strategy

Start with the cheapest read-only observation that can change the plan.
EOF
	cat >"$dir/MEMORY.md" <<'EOF'
# Curated facts

Keep this small and human-reviewed. Verified procedures belong in skills/.
EOF
	: >"$dir/HEARTBEAT.md"
	if [ "$definition_only" -eq 0 ]; then : >"$dir/state/plan.md"; fi
	cat >"$dir/bin/check" <<'EOF'
#!/bin/sh
# Replace with the executable definition of done. Exit 0 accepts, 1 rejects.
exit 1
EOF
	cat >"$dir/bin/wake" <<'EOF'
#!/bin/sh
# Replace when HEARTBEAT.md is used. Exit 0 is quiet, 1 wakes, other is broken.
exit 0
EOF
	chmod 700 "$dir/bin/check" "$dir/bin/wake"
	if [ "$definition_only" -eq 0 ]; then
		chmod 700 "$dir/.agent" "$dir/.agent/runs" "$dir/.agent/learning" "$dir/.agent/learning/proposals" "$dir/.agent/amendments" "$dir/.agent/checkpoints" "$dir/.agent/selections"
	fi
	printf '%s/AGENTS.md\n' "$dir"
	if [ "$definition_only" -eq 1 ]; then
		say "scaffolded $dir; edit the definition and bin/check, then run agent run -C WORKSPACE $dir -- GOAL"
	else
		say "scaffolded $dir; edit GOAL.md and bin/check, then run agent check $dir"
	fi
}

open_home() {
	raw=${1:-.}
	HOME_DIR=$(physical_dir "$raw") || die "not an agent home directory: $raw"
	export HOME_DIR
}

portable_name() {
	case $1 in
		''|*[!a-z0-9-]*|-*|*-) return 1 ;;
		[a-z]*) return 0 ;;
		*) return 1 ;;
	esac
}

session_in_home() {
	raw=$1
	case $raw in
		*/*) candidate=$raw ;;
		*)
			candidate=$HOME_DIR/.agent/runs/$raw
			[ -e "$candidate" ] || candidate=$candidate.jsonl
			;;
	esac
	[ ! -L "$candidate" ] || die "session may not be a symlink: $candidate"
	parent=$(physical_dir "$(dirname "$candidate")") || die "session directory not found: $candidate"
	SESSION_FILE=$parent/$(basename "$candidate")
	case $SESSION_FILE in
		"$HOME_DIR/.agent/runs/"*) ;;
		*) die "session is outside this home's run evidence: $SESSION_FILE" ;;
	esac
	safe_file "$SESSION_FILE" session || die "invalid session: $SESSION_FILE"
	export SESSION_FILE
}

action_proposal_in_home() {
	raw=$1
	proposal_root=$HOME_DIR/work/actions
	safe_dir "$proposal_root" work/actions || die "invalid action proposal directory: $proposal_root"
	case $raw in
		*/*) candidate=$raw ;;
		*) candidate=$proposal_root/$raw ;;
	esac
	[ ! -L "$candidate" ] || die "action proposal may not be a symlink: $candidate"
	parent=$(physical_dir "$(dirname "$candidate")") || die "action proposal directory not found: $candidate"
	ACTION_PROPOSAL_FILE=$parent/$(basename "$candidate")
	[ "$parent" = "$(physical_dir "$proposal_root")" ] || die "action proposal is outside this home's work/actions: $ACTION_PROPOSAL_FILE"
	proposal_name=$(basename "$ACTION_PROPOSAL_FILE")
	case $proposal_name in
		''|.*|*[!A-Za-z0-9._-]*) die "action proposal needs a portable .json filename: $proposal_name" ;;
		*.json) ;;
		*) die "action proposal needs a portable .json filename: $proposal_name" ;;
	esac
	safe_file "$ACTION_PROPOSAL_FILE" 'action proposal' || die "invalid action proposal: $ACTION_PROPOSAL_FILE"
	if find "$ACTION_PROPOSAL_FILE" -type f -links +1 -print -quit 2>/dev/null | grep -q .; then
		die "action proposal is multiply linked: $ACTION_PROPOSAL_FILE"
	fi
	proposal_bytes=$(bytes "$ACTION_PROPOSAL_FILE")
	[ "$proposal_bytes" -le "$FILE_MAX" ] || die "action proposal is $proposal_bytes bytes; the limit is $FILE_MAX"
	export ACTION_PROPOSAL_FILE
}

print_action_review() {
	action_bin=$1
	proposal_hash=$(hash_file "$ACTION_PROPOSAL_FILE")
	printf '%s\n' 'agent-action-proposal/v1'
	printf 'home: %s\n' "$HOME_DIR"
	printf 'definition-sha256: %s\n' "$(definition_hash)"
	printf 'proposal: %s\n' "$ACTION_PROPOSAL_FILE"
	printf 'proposal-sha256: %s\n' "$proposal_hash"
	printf 'proposal-bytes: %s\n' "$(bytes "$ACTION_PROPOSAL_FILE")"
	printf 'may-job: agent-action-%s-%s\n' "$(definition_hash)" "$proposal_hash"
	printf '%s\n' 'canonical-proposal:'
	"$action_bin" inspect "$ACTION_PROPOSAL_FILE"
}

actions_home() {
	[ "$#" -ge 1 ] && [ "$#" -le 2 ] || die 'actions needs HOME and optional PROPOSAL'
	open_home "$1"
	if ! check_home >/dev/null; then
		return 1
	fi
	action_bin=$(tool AGENT_ACTION action)
	if [ "$#" -eq 2 ]; then
		action_proposal_in_home "$2"
		print_action_review "$action_bin"
		return 0
	fi

	proposal_root=$HOME_DIR/work/actions
	safe_dir "$proposal_root" work/actions || die "invalid action proposal directory: $proposal_root"
	set -- "$proposal_root"/*.json
	if [ ! -e "$1" ] && [ ! -L "$1" ]; then
		printf 'agent-actions/v1\nhome: %s\ndefinition-sha256: %s\ncount: 0\n' "$HOME_DIR" "$(definition_hash)"
		return 0
	fi
	[ "$#" -le "$PROPOSAL_MAX" ] || die "action proposal catalogue has $# files; the limit is $PROPOSAL_MAX"
	proposal_total=0
	printf 'agent-actions/v1\nhome: %s\ndefinition-sha256: %s\ncount: %s\n' "$HOME_DIR" "$(definition_hash)" "$#"
	for proposal_path do
		action_proposal_in_home "$proposal_path"
		proposal_total=$((proposal_total + $(bytes "$ACTION_PROPOSAL_FILE")))
		[ "$proposal_total" -le "$CONTEXT_MAX" ] || die "action proposal bytes total $proposal_total; the limit is $CONTEXT_MAX"
		printf '\n'
		print_action_review "$action_bin"
	done
}

act_home() {
	policy=${AGENT_ACTION_POLICY:-}
	while [ "$#" -gt 0 ]; do
		case $1 in
			-policy) [ "$#" -ge 2 ] || die '-policy needs PROGRAM'; policy=$2; shift 2 ;;
			-*) die "unknown act flag: $1" ;;
			*) break ;;
		esac
	done
	[ "$#" -eq 3 ] || die 'act needs HOME PROPOSAL SESSION'
	open_home "$1"
	if ! check_home; then
		return 1
	fi
	action_proposal_in_home "$2"
	session_in_home "$3"
	action_path=${AGENT_ACTION_PATH:-}
	[ -n "$action_path" ] || die 'act needs AGENT_ACTION_PATH naming controller-only Action connectors'
	action_bin=$(tool AGENT_ACTION action)
	may_bin=$(tool AGENT_MAY may)
	ask_bin=$(tool AGENT_ASK ask)
	proposal_hash=$(hash_file "$ACTION_PROPOSAL_FILE")
	job=agent-action-$(definition_hash)-$proposal_hash
	say "action · home=$HOME_DIR · proposal=$ACTION_PROPOSAL_FILE · evidence=$SESSION_FILE"
	set -- run -job "$job" -may "$may_bin" -ask "$ask_bin" -record "$SESSION_FILE" -proposal "$ACTION_PROPOSAL_FILE"
	[ -z "$policy" ] || set -- "$@" -policy "$policy"
	set +e
	(cd "$HOME_DIR/work" && ACTION_PATH=$action_path "$action_bin" "$@")
	rc=$?
	set -e
	return "$rc"
}

learning_proposal_file() {
	proposal_name=$1
	proposal_mode=$2
	case $proposal_name in
		''|.*|*/*|*[!a-z0-9._-]*) die "learning proposal needs a portable .json filename: $proposal_name" ;;
		*.json) ;;
		*) die "learning proposal needs a portable .json filename: $proposal_name" ;;
	esac
	proposal_root=$HOME_DIR/.agent/learning/proposals
	if [ "$proposal_mode" = create ] && [ ! -e "$proposal_root" ] && [ ! -L "$proposal_root" ]; then
		mkdir "$proposal_root" || die "cannot create learning proposal directory: $proposal_root"
		chmod 700 "$proposal_root"
	fi
	safe_dir "$proposal_root" .agent/learning/proposals || die "invalid learning proposal directory: $proposal_root"
	[ -r "$proposal_root" ] && [ -w "$proposal_root" ] && [ -x "$proposal_root" ] ||
		die "learning proposal directory is not accessible and writable: $proposal_root"
	LEARNING_PROPOSAL_FILE=$proposal_root/$proposal_name
	[ ! -L "$LEARNING_PROPOSAL_FILE" ] || die "learning proposal may not be a symlink: $LEARNING_PROPOSAL_FILE"
	if [ "$proposal_mode" = read ]; then
		safe_file "$LEARNING_PROPOSAL_FILE" 'learning proposal' || die "invalid learning proposal: $LEARNING_PROPOSAL_FILE"
		if find "$LEARNING_PROPOSAL_FILE" -type f -links +1 -print -quit 2>/dev/null | grep -q .; then
			die "learning proposal is multiply linked: $LEARNING_PROPOSAL_FILE"
		fi
	elif [ -e "$LEARNING_PROPOSAL_FILE" ]; then
		die "learning proposal already exists: $LEARNING_PROPOSAL_FILE"
	fi
	export LEARNING_PROPOSAL_FILE
}

learn_home() {
	into=
	model=
	count=
	dry=0
	why=0
	quiet=0
	prepare=
	show=
	admit=
	while [ "$#" -gt 0 ]; do
		case $1 in
			-into) [ "$#" -ge 2 ] || die '-into needs SKILL'; into=$2; shift 2 ;;
			-m) [ "$#" -ge 2 ] || die '-m needs MODEL'; model=$2; shift 2 ;;
			-n) [ "$#" -ge 2 ] || die '-n needs COUNT'; count=$2; shift 2 ;;
			-N) dry=1; shift ;;
			-why) why=1; shift ;;
			-prepare) [ "$#" -ge 2 ] || die '-prepare needs PROPOSAL'; prepare=$2; shift 2 ;;
			-show) [ "$#" -ge 2 ] || die '-show needs PROPOSAL'; show=$2; shift 2 ;;
			-admit) [ "$#" -ge 2 ] || die '-admit needs PROPOSAL'; admit=$2; shift 2 ;;
			-q) quiet=1; shift ;;
			-*) die "unknown learn flag: $1" ;;
			*) break ;;
		esac
	done
	modes=0
	[ -z "$prepare" ] || modes=$((modes + 1))
	[ -z "$show" ] || modes=$((modes + 1))
	[ -z "$admit" ] || modes=$((modes + 1))
	[ "$modes" -le 1 ] || die 'learn accepts only one of -prepare, -show, or -admit'
	if [ -n "$prepare" ] && { [ "$dry" -ne 0 ] || [ "$why" -ne 0 ]; }; then
		die 'learn -prepare cannot be combined with -N or -why'
	fi

	if [ -n "$show" ] || [ -n "$admit" ]; then
		[ -z "$into$model$count" ] && [ "$dry" -eq 0 ] && [ "$why" -eq 0 ] && [ "$quiet" -eq 0 ] ||
			die 'learn -show/-admit do not accept wording flags or -into'
		[ "$#" -eq 1 ] || die 'learn -show/-admit need HOME and no SESSION'
		open_home "$1"
		if ! check_home; then
			return 1
		fi
		proposal_name=$show
		operation=show
		if [ -n "$admit" ]; then
			proposal_name=$admit
			operation=admit
		fi
		learning_proposal_file "$proposal_name" read
		hone_bin=$(tool AGENT_HONE hone)
		export HONE_DIR=$HOME_DIR/.agent/learning
		if [ "$operation" = admit ]; then
			[ -w "$HOME_DIR/skills" ] || die "skills directory is not writable by the controller: $HOME_DIR/skills"
			ask_bin=$(tool AGENT_ASK ask)
			brief_bin=$(tool AGENT_BRIEF brief)
			export ASK=$ask_bin
			export BRIEF=$brief_bin
			export BRIEF_PATH=$HOME_DIR/skills
		fi
		say "learn $operation · proposal=$LEARNING_PROPOSAL_FILE · skill-root=$HOME_DIR/skills"
		set +e
		"$hone_bin" "$operation" "$LEARNING_PROPOSAL_FILE"
		rc=$?
		set -e
		return "$rc"
	fi

	[ -n "$into" ] || die 'learn requires -into SKILL'
	portable_name "$into" || die "learning skill name is not portable: $into"
	[ "$#" -eq 2 ] || die 'learn needs HOME and SESSION'
	open_home "$1"
	if ! check_home; then
		return 1
	fi
	session_in_home "$2"
	[ -w "$HOME_DIR/skills" ] || die "skills directory is not writable by the controller: $HOME_DIR/skills"

	hone_bin=$(tool AGENT_HONE hone)
	brief_bin=$(tool AGENT_BRIEF brief)
	ask_bin=$(tool AGENT_ASK ask)
	export ASK=$ask_bin
	export BRIEF=$brief_bin
	export BRIEF_PATH=$HOME_DIR/skills
	export HONE_DIR=$HOME_DIR/.agent/learning
	set -- -into "$into"
	[ -z "$model" ] || set -- "$@" -m "$model"
	[ -z "$count" ] || set -- "$@" -n "$count"
	[ "$dry" -eq 0 ] || set -- "$@" -N
	[ "$why" -eq 0 ] || set -- "$@" -why
	if [ -n "$prepare" ]; then
		learning_proposal_file "$prepare" create
		set -- "$@" -prepare "$LEARNING_PROPOSAL_FILE"
	fi
	[ "$quiet" -eq 0 ] || set -- "$@" -q
	set -- "$@" "$SESSION_FILE"
	say "learn · session=$SESSION_FILE · skill=$HOME_DIR/skills/$into · evidence=$HOME_DIR/.agent/learning${prepare:+ · proposal=$LEARNING_PROPOSAL_FILE}"
	set +e
	"$hone_bin" "$@"
	rc=$?
	set -e
	return "$rc"
}

history_home() {
	[ "$#" -ge 1 ] || die 'history needs HOME'
	open_home "$1"
	shift
	if ! check_home >/dev/null; then
		return 1
	fi

	trail_bin=$(tool AGENT_TRAIL trail)
	archive=$HOME_DIR/.agent/runs
	history_command=${1:-ls}
	[ "$#" -eq 0 ] || shift
	case $history_command in
		ls)
			[ "$#" -eq 0 ] || die 'history ls takes no arguments'
			set -- ls "$archive"
			;;
		find)
			[ "$#" -eq 1 ] || die 'history find needs one quoted QUERY'
			[ -n "$1" ] || die 'history find query is empty'
			set -- find "$1" "$archive"
			;;
		show)
			[ "$#" -eq 1 ] || die 'history show needs SESSION'
			session_in_home "$1"
			set -- show "$SESSION_FILE"
			;;
		window)
			before=
			after=
			while [ "$#" -gt 0 ]; do
				case $1 in
					-before|-after)
						flag=$1
						[ "$#" -ge 2 ] || die "history window $flag needs N"
						case $2 in ''|*[!0-9]*) die "history window $flag must be a non-negative integer" ;; esac
						if [ "$flag" = -before ]; then before=$2; else after=$2; fi
						shift 2
						;;
					-*) die "unknown history window flag: $1" ;;
					*) break ;;
				esac
			done
			[ "$#" -eq 2 ] || die 'history window needs SESSION and SEQ'
			session_in_home "$1"
			seq=$2
			case $seq in ''|*[!0-9]*) die 'history window SEQ must be a positive integer' ;; esac
			[ "$seq" -gt 0 ] || die 'history window SEQ must be a positive integer'
			set -- window
			[ -z "$before" ] || set -- "$@" -before "$before"
			[ -z "$after" ] || set -- "$@" -after "$after"
			set -- "$@" "$SESSION_FILE" "$seq"
			;;
		lineage)
			[ "$#" -eq 1 ] || die 'history lineage needs SESSION'
			session_in_home "$1"
			set -- lineage "$SESSION_FILE" "$archive"
			;;
		check)
			[ "$#" -eq 0 ] || die 'history check takes no arguments'
			ask_bin=$(tool AGENT_ASK ask)
			export ASK=$ask_bin
			set -- check "$archive"
			;;
		*) die "unknown history command: $history_command" ;;
	esac
	say "history · home=$HOME_DIR · evidence=$archive · command=$history_command"
	set +e
	"$trail_bin" "$@"
	rc=$?
	set -e
	return "$rc"
}

proposal_in_home() {
	raw=$1
	proposal_root=$HOME_DIR/work/proposals
	safe_dir "$proposal_root" work/proposals || die "invalid proposal directory: $proposal_root"
	case $raw in
		*/*) candidate=$raw ;;
		*) candidate=$proposal_root/$raw ;;
	esac
	[ ! -L "$candidate" ] || die "proposal may not be a symlink: $candidate"
	parent=$(physical_dir "$(dirname "$candidate")") || die "proposal directory not found: $candidate"
	PROPOSAL_FILE=$parent/$(basename "$candidate")
	[ "$parent" = "$(physical_dir "$proposal_root")" ] || die "proposal is outside this home's work/proposals: $PROPOSAL_FILE"
	proposal_name=$(basename "$PROPOSAL_FILE")
	case $proposal_name in
		''|.*|*[!A-Za-z0-9._-]*) die "proposal needs a portable .patch filename: $proposal_name" ;;
		*.patch) ;;
		*) die "proposal needs a portable .patch filename: $proposal_name" ;;
	esac
	safe_file "$PROPOSAL_FILE" proposal || die "invalid proposal: $PROPOSAL_FILE"
	proposal_bytes=$(bytes "$PROPOSAL_FILE")
	[ "$proposal_bytes" -le "$FILE_MAX" ] || die "proposal is $proposal_bytes bytes; the limit is $FILE_MAX"
	export PROPOSAL_FILE
}

inspect_amendment() {
	command -v git >/dev/null 2>&1 || die 'amend needs git for strict patch parsing and application'

	stats=$(cd "$HOME_DIR" && git apply --numstat "$PROPOSAL_FILE") || die 'proposal is not a parseable patch'
	[ "$(printf '%s\n' "$stats" | sed '/^$/d' | wc -l | tr -d '[:space:]')" -eq 1 ] || die 'amend accepts exactly one definition file per proposal'
	AMEND_TARGET=$(printf '%s\n' "$stats" | awk -F '\t' 'NF == 3 { print $3 }')
	case $AMEND_TARGET in
		AGENTS.md|GOAL.md|SOUL.md|PLAN.md|HEARTBEAT.md|MEMORY.md) ;;
		*) die "amend target is not a supported root definition file: $AMEND_TARGET" ;;
	esac
	[ ! -L "$HOME_DIR/$AMEND_TARGET" ] || die "amend target may not be a symlink: $AMEND_TARGET"
	safe_file "$HOME_DIR/$AMEND_TARGET" 'amend target' || die "invalid amend target: $AMEND_TARGET"
	if find "$HOME_DIR/$AMEND_TARGET" -type f -links +1 -print -quit 2>/dev/null | grep -q .; then
		die "amend target is multiply linked: $AMEND_TARGET"
	fi
	(cd "$HOME_DIR" && git apply --check "$PROPOSAL_FILE") || die 'proposal does not apply cleanly to the current definition'

	AMEND_BEFORE_HASH=$(definition_hash)
	AMEND_PROPOSAL_HASH=$(hash_file "$PROPOSAL_FILE")
	AMEND_JOB=agent-amend-$AMEND_BEFORE_HASH-$AMEND_PROPOSAL_HASH
	AMEND_ACTION=$(printf '%s\n' \
		'agent-amend/v1' \
		"home: $HOME_DIR" \
		"definition-sha256: $AMEND_BEFORE_HASH" \
		"target: $AMEND_TARGET" \
		"proposal: $PROPOSAL_FILE" \
		"proposal-sha256: $AMEND_PROPOSAL_HASH" \
		'effect: apply this one root definition-file patch, then require agent check')
}

print_proposal_review() {
	inspect_amendment
	printf '%s\n' 'agent-proposal/v1'
	printf 'home: %s\n' "$HOME_DIR"
	printf 'definition-sha256: %s\n' "$AMEND_BEFORE_HASH"
	printf 'proposal: %s\n' "$PROPOSAL_FILE"
	printf 'proposal-sha256: %s\n' "$AMEND_PROPOSAL_HASH"
	printf 'proposal-bytes: %s\n' "$(bytes "$PROPOSAL_FILE")"
	printf 'target: %s\n' "$AMEND_TARGET"
	printf 'may-job: %s\n' "$AMEND_JOB"
	printf 'may-action-sha256: %s\n' "$(printf '%s\n' "$AMEND_ACTION" | hash_stream)"
	printf 'may-action (one final LF is included):\n%s\n' "$AMEND_ACTION"
	printf '%s\n' 'patch-bytes:'
	cat "$PROPOSAL_FILE"
	printf '\n'
}

proposals_home() {
	[ "$#" -ge 1 ] && [ "$#" -le 2 ] || die 'proposals needs HOME and optional PATCH'
	open_home "$1"
	if ! check_home >/dev/null; then
		return 1
	fi
	if [ "$#" -eq 2 ]; then
		proposal_in_home "$2"
		print_proposal_review
		return 0
	fi

	proposal_root=$HOME_DIR/work/proposals
	safe_dir "$proposal_root" work/proposals || die "invalid proposal directory: $proposal_root"
	set -- "$proposal_root"/*.patch
	if [ ! -e "$1" ] && [ ! -L "$1" ]; then
		printf 'agent-proposals/v1\nhome: %s\ndefinition-sha256: %s\ncount: 0\n' "$HOME_DIR" "$(definition_hash)"
		return 0
	fi
	[ "$#" -le "$PROPOSAL_MAX" ] || die "proposal catalogue has $# patches; the limit is $PROPOSAL_MAX"
	proposal_total=0
	printf 'agent-proposals/v1\nhome: %s\ndefinition-sha256: %s\ncount: %s\n' "$HOME_DIR" "$(definition_hash)" "$#"
	for proposal_path do
		proposal_in_home "$proposal_path"
		proposal_total=$((proposal_total + $(bytes "$PROPOSAL_FILE")))
		[ "$proposal_total" -le "$CONTEXT_MAX" ] || die "proposal bytes total $proposal_total; the limit is $CONTEXT_MAX"
		printf '\n'
		print_proposal_review
	done
}

amend_home() {
	[ "$#" -eq 2 ] || die 'amend needs HOME and PATCH'
	open_home "$1"
	if ! check_home; then
		return 1
	fi
	proposal_in_home "$2"
	inspect_amendment
	target=$AMEND_TARGET
	before_hash=$AMEND_BEFORE_HASH
	proposal_hash=$AMEND_PROPOSAL_HASH
	job=$AMEND_JOB
	action=$AMEND_ACTION

	receipt_dir=$HOME_DIR/.agent/amendments
	if [ ! -e "$receipt_dir" ]; then
		mkdir "$receipt_dir" || die "cannot create amendment evidence: $receipt_dir"
		chmod 700 "$receipt_dir"
	fi
	safe_dir "$receipt_dir" .agent/amendments || die "invalid amendment evidence directory: $receipt_dir"
	[ -r "$receipt_dir" ] && [ -w "$receipt_dir" ] && [ -x "$receipt_dir" ] ||
		die "amendment evidence is not accessible and writable: $receipt_dir"

	may_bin=$(tool AGENT_MAY may)
	set +e
	may_result=$(printf '%s\n' "$action" | "$may_bin" request "$job")
	may_status=$?
	set -e
	[ -z "$may_result" ] || printf '%s\n' "$may_result"
	[ "$may_status" -eq 0 ] || return "$may_status"

	[ "$(definition_hash)" = "$before_hash" ] || die 'definition changed after approval was requested; refusing stale grant'
	[ "$(hash_file "$PROPOSAL_FILE")" = "$proposal_hash" ] || die 'proposal changed after approval was requested; refusing stale grant'
	(cd "$HOME_DIR" && git apply --check "$PROPOSAL_FILE") || die 'proposal no longer applies cleanly after approval'

	tmp_parent=${TMPDIR:-/tmp}
	backup_dir=$(mktemp -d "$tmp_parent/agent-amend.XXXXXX") || die 'cannot create amendment backup'
	receipt_tmp=$(mktemp "$receipt_dir/.receipt.XXXXXX") || {
		rm -rf "$backup_dir"
		die 'cannot stage amendment receipt'
	}
	applied=0
	committed=0
	cleanup_amend() {
		if [ "$applied" -eq 1 ] && [ "$committed" -eq 0 ]; then
			cp -p "$backup_dir/$target" "$HOME_DIR/$target" 2>/dev/null || :
		fi
		rm -f "$receipt_tmp"
		rm -rf "$backup_dir"
	}
	trap cleanup_amend EXIT
	trap 'exit 129' HUP
	trap 'exit 130' INT
	trap 'exit 143' TERM
	chmod 600 "$receipt_tmp"
	cp -p "$HOME_DIR/$target" "$backup_dir/$target" || die 'cannot back up amendment target'
	if ! (cd "$HOME_DIR" && git apply "$PROPOSAL_FILE"); then
		die 'approved proposal could not be applied'
	fi
	applied=1
	if ! check_home; then
		cp -p "$backup_dir/$target" "$HOME_DIR/$target" || die 'amendment invalid and rollback failed'
		applied=0
		say 'approved amendment failed agent check and was rolled back'
		return 2
	fi
	after_hash=$(definition_hash)
	{
		printf '%s\n' 'agent-amend/v1'
		printf 'home: %s\n' "$HOME_DIR"
		printf 'target: %s\n' "$target"
		printf 'proposal: %s\n' "$PROPOSAL_FILE"
		printf 'proposal-sha256: %s\n' "$proposal_hash"
		printf 'definition-before-sha256: %s\n' "$before_hash"
		printf 'definition-after-sha256: %s\n' "$after_hash"
		printf 'may-result: %s\n' "$may_result"
	} >"$receipt_tmp"
	receipt=$receipt_dir/$proposal_hash.txt
	mv "$receipt_tmp" "$receipt" || die 'cannot publish amendment receipt'
	committed=1
	say "amended $target · before=$before_hash · after=$after_hash · evidence=$receipt"
}

cmd=${1:-help}
case $cmd in
	new)
		shift
		new_home "$@"
		;;
	learn)
		shift
		learn_home "$@"
		;;
	history)
		shift
		history_home "$@"
		;;
	actions)
		shift
		actions_home "$@"
		;;
	act)
		shift
		act_home "$@"
		;;
	proposals)
		shift
		proposals_home "$@"
		;;
	amend)
		shift
		amend_home "$@"
		;;
	*)
		die "unknown command: $cmd (run hire help)"
		;;
esac
