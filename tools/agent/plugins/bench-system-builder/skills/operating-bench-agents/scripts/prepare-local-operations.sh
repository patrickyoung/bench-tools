#!/bin/sh
set -eu

ME=prepare-local-operations.sh
mode=${1:-}
[ -n "$mode" ] || mode=help
[ "$mode" = help ] || shift

home_dir=
out_dir=
bench_bin=
agent_name=
model=
checkpoint=
cage_evidence=
minutes=60
approved=0
bench_set=0
name_set=0
model_set=0
checkpoint_set=0
run_cage_check=0
cage_proved=0

usage() {
  cat <<'EOF'
usage:
  prepare-local-operations.sh plan --home ABS --out ABS [--bench ABS]
      [--name SAFE-NAME] [--model provider/model] [--checkpoint SAFE-NAME] [--minutes N]
      [--run-cage-check | --cage-proved --cage-evidence ABS]
  prepare-local-operations.sh render --approve --home ABS --out ABS [--bench ABS]
      [--name SAFE-NAME] --model provider/model [--checkpoint SAFE-NAME] [--minutes N]
      [--run-cage-check | --cage-proved --cage-evidence ABS]

plan is read-only. render writes a local runbook and a disabled launchd or
systemd-user schedule for the suite's shipped Agent. It never activates the
schedule, stores credentials, adds network, disables Cage, or creates a runner.
--run-cage-check proves confinement with transient fixtures before rendering.
Use the external-evidence form only after the same target host passed Cage check.
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

while [ "$#" -gt 0 ]; do
  case $1 in
    --home)
      [ "$#" -ge 2 ] || usage_fail '--home needs a directory'
      home_dir=$2
      shift 2
      ;;
    --out)
      [ "$#" -ge 2 ] || usage_fail '--out needs a directory'
      out_dir=$2
      shift 2
      ;;
    --bench)
      [ "$#" -ge 2 ] || usage_fail '--bench needs an executable path'
      bench_bin=$2
      bench_set=1
      shift 2
      ;;
    --name)
      [ "$#" -ge 2 ] || usage_fail '--name needs a value'
      agent_name=$2
      name_set=1
      shift 2
      ;;
    --model)
      [ "$#" -ge 2 ] || usage_fail '--model needs provider/model'
      model=$2
      model_set=1
      shift 2
      ;;
    --checkpoint)
      [ "$#" -ge 2 ] || usage_fail '--checkpoint needs a value'
      checkpoint=$2
      checkpoint_set=1
      shift 2
      ;;
    --run-cage-check)
      run_cage_check=1
      shift
      ;;
    --cage-proved)
      cage_proved=1
      shift
      ;;
    --cage-evidence)
      [ "$#" -ge 2 ] || usage_fail '--cage-evidence needs a file'
      cage_evidence=$2
      shift 2
      ;;
    --minutes)
      [ "$#" -ge 2 ] || usage_fail '--minutes needs a positive integer'
      minutes=$2
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
  help|-h|--help)
    usage
    exit 0
    ;;
  plan|render) ;;
  *) usage_fail "unknown operation: $mode" ;;
esac

[ "$bench_set" -eq 0 ] || [ -n "$bench_bin" ] || usage_fail '--bench may not be empty'
[ "$name_set" -eq 0 ] || [ -n "$agent_name" ] || usage_fail '--name may not be empty'
[ "$model_set" -eq 0 ] || [ -n "$model" ] || usage_fail '--model may not be empty'
[ "$checkpoint_set" -eq 0 ] || [ -n "$checkpoint" ] || usage_fail '--checkpoint may not be empty'
[ "$run_cage_check" -eq 0 ] || [ "$cage_proved" -eq 0 ] || usage_fail 'choose --run-cage-check or --cage-proved, not both'
[ "$cage_proved" -eq 0 ] || [ -n "$cage_evidence" ] || usage_fail '--cage-proved requires --cage-evidence ABS'
[ -z "$cage_evidence" ] || [ "$cage_proved" -eq 1 ] || usage_fail '--cage-evidence requires --cage-proved'

[ -n "$home_dir" ] || usage_fail '--home is required'
[ -n "$out_dir" ] || usage_fail '--out is required'
case $home_dir in /*) ;; *) usage_fail '--home must be an absolute path' ;; esac
case $out_dir in /*) ;; *) usage_fail '--out must be an absolute path' ;; esac
for checked_path in "$home_dir" "$out_dir"; do
  case $checked_path in
    *'
'*) usage_fail 'paths may not contain control characters' ;;
  esac
  if printf '%s' "$checked_path" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    usage_fail 'paths may not contain control characters'
  fi
done
[ -d "$home_dir" ] || fail "Agent home is absent: $home_dir"
physical_home=$(CDPATH= cd -P -- "$home_dir" && pwd)
home_dir=$physical_home
if [ -n "$cage_evidence" ]; then
  case $cage_evidence in /*) ;; *) usage_fail '--cage-evidence must be an absolute path' ;; esac
  case $cage_evidence in
    *'
'*) usage_fail '--cage-evidence may not contain control characters' ;;
  esac
  if printf '%s' "$cage_evidence" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    usage_fail '--cage-evidence may not contain control characters'
  fi
  [ -f "$cage_evidence" ] && [ ! -L "$cage_evidence" ] || fail 'Cage evidence must be a regular, non-symlink file'
  cage_evidence_dir=$(CDPATH= cd -P -- "$(dirname -- "$cage_evidence")" && pwd)
  cage_evidence=$cage_evidence_dir/$(basename -- "$cage_evidence")
  case $cage_evidence in
    "$physical_home"|"$physical_home"/*) fail 'Cage proof evidence must be outside the Agent home' ;;
  esac
fi
out_parent=$(dirname -- "$out_dir")
out_base=$(basename -- "$out_dir")
case $out_base in .|..) usage_fail '--out may not end in . or ..' ;; esac
[ -d "$out_parent" ] || usage_fail '--out parent must already exist so its ownership and symlinks can be checked'
physical_out_parent=$(CDPATH= cd -P -- "$out_parent" && pwd)
physical_out=$physical_out_parent/$out_base
[ ! -e "$out_dir" ] || fail "output path already exists; choose a fresh directory: $out_dir"
case $physical_out in
  "$physical_home"|"$physical_home"/*)
    fail 'controller output must be outside the Agent home'
    ;;
esac
out_dir=$physical_out

if [ -z "$bench_bin" ]; then
  bench_bin=$(command -v bench 2>/dev/null || true)
fi
[ -n "$bench_bin" ] || fail 'Bench is not available; pass its absolute --bench path'
case $bench_bin in /*) ;; *) usage_fail '--bench must resolve to an absolute path' ;; esac
case $bench_bin in
  *'
'*) usage_fail '--bench may not contain control characters' ;;
esac
if printf '%s' "$bench_bin" | LC_ALL=C grep -q '[[:cntrl:]]'; then
  usage_fail '--bench may not contain control characters'
fi
[ -x "$bench_bin" ] || fail "Bench is not executable: $bench_bin"

resolve_path() {
  candidate=$1
  link_hops=0
  while [ -L "$candidate" ]; do
    link_hops=$((link_hops + 1))
    [ "$link_hops" -le 40 ] || return 1
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

physical_bench=$(resolve_path "$bench_bin" 2>/dev/null || true)
[ -n "$physical_bench" ] || fail "cannot resolve Bench executable: $bench_bin"
suite_root=$(dirname -- "$(dirname -- "$physical_bench")")
[ -f "$suite_root/suite.json" ] || fail "the selected Bench is not in a suite: $bench_bin"
grep -Eq '"version"[[:space:]]*:[[:space:]]*"0\.13\.0"' "$suite_root/suite.json" || \
  fail 'the selected Bench is not from suite 0.13.0'
[ -f "$suite_root/SHA256SUMS" ] || fail 'the selected suite lacks internal checksums'
case $suite_root in
  "$physical_home"|"$physical_home"/*)
    fail 'the controller suite must be outside the Agent home'
    ;;
esac
if command -v sha256sum >/dev/null 2>&1; then
  manifest_sha=$(sha256sum "$suite_root/SHA256SUMS" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  manifest_sha=$(shasum -a 256 "$suite_root/SHA256SUMS" | awk '{print $1}')
else
  fail 'sha256sum or shasum is required'
fi
case $manifest_sha in
  7d3a7a82c32add64558348fc5184ab7a3b772846b6bd078dff162100d381c6ab|\
  fe0998c77a79bcd4018ee1bd14bd2e74a3afb15f1a84d71d4afee0e038138a5d|\
  153f8e02122c8b115fd4adf50f186851b442d2909a72b8b0a31f5b278bb3a951|\
  29c65de3166e294eb43dcf0978547b9e01c782e783f4863a7ef76258e487c408) ;;
  *) fail 'the selected suite checksum manifest is not anchored to a pinned 0.13.0 release' ;;
esac
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$suite_root" && sha256sum -c SHA256SUMS >/dev/null) || fail 'suite checksum verification failed'
else
  (cd "$suite_root" && shasum -a 256 -c SHA256SUMS >/dev/null) || fail 'suite checksum verification failed'
fi
bench_bin=$physical_bench
agent_bin=$suite_root/bin/agent
[ -x "$agent_bin" ] || fail "the selected suite lacks Agent: $agent_bin"

bench_version=$("$physical_bench" version 2>/dev/null || true)
agent_version=$("$agent_bin" version 2>/dev/null || true)
case $bench_version in *' 0.7.0') ;; *) fail "expected Bench 0.7.0; observed ${bench_version:-broken}" ;; esac
case $agent_version in *' 0.2.1') ;; *) fail "expected Agent 0.2.1; observed ${agent_version:-broken}" ;; esac
if ! cage_status=$("$suite_root/bin/cage" status 2>&1); then
  fail 'Cage confinement is unavailable; scheduled Agent runs would not be ready'
fi
printf '%s\n' "$cage_status" | grep -Eq '"available"[[:space:]]*:[[:space:]]*true' || \
  fail 'Cage reports no available confinement backend; scheduled Agent runs would not be ready'
printf '%s\n' "$cage_status" | grep -Eq '"complete"[[:space:]]*:[[:space:]]*true' || \
  fail 'Cage reports an incomplete confinement backend; scheduled Agent runs would not be ready'

if [ -z "$agent_name" ]; then
  agent_name=$(basename -- "$home_dir")
fi
if ! printf '%s\n' "$agent_name" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*$'; then
  usage_fail '--name must contain only letters, numbers, dot, underscore, or hyphen'
fi
if ! printf '%s\n' "$(basename -- "$home_dir")" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*$'; then
  usage_fail 'the Agent home directory name must not look like a flag and may contain only letters, numbers, dot, underscore, or hyphen'
fi
if ! printf '%s\n' "$minutes" | LC_ALL=C grep -Eq '^[1-9][0-9]*$'; then
  usage_fail '--minutes must be a positive integer'
fi
[ "$minutes" -le 10080 ] || usage_fail '--minutes may not exceed one week'
if [ -n "$model" ] && ! printf '%s\n' "$model" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:/-]*$'; then
  usage_fail '--model must be a non-secret provider/model identifier'
fi
if [ -n "$checkpoint" ] && ! printf '%s\n' "$checkpoint" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*$'; then
  usage_fail '--checkpoint must contain only letters, numbers, dot, underscore, or hyphen'
fi

os=$(uname -s 2>/dev/null || printf unknown)
arch=$(uname -m 2>/dev/null || printf unknown)
proof_host=$(hostname 2>/dev/null || printf unknown)
[ "$proof_host" != unknown ] || fail 'hostname is required to bind a Cage proof to this host'
case $os in
  Darwin) scheduler=launchd ;;
  Linux) scheduler=systemd-user ;;
  *) fail "unsupported local scheduler platform: $os" ;;
esac

validate_cage_record() {
  record=$1
  record_size=$(wc -c < "$record" | tr -d '[:space:]')
  [ -n "$record_size" ] && [ "$record_size" -le 65536 ] || \
    fail 'Cage evidence must be no larger than 64 KiB'
  grep -Eq '^cage check: 13/13 passed$' "$record" || \
    fail 'Cage evidence does not contain the complete 13/13 acceptance result'
  grep -Eq '^confinement-proof=passed$' "$record" || \
    fail 'Cage evidence is not a successful steward cage-check transcript'
  grep -Fqx "proof-suite-manifest-sha256=$manifest_sha" "$record" || \
    fail 'Cage evidence was not produced for this exact pinned suite manifest'
  grep -Fqx "proof-platform=$os/$arch" "$record" || \
    fail 'Cage evidence was not produced for this target platform'
  grep -Fqx "proof-host=$proof_host" "$record" || \
    fail 'Cage evidence was not produced on this target host'
  grep -Eq '^proof-created-utc=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' "$record" || \
    fail 'Cage evidence lacks a valid UTC creation timestamp'
  grep -Eq '^runtime=suite-compatible-and-confinement-proved$' "$record" || \
    fail 'Cage evidence lacks the steward readiness result'
  grep -Eq '^milestone=SUITE-READY$' "$record" || \
    fail 'Cage evidence lacks the steward SUITE-READY milestone'
}

if [ -n "$cage_evidence" ] && [ "$mode" = plan ]; then
  validate_cage_record "$cage_evidence"
fi
if [ "$scheduler" = systemd-user ]; then
  for systemd_path in "$home_dir" "$out_dir" "$bench_bin"; do
    case $systemd_path in *'$'*|*'%'*) fail 'systemd paths may not contain $ or % specifier characters' ;; esac
  done
fi

parent_dir=$(dirname -- "$home_dir")
home_base=$(basename -- "$home_dir")
label=io.bench.agent.$agent_name

if ! "$agent_bin" check "$home_dir" >/dev/null; then
  fail 'agent check rejected the home; repair it before preparing operations'
fi

if [ -n "$model" ]; then
  model_state=declared
else
  model_state=not-declared
fi

printf 'operation=%s\n' "$mode"
printf 'runtime=bench-suite-agent\n'
printf 'bench=%s\n' "$bench_bin"
printf 'agent=%s\n' "$agent_bin"
printf 'home=%s\n' "$home_dir"
printf 'home-check=valid\n'
printf 'suite-release-manifest=anchored sha256=%s\n' "$manifest_sha"
printf 'cage-backend=discovered-available-and-complete\n'
if [ "$run_cage_check" -eq 1 ]; then
  printf 'cage-proof=will-run-on-render\n'
elif [ "$cage_proved" -eq 1 ]; then
  printf 'cage-proof=operator-attested-same-host-record path=%s\n' "$cage_evidence"
else
  printf 'cage-proof=pending\n'
fi
printf 'platform=%s\n' "$os"
printf 'scheduler=%s\n' "$scheduler"
printf 'schedule-operation=bench-home-agent-tick\n'
printf 'schedule-interval-minutes=%s\n' "$minutes"
if [ -n "$checkpoint" ]; then
  printf 'schedule-checkpoint=%s\n' "$checkpoint"
else
  printf 'schedule-checkpoint=none-distinct-runs\n'
fi
if [ -n "$model" ]; then
  printf 'model-identifier=%s\n' "$model"
else
  printf 'model-identifier=%s\n' "$model_state"
fi
printf 'output-directory=%s\n' "$out_dir"
printf 'activation=not-performed\n'
printf 'credentials=not-read-or-written\n'

if [ "$mode" = plan ]; then
  printf 'writes-on-render=optional-transient-cage-fixtures,runbook,disabled-scheduler-definition,activation-instructions,log-directory\n'
  printf 'network-on-render=loopback-only-when-run-cage-check-is-selected\n'
  if [ -z "$model" ]; then
    printf 'readiness=MODEL-IDENTIFIER-REQUIRED\n'
    printf 'next=name the approved non-secret provider/model, then prove Cage, wake fixtures, and fresh-process model access before render\n'
  else
    printf 'next=prove Cage, wake fixtures, and fresh-process model access; approve render; inspect files; separately approve activation\n'
  fi
  exit 0
fi

[ "$approved" -eq 1 ] || usage_fail 'render requires --approve after the plan is reviewed'
[ -n "$model" ] || usage_fail 'render requires --model because a fresh checkpoint-free scheduler process has no inherited model selection'
[ "$run_cage_check" -eq 1 ] || [ "$cage_proved" -eq 1 ] || \
  usage_fail 'render requires --run-cage-check, or --cage-proved with a reviewed --cage-evidence file'
umask 077
cage_temp_root=$(mktemp -d "$physical_out_parent/.bench-cage-proof.XXXXXX") || \
  fail 'could not create a private Cage proof directory beside the controller output'
chmod 700 "$cage_temp_root"
cage_check_record=$cage_temp_root/CAGE-CHECK.txt
cleanup_cage_record() { rm -rf -- "$cage_temp_root"; }
trap cleanup_cage_record EXIT HUP INT TERM
if [ "$run_cage_check" -eq 1 ]; then
  cage_raw=$cage_temp_root/cage-output.txt
  if ! TMPDIR="$cage_temp_root" "$suite_root/bin/cage" check >"$cage_raw" 2>&1; then
    sed -n '1,160p' "$cage_raw" >&2
    printf 'state=CAGE-HOST-CHECK-REQUIRED-OR-CONFINEMENT-BROKEN\n' >&2
    printf 'next=rerun this exact approved render command in a terminal on the target host; if it fails there, stop for steward repair; never add -no-cage\n' >&2
    fail 'Cage enforcement check failed; no schedule files were rendered'
  fi
  grep -Eq '^cage check: 13/13 passed$' "$cage_raw" || \
    fail 'Cage enforcement check did not report the complete 13/13 acceptance result'
  proof_created=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  {
    sed -n '1,160p' "$cage_raw"
    printf 'confinement-proof=passed\n'
    printf 'proof-suite-manifest-sha256=%s\n' "$manifest_sha"
    printf 'proof-platform=%s/%s\n' "$os" "$arch"
    printf 'proof-host=%s\n' "$proof_host"
    printf 'proof-created-utc=%s\n' "$proof_created"
    printf 'runtime=suite-compatible-and-confinement-proved\n'
    printf 'milestone=SUITE-READY\n'
  } > "$cage_check_record"
  chmod 600 "$cage_check_record"
  validate_cage_record "$cage_check_record"
  cage_proof_state=passed-current-host-check
elif [ "$cage_proved" -eq 1 ]; then
  dd if="$cage_evidence" of="$cage_check_record" bs=65537 count=1 2>/dev/null || \
    fail 'could not snapshot the Cage evidence'
  chmod 600 "$cage_check_record"
  validate_cage_record "$cage_check_record"
  cage_proof_state=operator-attested-same-host-record
fi
mkdir "$out_dir" || fail "could not create fresh controller output: $out_dir"
mkdir "$out_dir/logs" || fail "could not create controller log directory: $out_dir/logs"
chmod 700 "$out_dir" "$out_dir/logs"
cp "$cage_check_record" "$out_dir/CAGE-CHECK.txt"
chmod 600 "$out_dir/CAGE-CHECK.txt"
rm -rf -- "$cage_temp_root"
trap - EXIT HUP INT TERM

xml_escape() {
  printf '%s' "$1" | sed \
    -e 's/&/\&amp;/g' \
    -e 's/</\&lt;/g' \
    -e 's/>/\&gt;/g' \
    -e 's/"/\&quot;/g' \
    -e "s/'/\\&apos;/g"
}

systemd_quote() {
  printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/^/"/' -e 's/$/"/'
}

shell_quote() {
  escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
  printf "'%s'" "$escaped"
}

model_arg_xml=
model_arg_systemd=
model_arg_text=
checkpoint_arg_xml=
checkpoint_arg_systemd=
checkpoint_arg_text=
if [ -n "$model" ]; then
  model_arg_xml="    <string>-m</string>\n    <string>$(xml_escape "$model")</string>\n"
  model_arg_systemd=" -m $(systemd_quote "$model")"
  model_arg_text=" -m $model"
fi
if [ -n "$checkpoint" ]; then
  checkpoint_arg_xml="    <string>-checkpoint</string>\n    <string>$(xml_escape "$checkpoint")</string>\n"
  checkpoint_arg_systemd=" -checkpoint $(systemd_quote "$checkpoint")"
  checkpoint_arg_text=" -checkpoint $checkpoint"
fi

interval_seconds=$((minutes * 60))
if [ "$scheduler" = launchd ]; then
  user_id=$(id -u)
  schedule_file=$out_dir/$label.plist
  {
    printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>'
    printf '%s\n' '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
    printf '%s\n' '<plist version="1.0">'
    printf '%s\n' '<dict>'
    printf '%s\n' '  <key>Label</key>' "  <string>$(xml_escape "$label")</string>"
    printf '%s\n' '  <key>ProgramArguments</key>' '  <array>'
    printf '    <string>%s</string>\n' "$(xml_escape "$bench_bin")"
    printf '%s\n' '    <string>home</string>' '    <string>-C</string>'
    printf '    <string>%s</string>\n' "$(xml_escape "$parent_dir")"
    printf '%s\n' '    <string>tick</string>' '    <string>-q</string>'
    [ -z "$checkpoint_arg_xml" ] || printf '%b' "$checkpoint_arg_xml"
    [ -z "$model_arg_xml" ] || printf '%b' "$model_arg_xml"
    printf '    <string>%s</string>\n' "$(xml_escape "$home_base")"
    printf '%s\n' '  </array>'
    printf '%s\n' '  <key>StartInterval</key>' "  <integer>$interval_seconds</integer>"
    printf '%s\n' '  <key>RunAtLoad</key>' '  <false/>'
    printf '%s\n' '  <key>KeepAlive</key>' '  <false/>'
    printf '%s\n' '  <key>StandardOutPath</key>' "  <string>$(xml_escape "$out_dir/logs/stdout.log")</string>"
    printf '%s\n' '  <key>StandardErrorPath</key>' "  <string>$(xml_escape "$out_dir/logs/stderr.log")</string>"
    printf '%s\n' '</dict>' '</plist>'
  } > "$schedule_file"
  activation_file=$out_dir/ACTIVATE.txt
  {
    printf 'Review %s, then copy it to:\n' "$schedule_file"
    printf '  ~/Library/LaunchAgents/%s.plist\n\n' "$label"
    printf 'Activate only after a separate approval and fresh-process provider test:\n'
    printf '  launchctl bootstrap gui/%s "$HOME/Library/LaunchAgents/%s.plist"\n' "$user_id" "$label"
    printf 'Run once / inspect / pause:\n'
    printf '  launchctl kickstart gui/%s/%s\n' "$user_id" "$label"
    printf '  launchctl print gui/%s/%s\n' "$user_id" "$label"
    printf '  launchctl bootout gui/%s "$HOME/Library/LaunchAgents/%s.plist"\n' "$user_id" "$label"
  } > "$activation_file"
else
  service_name=bench-agent-$agent_name.service
  timer_name=bench-agent-$agent_name.timer
  schedule_file=$out_dir/$service_name
  {
    printf '%s\n' '[Unit]' "Description=Bench Agent tick for $agent_name" '' '[Service]' 'Type=oneshot'
    printf 'ExecStart=%s home -C %s tick -q%s%s %s\n' \
      "$(systemd_quote "$bench_bin")" "$(systemd_quote "$parent_dir")" \
      "$checkpoint_arg_systemd" "$model_arg_systemd" "$(systemd_quote "$home_base")"
    printf '%s\n' 'Restart=no'
    printf 'StandardOutput=%s\n' "$(systemd_quote "append:$out_dir/logs/stdout.log")"
    printf 'StandardError=%s\n' "$(systemd_quote "append:$out_dir/logs/stderr.log")"
  } > "$schedule_file"
  {
    printf '%s\n' '[Unit]' "Description=Schedule Bench Agent tick for $agent_name" '' '[Timer]'
    printf '%s\n' 'OnBootSec=5m' "OnUnitActiveSec=${minutes}m" 'Persistent=false' 'RandomizedDelaySec=0'
    printf '%s\n' "Unit=$service_name" '' '[Install]' 'WantedBy=timers.target'
  } > "$out_dir/$timer_name"
  activation_file=$out_dir/ACTIVATE.txt
  {
    printf 'Review %s and %s, then copy them to:\n' "$schedule_file" "$out_dir/$timer_name"
    printf '  ~/.config/systemd/user/\n\n'
    printf 'Activate only after a separate approval and fresh-process provider test:\n'
    printf '  systemctl --user daemon-reload\n'
    printf '  systemctl --user enable --now %s\n' "$timer_name"
    printf 'Run once / inspect / pause:\n'
    printf '  systemctl --user start %s\n' "$service_name"
    printf '  systemctl --user status %s\n' "$service_name"
    printf '  systemctl --user list-timers %s\n' "$timer_name"
    printf '  systemctl --user disable --now %s\n' "$timer_name"
    printf '  systemctl --user stop %s\n' "$service_name"
  } > "$activation_file"
fi

runbook=$out_dir/LOCAL-RUNBOOK.md
bench_q=$(shell_quote "$bench_bin")
parent_q=$(shell_quote "$parent_dir")
home_base_q=$(shell_quote "$home_base")
{
  printf '# Local runbook: %s\n\n' "$agent_name"
  printf 'This home runs on the Bench suite Agent system. Claude Code or Cowork may help operate it; neither replaces the runtime.\n\n'
  printf '## Fixed paths\n\n'
  printf -- '- Bench: `%s`\n' "$bench_bin"
  printf -- '- Agent: `%s`\n' "$agent_bin"
  printf -- '- Agent home: `%s`\n' "$home_dir"
  printf -- '- Controller files and logs: `%s`\n' "$out_dir"
  printf -- '- Cage enforcement proof: `%s/CAGE-CHECK.txt` (`%s`)\n' "$out_dir" "$cage_proof_state"
  if [ -n "$checkpoint" ]; then
    printf -- '- Scheduled checkpoint: `%s` (explicit same-conversation continuity)\n\n' "$checkpoint"
  else
    printf -- '- Scheduled checkpoint: none; each wake starts a distinct run\n\n'
  fi
  printf '## Inspect\n\n```text\n%s home -C %s check %s\n%s home -C %s show %s\n%s home -C %s history %s check\n```\n\n' \
    "$bench_q" "$parent_q" "$home_base_q" "$bench_q" "$parent_q" "$home_base_q" \
    "$bench_q" "$parent_q" "$home_base_q"
  printf '## Interactive\n\nRun this in a real terminal:\n\n```text\n%s -C %s%s -home %s\n```\n\n' \
    "$bench_q" "$parent_q" "$model_arg_text" "$home_base_q"
  printf '## On demand\n\nA distinct run:\n\n```text\n%s home -C %s run%s %s\n```\n\n' \
    "$bench_q" "$parent_q" "$model_arg_text" "$home_base_q"
  printf 'Continue one named conversation only when that is intended:\n\n```text\n%s home -C %s run -checkpoint CASE-NAME%s %s\n```\n\n' \
    "$bench_q" "$parent_q" "$model_arg_text" "$home_base_q"
  printf '## Cheap wake\n\nAfter quiet, wake, and broken wake fixtures pass:\n\n```text\n%s home -C %s tick%s%s %s\n```\n\n' \
    "$bench_q" "$parent_q" "$checkpoint_arg_text" "$model_arg_text" "$home_base_q"
  printf '## Schedule\n\nThe rendered `%s` definition is disabled. It invokes the pinned suite `bench home … tick`, which transparently supplies the same shipped Agent and its exact companions; it is not another runtime or scheduler. Review `ACTIVATE.txt`. Prove that a fresh scheduler process can reach the selected provider without putting a secret in this directory, the Agent home, argv, logs, plist, or unit. Activation is a separate approval. This pilot scaffold does not itself enforce maximum runtime or model cost, rotate logs, or send failure notifications; the surrounding platform must supply those controls before production admission.\n\n' "$scheduler"
  printf '## Pause and retire\n\nDisable the outer timer first, then stop any active service. Inspect Agent history, May/Action receipts, and external systems before retrying interrupted or unknown work. Retirement additionally reconciles effects, revokes identities and connections, applies evidence retention, and records owner sign-off; deleting this home alone is not retirement.\n'
} > "$runbook"

printf 'rendered-runbook=%s\n' "$runbook"
printf 'rendered-schedule=%s\n' "$schedule_file"
printf 'rendered-activation-instructions=%s\n' "$activation_file"
printf 'cage-proof=%s record=%s/CAGE-CHECK.txt\n' "$cage_proof_state" "$out_dir"
printf 'schedule-active=no\n'
