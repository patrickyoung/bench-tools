#!/bin/sh
set -eu

# Offline, read-only Bench suite compatibility check for skill release 0.2.0.
# It reads only public runtime metadata, command versions, uname, and Cage status.

ME=doctor.sh
prefix=
suite_dir=
bench_entry=

usage() {
  cat <<'EOF'
usage: doctor.sh [--prefix ABS | --suite-dir ABS | --bench ABS]

With no option, inspect the Bench selected by PATH. A compatible runtime must
be one complete Bench suite 0.13.0, not matching standalone command versions.
EOF
}

fail_usage() {
  printf '%s: %s\n' "$ME" "$*" >&2
  usage >&2
  exit 2
}

selection_count=0
prefix_selected=0
while [ "$#" -gt 0 ]; do
  case $1 in
    --prefix)
      [ "$#" -ge 2 ] || fail_usage '--prefix needs a directory'
      prefix=$2
      [ -n "$prefix" ] || fail_usage '--prefix may not be empty'
      prefix_selected=1
      selection_count=$((selection_count + 1))
      shift 2
      ;;
    --suite-dir)
      [ "$#" -ge 2 ] || fail_usage '--suite-dir needs a directory'
      suite_dir=$2
      [ -n "$suite_dir" ] || fail_usage '--suite-dir may not be empty'
      selection_count=$((selection_count + 1))
      shift 2
      ;;
    --bench)
      [ "$#" -ge 2 ] || fail_usage '--bench needs an executable path'
      bench_entry=$2
      [ -n "$bench_entry" ] || fail_usage '--bench may not be empty'
      selection_count=$((selection_count + 1))
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) fail_usage "unknown option: $1" ;;
  esac
done
[ "$selection_count" -le 1 ] || fail_usage 'choose only one runtime selector'

for selected_path in "$prefix" "$suite_dir" "$bench_entry"; do
  [ -z "$selected_path" ] && continue
  case $selected_path in /*) ;; *) fail_usage 'runtime selector paths must be absolute' ;; esac
  case $selected_path in
    *'
'*) fail_usage 'runtime selector paths may not contain control characters' ;;
  esac
  if printf '%s' "$selected_path" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    fail_usage 'runtime selector paths may not contain control characters'
  fi
done

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

if [ -n "$prefix" ]; then
  bench_entry=$prefix/bin/bench
elif [ -n "$suite_dir" ]; then
  bench_entry=$suite_dir/bin/bench
elif [ -z "$bench_entry" ]; then
  bench_entry=$(command -v bench 2>/dev/null || true)
fi

printf 'surface=local-shell\n'
if command -v uname >/dev/null 2>&1; then
  printf 'os=%s\n' "$(uname -s 2>/dev/null || printf unknown)"
  printf 'arch=%s\n' "$(uname -m 2>/dev/null || printf unknown)"
else
  printf 'os=unknown\narch=unknown\n'
fi

if [ -z "$bench_entry" ] || [ ! -x "$bench_entry" ]; then
  printf 'runtime=missing\n'
  exit 1
fi
case $bench_entry in /*) ;; *) bench_entry=$(command -v "$bench_entry" 2>/dev/null || true) ;; esac
physical_bench=$(resolve_path "$bench_entry" 2>/dev/null || true)
[ -n "$physical_bench" ] || {
  printf 'runtime=broken-entry\n'
  exit 1
}
suite_root=$(dirname -- "$(dirname -- "$physical_bench")")
printf 'bench-entry=%s\n' "$bench_entry"
printf 'suite-root=%s\n' "$suite_root"

if [ ! -f "$suite_root/suite.json" ] || [ ! -f "$suite_root/SHA256SUMS" ]; then
  printf 'runtime=not-a-suite\n'
  exit 1
fi
if ! grep -Eq '"version"[[:space:]]*:[[:space:]]*"0\.13\.0"' "$suite_root/suite.json"; then
  printf 'runtime=suite-version-mismatch\n'
  exit 1
fi

sha256_file() {
  file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    return 1
  fi
}

manifest_sha=$(sha256_file "$suite_root/SHA256SUMS" 2>/dev/null || true)
case $manifest_sha in
  7d3a7a82c32add64558348fc5184ab7a3b772846b6bd078dff162100d381c6ab|\
  fe0998c77a79bcd4018ee1bd14bd2e74a3afb15f1a84d71d4afee0e038138a5d|\
  153f8e02122c8b115fd4adf50f186851b442d2909a72b8b0a31f5b278bb3a951|\
  29c65de3166e294eb43dcf0978547b9e01c782e783f4863a7ef76258e487c408)
    printf 'suite-release-manifest=anchored sha256=%s\n' "$manifest_sha"
    ;;
  '')
    printf 'runtime=checksum-tool-missing\n'
    exit 1
    ;;
  *)
    printf 'runtime=unanchored-checksum-manifest\n'
    exit 1
    ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  if ! (cd "$suite_root" && sha256sum -c SHA256SUMS >/dev/null); then
    printf 'runtime=suite-checksum-failed\n'
    exit 1
  fi
elif command -v shasum >/dev/null 2>&1; then
  if ! (cd "$suite_root" && shasum -a 256 -c SHA256SUMS >/dev/null); then
    printf 'runtime=suite-checksum-failed\n'
    exit 1
  fi
else
  printf 'runtime=checksum-tool-missing\n'
  exit 1
fi
printf 'suite-integrity=verified\n'

mismatch=0
while IFS=' ' read -r command_name expected_version; do
  [ -n "$command_name" ] || continue
  command_path=$suite_root/bin/$command_name
  if [ "$prefix_selected" -eq 1 ]; then
    public_entry=$prefix/bin/$command_name
    if [ ! -x "$public_entry" ]; then
      printf 'command=%s status=missing-public-entry expected=%s path=%s\n' \
        "$command_name" "$expected_version" "$public_entry"
      mismatch=1
      continue
    fi
    resolved_entry=$(resolve_path "$public_entry" 2>/dev/null || true)
    resolved_command=$(resolve_path "$command_path" 2>/dev/null || true)
    if [ "$resolved_entry" != "$resolved_command" ]; then
      printf 'command=%s status=wrong-public-entry expected=%s path=%s\n' \
        "$command_name" "$expected_version" "$public_entry"
      mismatch=1
      continue
    fi
  fi
  if [ ! -x "$command_path" ]; then
    printf 'command=%s status=missing expected=%s path=%s\n' \
      "$command_name" "$expected_version" "$command_path"
    mismatch=1
    continue
  fi
  if ! version_output=$("$command_path" version 2>&1); then
    printf 'command=%s status=broken expected=%s path=%s\n' \
      "$command_name" "$expected_version" "$command_path"
    mismatch=1
    continue
  fi
  actual_version=${version_output##* }
  if [ "$actual_version" = "$expected_version" ]; then
    command_status=exact
  else
    command_status=mismatch
    mismatch=1
  fi
  printf 'command=%s status=%s expected=%s actual=%s path=%s\n' \
    "$command_name" "$command_status" "$expected_version" \
    "$actual_version" "$command_path"
done <<'VERSIONS'
bench 0.7.0
ask 0.2.0
brief 0.1.1
ply 0.1.2
context 0.1.0
action 0.1.0
cite 0.1.0
cage 0.1.0
may 0.1.0
hone 0.2.0
trail 0.1.0
agent 0.2.1
tend 0.1.2
draft 1.0.0
mcp 0.3.0
mcpbox 0.3.0
mcpserve 0.3.0
oauth 0.1.1
VERSIONS

[ "$mismatch" -eq 0 ] || {
  printf 'runtime=version-mismatch\n'
  exit 1
}

if ! cage_output=$("$suite_root/bin/cage" status 2>&1); then
  printf 'cage-status=unavailable\n'
  printf 'runtime=cage-unavailable\n'
  exit 1
fi
if ! printf '%s\n' "$cage_output" | grep -Eq '"available"[[:space:]]*:[[:space:]]*true'; then
  printf 'cage-status=unavailable detail=%s\n' "$cage_output"
  printf 'runtime=cage-unavailable\n'
  exit 1
fi
if ! printf '%s\n' "$cage_output" | grep -Eq '"complete"[[:space:]]*:[[:space:]]*true'; then
  printf 'cage-status=incomplete detail=%s\n' "$cage_output"
  printf 'runtime=cage-incomplete\n'
  exit 1
fi
printf 'cage-status=available detail=%s\n' "$cage_output"
printf 'cage-proof=not-performed\n'
printf 'runtime=suite-compatible\n'
printf 'milestone=SUITE-INSTALLED\n'
printf 'next=run the steward helper cage-plan with a reviewed proof-record path, then its approved target-host cage-check; award SUITE-READY only after 13/13 passes\n'
