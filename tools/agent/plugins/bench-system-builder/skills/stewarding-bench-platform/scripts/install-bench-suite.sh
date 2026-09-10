#!/bin/sh
set -eu

ME=install-bench-suite.sh
suite_version=0.13.0
release_base=https://github.com/patrickyoung/bench/releases/download/v0.13.0
source_commit=c4e02f1cce7b57265493d95964534e7ae774ad44
mode=${1:-}
[ -n "$mode" ] || mode=help
[ "$mode" = help ] || shift

prefix=
cache=
source_dir=
evidence_out=
approved=0
prefix_set=0
cache_set=0
source_set=0
evidence_set=0

usage() {
  cat <<'EOF'
usage:
  install-bench-suite.sh plan [--prefix ABS] [--cache ABS]
  install-bench-suite.sh download --approve [--cache ABS]
  install-bench-suite.sh install --approve [--prefix ABS] [--cache ABS]
  install-bench-suite.sh verify [--prefix ABS]
  install-bench-suite.sh cage-plan [--prefix ABS] [--evidence-out ABS]
  install-bench-suite.sh cage-check --approve [--prefix ABS] [--evidence-out ABS]
  install-bench-suite.sh source-plan --source ABS [--prefix ABS]
  install-bench-suite.sh source-build --approve --source ABS [--prefix ABS]

The prebuilt release is the ordinary path. plan, verify, and cage-plan are
read-only. download, install, cage-check, and source-build refuse to act without
--approve. cage-check uses transient fixtures and loopback only. source-build
is an explicit engineering fallback and never runs automatically.
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

abspath_required() {
  label=$1
  value=$2
  case $value in
    /*) ;;
    *) usage_fail "$label must be an absolute path" ;;
  esac
  case $value in
    *'
'*) usage_fail "$label may not contain control characters" ;;
  esac
  if printf '%s' "$value" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    usage_fail "$label may not contain control characters"
  fi
}

while [ "$#" -gt 0 ]; do
  case $1 in
    --prefix)
      [ "$#" -ge 2 ] || usage_fail '--prefix needs a directory'
      prefix=$2
      prefix_set=1
      shift 2
      ;;
    --cache)
      [ "$#" -ge 2 ] || usage_fail '--cache needs a directory'
      cache=$2
      cache_set=1
      shift 2
      ;;
    --source)
      [ "$#" -ge 2 ] || usage_fail '--source needs a directory'
      source_dir=$2
      source_set=1
      shift 2
      ;;
    --evidence-out)
      [ "$#" -ge 2 ] || usage_fail '--evidence-out needs a file'
      evidence_out=$2
      evidence_set=1
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
  plan|download|install|verify|cage-plan|cage-check|source-plan|source-build) ;;
  *) usage_fail "unknown operation: $mode" ;;
esac

[ "$prefix_set" -eq 0 ] || [ -n "$prefix" ] || usage_fail '--prefix may not be empty'
[ "$cache_set" -eq 0 ] || [ -n "$cache" ] || usage_fail '--cache may not be empty'
[ "$source_set" -eq 0 ] || [ -n "$source_dir" ] || usage_fail '--source may not be empty'
[ "$evidence_set" -eq 0 ] || [ -n "$evidence_out" ] || usage_fail '--evidence-out may not be empty'
[ "$evidence_set" -eq 0 ] || [ "$mode" = cage-plan ] || [ "$mode" = cage-check ] || \
  usage_fail '--evidence-out is valid only with cage-plan or cage-check'

if [ -z "$prefix" ]; then
  [ -n "${HOME:-}" ] || usage_fail '--prefix is required when HOME is unset'
  prefix=$HOME/.local
fi
if [ -z "$cache" ]; then
  cache=${TMPDIR:-/tmp}/bench-suite-downloads
fi
abspath_required prefix "$prefix"
abspath_required cache "$cache"
[ -z "$source_dir" ] || abspath_required source "$source_dir"
[ -z "$evidence_out" ] || abspath_required evidence-out "$evidence_out"

os=$(uname -s 2>/dev/null || printf unknown)
arch=$(uname -m 2>/dev/null || printf unknown)
case "$os:$arch" in
  Darwin:x86_64)
    target=darwin-amd64
    archive_sha=d9f46e17bf8ed722ef2d635332e0513f6fc158a3bcff63f20ec8555b3da86b04
    manifest_sha=7d3a7a82c32add64558348fc5184ab7a3b772846b6bd078dff162100d381c6ab
    release_go_version=go1.26.7
    ;;
  Darwin:arm64)
    target=darwin-arm64
    archive_sha=8c5b66085523410171ea13e5f813745292f88bc7513bca8d2769b9800c6c6ba6
    manifest_sha=fe0998c77a79bcd4018ee1bd14bd2e74a3afb15f1a84d71d4afee0e038138a5d
    release_go_version=go1.26.5
    ;;
  Linux:x86_64)
    target=linux-amd64
    archive_sha=0fef42246115fc84cc14cef029f46f8347fb6f1737548eadf0da15b1ce9af9d4
    manifest_sha=153f8e02122c8b115fd4adf50f186851b442d2909a72b8b0a31f5b278bb3a951
    release_go_version=go1.26.7
    ;;
  Linux:aarch64|Linux:arm64)
    target=linux-arm64
    archive_sha=6621771d68d1c52dc7939b6db1d994de0a11d186cc2757a8b0edace3a6983965
    manifest_sha=29c65de3166e294eb43dcf0978547b9e01c782e783f4863a7ef76258e487c408
    release_go_version=go1.26.7
    ;;
  *)
    fail "unsupported platform $os/$arch; supported: macOS or Linux on amd64/arm64"
    ;;
esac

archive_name=bench-suite-$suite_version-$target.tar.gz
archive=$cache/$archive_name
archive_url=$release_base/$archive_name
bundle_name=${archive_name%.tar.gz}
install_root=$prefix/lib/bench-suite/$suite_version

print_plan() {
  printf 'operation=%s\n' "$1"
  printf 'observed-platform=%s/%s\n' "$os" "$arch"
  printf 'selected-target=%s\n' "$target"
  printf 'suite-version=%s\n' "$suite_version"
  printf 'archive-url=%s\n' "$archive_url"
  printf 'archive-sha256=%s\n' "$archive_sha"
  printf 'installed-manifest-sha256=%s\n' "$manifest_sha"
  printf 'release-go-version=%s\n' "$release_go_version"
  printf 'download-path=%s\n' "$archive"
  printf 'install-prefix=%s\n' "$prefix"
  printf 'install-root=%s\n' "$install_root"
  printf 'public-bin=%s/bin\n' "$prefix"
}

sha256_file() {
  file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    fail 'sha256sum or shasum is required'
  fi
}

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

case $0 in
  /*) helper_path=$0 ;;
  */*) helper_path=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)/$(basename -- "$0") ;;
  *)
    helper_candidate=$(command -v "$0" 2>/dev/null || true)
    [ -n "$helper_candidate" ] || fail 'cannot resolve this helper for the host handoff command'
    case $helper_candidate in
      /*) helper_path=$helper_candidate ;;
      *) helper_path=$(CDPATH= cd -P -- "$(dirname -- "$helper_candidate")" && pwd)/$(basename -- "$helper_candidate") ;;
    esac
    ;;
esac

shell_quote() {
  escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
  printf "'%s'" "$escaped"
}

path_owner_id() {
  owned_path=$1
  if stat -f '%u' "$owned_path" >/dev/null 2>&1; then
    stat -f '%u' "$owned_path"
  elif stat -c '%u' "$owned_path" >/dev/null 2>&1; then
    stat -c '%u' "$owned_path"
  else
    return 1
  fi
}

prepare_evidence_target() {
  [ -n "$evidence_out" ] || return 0
  evidence_parent=$(dirname -- "$evidence_out")
  evidence_base=$(basename -- "$evidence_out")
  case $evidence_base in .|..) fail '--evidence-out may not end in . or ..' ;; esac
  [ -d "$evidence_parent" ] || fail "evidence parent must already exist: $evidence_parent"
  [ ! -L "$evidence_parent" ] || fail "evidence parent may not be a symlink: $evidence_parent"
  evidence_parent=$(CDPATH= cd -P -- "$evidence_parent" && pwd)
  evidence_out=$evidence_parent/$evidence_base
  evidence_owner=$(path_owner_id "$evidence_parent" 2>/dev/null || true)
  [ "$evidence_owner" = "$(id -u)" ] || fail 'evidence parent must be owned by the current identity'
  [ ! -e "$evidence_out" ] && [ ! -L "$evidence_out" ] || fail "evidence output already exists: $evidence_out"
}

prepare_cache() {
  [ ! -L "$cache" ] || fail "cache directory may not be a symlink: $cache"
  if [ -e "$cache" ]; then
    [ -d "$cache" ] || fail "cache path is not a directory: $cache"
    cache_owner=$(path_owner_id "$cache" 2>/dev/null || true)
    [ -n "$cache_owner" ] || fail 'stat is required to verify cache ownership'
    [ "$cache_owner" = "$(id -u)" ] || fail "cache directory is owned by another identity: $cache"
    chmod 700 "$cache"
  else
    cache_parent=$(dirname -- "$cache")
    [ -d "$cache_parent" ] || fail "cache parent must already exist: $cache_parent"
    umask 077
    mkdir "$cache" || fail "could not create private cache directory: $cache"
    chmod 700 "$cache"
  fi
  [ ! -L "$archive" ] || fail "cached archive may not be a symlink: $archive"
  if [ -e "$archive" ] && [ ! -f "$archive" ]; then
    fail "cached archive path must be absent or a regular file: $archive"
  fi
}

verify_archive() {
  [ -f "$archive" ] || fail "archive is absent: $archive"
  actual=$(sha256_file "$archive")
  [ "$actual" = "$archive_sha" ] || fail "archive checksum mismatch: expected $archive_sha, observed $actual"
}

download_archive() {
  [ "$approved" -eq 1 ] || usage_fail 'download/install requires --approve after the plan is reviewed'
  prepare_cache
  if [ -f "$archive" ]; then
    verify_archive
    printf 'download=reused-verified\n'
    return
  fi
  command -v curl >/dev/null 2>&1 || fail 'curl is required when the pinned archive is not already cached'
  umask 077
  temporary=$(mktemp "$cache/.${archive_name}.part.XXXXXX") || fail 'could not create a private download file'
  trap 'rm -f -- "$temporary"' EXIT HUP INT TERM
  curl -fL --proto '=https' --tlsv1.2 -o "$temporary" "$archive_url"
  actual=$(sha256_file "$temporary")
  [ "$actual" = "$archive_sha" ] || fail "download checksum mismatch: expected $archive_sha, observed $actual"
  [ ! -e "$archive" ] && [ ! -L "$archive" ] || fail "archive cache target appeared during download: $archive"
  ln "$temporary" "$archive" || fail "could not publish the verified archive without replacing an existing path: $archive"
  rm -f -- "$temporary"
  temporary=
  trap - EXIT HUP INT TERM
  printf 'download=verified\n'
}

verify_commands() {
  [ -f "$install_root/suite.json" ] || fail "installed suite manifest is absent: $install_root/suite.json"
  [ -f "$install_root/SHA256SUMS" ] || fail "installed suite checksums are absent: $install_root/SHA256SUMS"
  grep -Eq '"version"[[:space:]]*:[[:space:]]*"0\.13\.0"' "$install_root/suite.json" || \
    fail 'installed suite manifest has the wrong version'
  actual_manifest_sha=$(sha256_file "$install_root/SHA256SUMS")
  [ "$actual_manifest_sha" = "$manifest_sha" ] || \
    fail "installed checksum manifest is not anchored to the pinned $target release"
  printf 'suite-release-manifest=anchored sha256=%s\n' "$actual_manifest_sha"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$install_root" && sha256sum -c SHA256SUMS >/dev/null) || fail 'installed suite checksum verification failed'
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$install_root" && shasum -a 256 -c SHA256SUMS >/dev/null) || fail 'installed suite checksum verification failed'
  else
    fail 'sha256sum or shasum is required'
  fi
  printf 'suite-integrity=verified\n'
  failed=0
  while IFS=' ' read -r command_name expected_version; do
    command_path=$install_root/bin/$command_name
    public_entry=$prefix/bin/$command_name
    if [ ! -x "$command_path" ] || [ ! -x "$public_entry" ]; then
      printf 'command=%s status=missing expected=%s path=%s\n' \
        "$command_name" "$expected_version" "$public_entry"
      failed=1
      continue
    fi
    resolved_entry=$(resolve_path "$public_entry" 2>/dev/null || true)
    resolved_command=$(resolve_path "$command_path" 2>/dev/null || true)
    if [ "$resolved_entry" != "$resolved_command" ]; then
      printf 'command=%s status=wrong-suite expected=%s path=%s\n' \
        "$command_name" "$expected_version" "$public_entry"
      failed=1
      continue
    fi
    if ! output=$("$command_path" version 2>&1); then
      printf 'command=%s status=broken expected=%s path=%s\n' \
        "$command_name" "$expected_version" "$command_path"
      failed=1
      continue
    fi
    actual_version=${output##* }
    if [ "$actual_version" != "$expected_version" ]; then
      printf 'command=%s status=mismatch expected=%s actual=%s path=%s\n' \
        "$command_name" "$expected_version" "$actual_version" "$command_path"
      failed=1
      continue
    fi
    printf 'command=%s status=exact version=%s path=%s\n' \
      "$command_name" "$actual_version" "$command_path"
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
  [ "$failed" -eq 0 ] || return 1
  cage_output=$("$install_root/bin/cage" status 2>&1) || fail 'Cage status discovery failed'
  printf '%s\n' "$cage_output" | grep -Eq '"available"[[:space:]]*:[[:space:]]*true' || \
    fail 'Cage reports no available confinement backend'
  printf '%s\n' "$cage_output" | grep -Eq '"complete"[[:space:]]*:[[:space:]]*true' || \
    fail 'Cage reports an incomplete confinement backend'
  printf '%s\n' "$cage_output"
  printf 'cage-backend=discovered-available-and-complete\n'
  printf 'cage-proof=not-performed\n'
  printf 'runtime=suite-compatible\n'
  printf 'milestone=SUITE-INSTALLED\n'
}

case $mode in
  plan)
    print_plan prebuilt-install
    if [ -d "$install_root" ] && (verify_commands >/dev/null 2>&1); then
      printf 'existing-install=compatible\n'
      printf 'network=none\n'
      printf 'writes=none\n'
      printf 'next=run cage-plan and the approved cage-check on the target host, then resume the saved Bench build gate using %s/bin/bench\n' "$prefix"
      exit 0
    fi
    if [ -e "$install_root" ]; then
      printf 'existing-install=incompatible-or-incomplete\n'
      printf 'next=stop for steward review; do not overwrite or download as repair\n'
      exit 1
    fi
    printf 'existing-install=absent\n'
    if [ -L "$cache" ]; then
      printf 'archive-cache=unsafe-symlink\n'
      printf 'next=choose a private cache directory and rerun plan\n'
      exit 1
    elif [ -f "$archive" ] && [ ! -L "$archive" ]; then
      verify_archive
      printf 'archive-cache=verified\n'
      printf 'network=none\n'
    elif [ -e "$archive" ] || [ -L "$archive" ]; then
      printf 'archive-cache=unsafe-or-not-a-regular-file\n'
      printf 'next=choose a private cache directory and rerun plan\n'
      exit 1
    else
      printf 'archive-cache=absent\n'
      printf 'network=download-one-pinned-https-archive\n'
    fi
    printf 'writes=download-cache,versioned-suite-directory,public-command-symlinks\n'
    printf 'collision-policy=archive-installer-refuses-unrelated-files-or-links\n'
    printf 'rollback=remove-only-reviewed-links-and-%s\n' "$install_root"
    printf 'next=obtain approval, then rerun install with --approve and the same paths\n'
    ;;
  download)
    print_plan download-only
    download_archive
    verify_archive
    ;;
  install)
    print_plan prebuilt-install
    if [ -d "$install_root" ] && (verify_commands >/dev/null 2>&1); then
      printf 'install=noop-already-compatible\n'
      verify_commands
      printf 'next=run cage-plan with a reviewed proof-record path, then the approved target-host cage-check\n'
      exit 0
    fi
    [ ! -e "$install_root" ] || fail 'existing suite root is incompatible or incomplete; stop for steward review'
    download_archive
    verify_archive
    command -v tar >/dev/null 2>&1 || fail 'tar is required'
    listing=$(mktemp "${TMPDIR:-/tmp}/bench-suite-list.XXXXXX")
    extract=$(mktemp -d "${TMPDIR:-/tmp}/bench-suite-extract.XXXXXX")
    cleanup() { rm -f -- "$listing"; rm -rf -- "$extract"; }
    trap cleanup EXIT HUP INT TERM
    tar -tzf "$archive" > "$listing"
    awk -v root="$bundle_name/" '
      index($0, root) != 1 { bad=1 }
      $0 ~ /(^|\/)\.\.($|\/)/ { bad=1 }
      END { exit bad ? 1 : 0 }
    ' "$listing" || fail 'archive contains an unexpected or unsafe path'
    tar -xzf "$archive" -C "$extract"
    [ -x "$extract/$bundle_name/install.sh" ] || fail 'verified archive lacks its installer'
    "$extract/$bundle_name/install.sh" "$prefix"
    verify_commands
    printf 'installed-suite=%s\n' "$install_root"
    printf 'next=run cage-plan, approve cage-check on the target host, then award SUITE-READY only if it passes\n'
    ;;
  verify)
    print_plan verify-installed
    [ -d "$install_root" ] || fail "suite installation is absent: $install_root"
    verify_commands
    printf 'next=run cage-plan with a reviewed proof-record path, then the approved target-host cage-check\n'
    ;;
  cage-plan)
    prepare_evidence_target
    print_plan confinement-proof
    [ -d "$install_root" ] || fail "suite installation is absent: $install_root"
    verify_commands
    if [ -n "$evidence_out" ]; then
      printf 'writes-on-check=isolated-temporary-fixtures,%s\n' "$evidence_out"
      printf 'proof-record=%s\n' "$evidence_out"
    else
      printf 'writes-on-check=isolated-temporary-fixtures-only\n'
      printf 'proof-record=stdout-only\n'
    fi
    printf 'network-on-check=loopback-only\n'
    printf 'credentials-on-check=none\n'
    printf 'host-command=%s cage-check --approve --prefix %s' \
      "$(shell_quote "$helper_path")" "$(shell_quote "$prefix")"
    if [ -n "$evidence_out" ]; then
      printf ' --evidence-out %s' "$(shell_quote "$evidence_out")"
    fi
    printf '\n'
    printf 'next=obtain approval, then rerun cage-check with --approve on the target host\n'
    ;;
  cage-check)
    [ "$approved" -eq 1 ] || usage_fail 'cage-check requires --approve after cage-plan is reviewed'
    prepare_evidence_target
    print_plan confinement-proof
    [ -d "$install_root" ] || fail "suite installation is absent: $install_root"
    verify_commands
    umask 077
    cage_temp_root=$(mktemp -d /tmp/bench-cage-proof.XXXXXX) || fail 'could not create a private Cage proof directory'
    chmod 700 "$cage_temp_root"
    cage_result=$cage_temp_root/cage-output.txt
    cage_record=$cage_temp_root/CAGE-CHECK.txt
    published_record=
    cleanup_cage_result() {
      [ -z "${published_record:-}" ] || rm -f -- "$published_record"
      rm -rf -- "$cage_temp_root"
    }
    trap cleanup_cage_result EXIT HUP INT TERM
    if ! TMPDIR="$cage_temp_root" "$install_root/bin/cage" check >"$cage_result" 2>&1; then
      sed -n '1,160p' "$cage_result" >&2
      printf 'state=CAGE-HOST-CHECK-REQUIRED-OR-CONFINEMENT-BROKEN\n' >&2
      printf 'host-command=%s cage-check --approve --prefix %s' \
        "$(shell_quote "$helper_path")" "$(shell_quote "$prefix")" >&2
      if [ -n "$evidence_out" ]; then
        printf ' --evidence-out %s' "$(shell_quote "$evidence_out")" >&2
      fi
      printf '\n' >&2
      fail 'Cage enforcement check failed; rerun the printed command in a terminal on the target host if this was a nested Claude/Cowork denial; if it fails there, stop for steward repair'
    fi
    grep -Eq '^cage check: 13/13 passed$' "$cage_result" || \
      fail 'Cage enforcement check did not report the complete 13/13 acceptance result'
    proof_host=$(hostname 2>/dev/null || printf unknown)
    [ "$proof_host" != unknown ] || fail 'hostname is required to bind the Cage proof to the target host'
    proof_created=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    {
      sed -n '1,160p' "$cage_result"
      printf 'confinement-proof=passed\n'
      printf 'proof-suite-manifest-sha256=%s\n' "$manifest_sha"
      printf 'proof-platform=%s/%s\n' "$os" "$arch"
      printf 'proof-host=%s\n' "$proof_host"
      printf 'proof-created-utc=%s\n' "$proof_created"
      printf 'runtime=suite-compatible-and-confinement-proved\n'
      printf 'milestone=SUITE-READY\n'
    } > "$cage_record"
    chmod 600 "$cage_record"
    sed -n '1,180p' "$cage_record"
    if [ -n "$evidence_out" ]; then
      published_record=$(mktemp "$evidence_parent/.CAGE-CHECK.part.XXXXXX") || fail 'could not stage the Cage proof record'
      cp "$cage_record" "$published_record"
      chmod 600 "$published_record"
      ln "$published_record" "$evidence_out" || fail 'could not publish Cage proof without replacing an existing path'
      rm -f -- "$published_record"
      published_record=
      printf 'proof-record=%s\n' "$evidence_out"
    fi
    ;;
  source-plan)
    [ -n "$source_dir" ] || usage_fail 'source-plan requires --source ABS'
    printf 'operation=source-build\n'
    printf 'observed-platform=%s/%s\n' "$os" "$arch"
    printf 'selected-target=%s\n' "$target"
    printf 'suite-version=%s\n' "$suite_version"
    printf 'install-prefix=%s\n' "$prefix"
    printf 'install-root=%s\n' "$install_root"
    printf 'public-bin=%s/bin\n' "$prefix"
    printf 'source-checkout=%s\n' "$source_dir"
    printf 'required-source-commit=%s\n' "$source_commit"
    printf 'requirements=clean exact v%s checkout at the pinned commit,%s,Git,tar,network\n' "$suite_version" "$release_go_version"
    printf 'writes=source-.benchpack-cache,temporary-build,versioned-suite-directory,public-command-symlinks\n'
    printf 'fallback=none; source build never runs from prebuilt install failure\n'
    printf 'next=engineering review, then rerun source-build with --approve\n'
    ;;
  source-build)
    [ "$approved" -eq 1 ] || usage_fail 'source-build requires --approve after source-plan is reviewed'
    [ -n "$source_dir" ] || usage_fail 'source-build requires --source ABS'
    [ -x "$source_dir/install.sh" ] || fail 'source checkout lacks executable install.sh'
    for required in go git tar; do
      command -v "$required" >/dev/null 2>&1 || fail "$required is required for a source build"
    done
    checkout_root=$(git -C "$source_dir" rev-parse --show-toplevel 2>/dev/null || true)
    [ "$checkout_root" = "$source_dir" ] || fail 'source must be the root of a Bench Git checkout'
    [ -z "$(git -C "$source_dir" status --porcelain)" ] || fail 'source checkout must be clean'
    tag=$(git -C "$source_dir" describe --tags --exact-match 2>/dev/null || true)
    [ "$tag" = "v$suite_version" ] || fail "source checkout must be exactly tagged v$suite_version"
    head_commit=$(git -C "$source_dir" rev-parse HEAD 2>/dev/null || true)
    [ "$head_commit" = "$source_commit" ] || \
      fail "source checkout must be pinned commit $source_commit; observed ${head_commit:-unknown}"
    go_version=$(go env GOVERSION 2>/dev/null || true)
    [ "$go_version" = "$release_go_version" ] || \
      fail "$release_go_version is required for a release-identical $target source build; observed ${go_version:-unknown}"
    printf 'operation=source-build\n'
    printf 'observed-platform=%s/%s\n' "$os" "$arch"
    printf 'selected-target=%s\n' "$target"
    printf 'suite-version=%s\n' "$suite_version"
    printf 'source-checkout=%s\n' "$source_dir"
    printf 'install-prefix=%s\n' "$prefix"
    if [ -d "$install_root" ] && (verify_commands >/dev/null 2>&1); then
      printf 'install=noop-already-compatible\n'
      verify_commands
      printf 'next=run cage-plan with a reviewed proof-record path, then the approved target-host cage-check\n'
      exit 0
    fi
    [ ! -e "$install_root" ] || fail 'existing suite root is incompatible or incomplete; stop for steward review'
    stage_prefix=$(mktemp -d "${TMPDIR:-/tmp}/bench-source-stage.XXXXXX")
    cleanup_source_stage() { rm -rf -- "$stage_prefix"; }
    trap cleanup_source_stage EXIT HUP INT TERM
    "$source_dir/install.sh" -prefix "$stage_prefix"
    final_prefix=$prefix
    final_install_root=$install_root
    prefix=$stage_prefix
    install_root=$stage_prefix/lib/bench-suite/$suite_version
    verify_commands
    prefix=$final_prefix
    install_root=$final_install_root
    "$stage_prefix/lib/bench-suite/$suite_version/install.sh" "$prefix"
    verify_commands
    printf 'source-build=release-identical-and-verified\n'
    printf 'next=run cage-plan, approve cage-check on the target host, then award SUITE-READY only if it passes\n'
    ;;
esac
