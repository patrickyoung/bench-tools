#!/bin/sh
set -eu

ME=probe-model-access.sh
mode=${1:-}
[ -n "$mode" ] || mode=help
[ "$mode" = help ] || shift

ask_bin=
model=
out_dir=
timeout_seconds=60
approved=0

usage() {
  cat <<'EOF'
usage:
  probe-model-access.sh plan --ask ABS --model provider/model --out ABS [--timeout-seconds N]
  probe-model-access.sh run --approve --ask ABS --model provider/model --out ABS [--timeout-seconds N]

plan is read-only. run performs one approved, schema-constrained provider call
through the pinned suite Ask, with an empty system prompt, no attachments,
no tools, and a 60-second default timeout. It never prints or records credential
values and never retains provider error bodies. The evidence root is controller-owned and can
retain multiple attempts for the same exact resume command.
EOF
}

fail() {
  printf '%s: %s\n' "$ME" "$*" >&2
  exit 1
}

usage_fail() {
  printf '%s: %s\n' "$ME" "$*" >&2
  usage >&2
  exit 2
}

abspath() {
  case $2 in /*) ;; *) usage_fail "$1 must be an absolute path" ;; esac
  if printf '%s' "$2" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    usage_fail "$1 may not contain control characters"
  fi
}

shell_quote() {
  escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
  printf "'%s'" "$escaped"
}

resolve_path() {
  candidate=$1
  hops=0
  while [ -L "$candidate" ]; do
    hops=$((hops + 1))
    [ "$hops" -le 40 ] || return 1
    link_dir=$(CDPATH= cd -P -- "$(dirname -- "$candidate")" && pwd)
    link_target=$(readlink "$candidate")
    case $link_target in
      /*) candidate=$link_target ;;
      *) candidate=$link_dir/$link_target ;;
    esac
  done
  resolved_dir=$(CDPATH= cd -P -- "$(dirname -- "$candidate")" 2>/dev/null && pwd) || return 1
  printf '%s/%s\n' "$resolved_dir" "$(basename -- "$candidate")"
}

path_owner_id() {
  if stat -f '%u' "$1" >/dev/null 2>&1; then
    stat -f '%u' "$1"
  else
    stat -c '%u' "$1" 2>/dev/null
  fi
}

while [ "$#" -gt 0 ]; do
  case $1 in
    --ask)
      [ "$#" -ge 2 ] || usage_fail '--ask needs an executable path'
      ask_bin=$2
      shift 2
      ;;
    --model)
      [ "$#" -ge 2 ] || usage_fail '--model needs provider/model'
      model=$2
      shift 2
      ;;
    --out)
      [ "$#" -ge 2 ] || usage_fail '--out needs a directory'
      out_dir=$2
      shift 2
      ;;
    --timeout-seconds)
      [ "$#" -ge 2 ] || usage_fail '--timeout-seconds needs a positive integer'
      timeout_seconds=$2
      shift 2
      ;;
    --approve)
      approved=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) usage_fail "unknown option: $1" ;;
  esac
done

case $mode in
  help|-h|--help) usage; exit 0 ;;
  plan|run) ;;
  *) usage_fail "unknown operation: $mode" ;;
esac

[ -n "$ask_bin" ] || usage_fail '--ask is required'
[ -n "$model" ] || usage_fail '--model is required'
[ -n "$out_dir" ] || usage_fail '--out is required'
abspath --ask "$ask_bin"
abspath --out "$out_dir"
[ -x "$ask_bin" ] || fail "Ask is not executable: $ask_bin"
printf '%s\n' "$model" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:/-]*$' || \
  usage_fail '--model must be a non-secret provider/model identifier'
printf '%s\n' "$timeout_seconds" | LC_ALL=C grep -Eq '^[1-9][0-9]*$' || \
  usage_fail '--timeout-seconds must be a positive integer'
[ "$timeout_seconds" -le 300 ] || usage_fail '--timeout-seconds may not exceed 300'

case $0 in
  /*) helper_path=$0 ;;
  */*) helper_path=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)/$(basename -- "$0") ;;
  *)
    helper_candidate=$(command -v "$0" 2>/dev/null || true)
    [ -n "$helper_candidate" ] || fail 'cannot resolve this helper for the resume command'
    case $helper_candidate in
      /*) helper_path=$helper_candidate ;;
      *) helper_path=$(CDPATH= cd -P -- "$(dirname -- "$helper_candidate")" && pwd)/$(basename -- "$helper_candidate") ;;
    esac
    ;;
esac
helper_path=$(resolve_path "$helper_path" 2>/dev/null || true)
[ -n "$helper_path" ] || fail 'cannot resolve this helper for its doctor and resume command'

physical_ask=$(resolve_path "$ask_bin" 2>/dev/null || true)
[ -n "$physical_ask" ] || fail "cannot resolve Ask executable: $ask_bin"
ask_bin=$physical_ask
ask_version=$("$ask_bin" version 2>/dev/null || true)
case $ask_version in *' 0.2.0') ;; *) fail "expected Ask 0.2.0; observed ${ask_version:-broken}" ;; esac
suite_root=$(dirname -- "$(dirname -- "$ask_bin")")
doctor=$(dirname -- "$helper_path")/doctor.sh
[ -x "$doctor" ] || fail "runtime doctor is absent: $doctor"
"$doctor" --suite-dir "$suite_root" >/dev/null || fail 'the selected Ask does not belong to the exact compatible suite'

out_parent=$(dirname -- "$out_dir")
out_base=$(basename -- "$out_dir")
case $out_base in .|..) usage_fail '--out may not end in . or ..' ;; esac
[ -d "$out_parent" ] || usage_fail '--out parent must already exist'
[ ! -L "$out_parent" ] || fail '--out parent may not be a symlink'
out_parent=$(CDPATH= cd -P -- "$out_parent" && pwd)
out_parent_owner=$(path_owner_id "$out_parent" 2>/dev/null || true)
[ "$out_parent_owner" = "$(id -u)" ] || fail '--out parent must be owned by the current identity'
out_dir=$out_parent/$out_base
if [ -e "$out_dir" ]; then
  [ -d "$out_dir" ] && [ ! -L "$out_dir" ] || fail 'model-probe output must be a regular directory'
  out_owner=$(path_owner_id "$out_dir" 2>/dev/null || true)
  [ "$out_owner" = "$(id -u)" ] || fail 'existing model-probe output must be owned by the current identity'
  [ -f "$out_dir/.bench-model-probe-root" ] && [ ! -L "$out_dir/.bench-model-probe-root" ] || \
    fail 'existing output is not a model-probe evidence root'
fi

printf 'operation=%s\n' "$mode"
printf 'runtime=bench-suite-ask\n'
printf 'ask=%s\n' "$ask_bin"
printf 'model-identifier=%s\n' "$model"
printf 'output-root=%s\n' "$out_dir"
printf 'provider-calls-on-run=one\n'
printf 'probe=empty-system,no-attachments,no-tools,const-string-schema\n'
printf 'timeout-seconds=%s\n' "$timeout_seconds"
printf 'credentials=used-by-ask-but-never-read-printed-or-recorded-by-helper\n'
printf 'provider-errors=exit-status-only-bodies-not-retained\n'
printf 'claim=current-process-provider-route-only\n'
printf 'writes-on-run=private-attempt-directory,schema,temporary-session-and-response,sanitized-result\n'
printf 'resume-command=%s run --approve --ask %s --model %s --out %s --timeout-seconds %s\n' \
  "$(shell_quote "$helper_path")" "$(shell_quote "$ask_bin")" \
  "$(shell_quote "$model")" "$(shell_quote "$out_dir")" "$timeout_seconds"

[ "$mode" = run ] || exit 0
[ "$approved" -eq 1 ] || usage_fail 'run requires --approve after the plan is reviewed'

umask 077
if [ ! -e "$out_dir" ]; then
  mkdir "$out_dir" || fail "could not create model-probe evidence root: $out_dir"
  chmod 700 "$out_dir"
  printf '%s\n' 'Bench model-probe evidence root v1' > "$out_dir/.bench-model-probe-root"
  chmod 600 "$out_dir/.bench-model-probe-root"
fi
attempt_id=$(date -u '+%Y%m%dT%H%M%SZ')-$$
attempt=$out_dir/$attempt_id
mkdir "$attempt" "$attempt/sessions"
chmod 700 "$attempt" "$attempt/sessions"
schema=$attempt/readiness.schema.json
response=$attempt/response.json
timed_out=$attempt/timed-out
printf '%s\n' '{"type":"string","const":"READY"}' > "$schema"
chmod 600 "$schema"

"$ask_bin" -d "$attempt/sessions" -m "$model" -S '' -schema "$schema" -q -- \
  'Return the JSON string "READY". Do not add any other text.' > "$response" 2>/dev/null &
ask_pid=$!
watchdog_pid=
cleanup_probe_processes() {
  [ -z "${ask_pid:-}" ] || kill -TERM "$ask_pid" 2>/dev/null || true
  [ -z "${watchdog_pid:-}" ] || kill "$watchdog_pid" 2>/dev/null || true
}
trap cleanup_probe_processes EXIT HUP INT TERM
(
  sleep "$timeout_seconds"
  if kill -0 "$ask_pid" 2>/dev/null; then
    : > "$timed_out"
    kill -TERM "$ask_pid" 2>/dev/null || true
  fi
) &
watchdog_pid=$!
if wait "$ask_pid"; then
  ask_status=0
else
  ask_status=$?
fi
kill "$watchdog_pid" 2>/dev/null || true
wait "$watchdog_pid" 2>/dev/null || true
ask_pid=
watchdog_pid=
trap - EXIT HUP INT TERM
chmod 600 "$response"

response_size=$(wc -c < "$response" | tr -d '[:space:]')
probe_result=failed
if [ "$ask_status" -eq 0 ] && [ ! -e "$timed_out" ] && [ "$response_size" -le 1024 ] && \
  grep -Eq '^("READY"|READY)[[:space:]]*$' "$response"; then
  probe_result=passed
fi

result=$attempt/MODEL-PROBE.txt
{
  printf 'model-identifier=%s\n' "$model"
  printf 'ask-version=0.2.0\n'
  printf 'ask-exit-status=%s\n' "$ask_status"
  printf 'timed-out=%s\n' "$([ -e "$timed_out" ] && printf yes || printf no)"
  printf 'response-bytes=%s\n' "$response_size"
  printf 'probe-result=%s\n' "$probe_result"
  printf 'claim=current-process-provider-route-only\n'
} > "$result"
chmod 600 "$result"
rm -f -- "$timed_out"
rm -rf -- "$attempt/sessions"
if [ "$probe_result" != passed ]; then
  rm -f -- "$response"
fi

printf 'evidence=%s\n' "$result"
if [ "$probe_result" = passed ]; then
  printf 'milestone=MODEL-READY\n'
  printf 'next=resume the saved first model-backed Bench gate; prove the same route again inside any fresh scheduler lane\n'
  exit 0
fi
printf 'milestone=MODEL-ACCESS-REQUIRED\n'
printf 'next=have the named provider-access owner repair the controller boundary, then rerun the exact resume-command above\n'
exit 1
