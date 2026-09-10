#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
installer=$root/plugins/bench-system-builder/skills/stewarding-bench-platform/scripts/install-bench-suite.sh
doctor=$root/plugins/bench-system-builder/skills/building-with-bench/scripts/doctor.sh
operator=$root/plugins/bench-system-builder/skills/operating-bench-agents/scripts/prepare-local-operations.sh
model_probe=$root/plugins/bench-system-builder/skills/building-with-bench/scripts/probe-model-access.sh
suite_prefix=${1:-}

temporary=$(mktemp -d "${TMPDIR:-/tmp}/bench-system-builder-test.XXXXXX")
cleanup() { rm -rf -- "$temporary"; }
trap cleanup EXIT HUP INT TERM
mkdir -p "$temporary/fake-bin"

cat > "$temporary/fake-bin/uname" <<'EOF'
#!/bin/sh
case ${1:-} in
  -s) printf '%s\n' "${BENCH_TEST_OS:-Darwin}" ;;
  -m) printf '%s\n' "${BENCH_TEST_ARCH:-arm64}" ;;
  *) printf '%s %s\n' "${BENCH_TEST_OS:-Darwin}" "${BENCH_TEST_ARCH:-arm64}" ;;
esac
EOF
chmod +x "$temporary/fake-bin/uname"

assert_contains() {
  haystack=$1
  needle=$2
  printf '%s\n' "$haystack" | grep -F "$needle" >/dev/null || {
    printf 'missing expected text: %s\n' "$needle" >&2
    exit 1
  }
}

assert_not_contains() {
  haystack=$1
  needle=$2
  if printf '%s\n' "$haystack" | grep -F "$needle" >/dev/null; then
    printf 'unexpected text: %s\n' "$needle" >&2
    exit 1
  fi
}

permission_mode() {
  if stat -f '%Lp' "$1" >/dev/null 2>&1; then
    stat -f '%Lp' "$1"
  else
    stat -c '%a' "$1"
  fi
}

sha256_file_for_test() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

write_cage_fixture() {
  fixture_path=$1
  fixture_platform=$2
  fixture_manifest=$3
  fixture_host=$4
  printf '%s\n' \
    'cage check: 13/13 passed' \
    'confinement-proof=passed' \
    "proof-suite-manifest-sha256=$fixture_manifest" \
    "proof-platform=$fixture_platform" \
    "proof-host=$fixture_host" \
    'proof-created-utc=2026-09-01T12:00:00Z' \
    'runtime=suite-compatible-and-confinement-proved' \
    'milestone=SUITE-READY' > "$fixture_path"
}

check_target() {
  test_os=$1
  test_arch=$2
  expected_target=$3
  expected_hash=$4
  expected_manifest_hash=$5
  expected_go_version=$6
  test_root=$temporary/plan-$expected_target
  output=$(BENCH_TEST_OS=$test_os BENCH_TEST_ARCH=$test_arch \
    PATH=$temporary/fake-bin:$PATH \
    "$installer" plan --prefix "$test_root/prefix" --cache "$test_root/cache")
  assert_contains "$output" "selected-target=$expected_target"
  assert_contains "$output" "archive-sha256=$expected_hash"
  assert_contains "$output" "installed-manifest-sha256=$expected_manifest_hash"
  assert_contains "$output" "release-go-version=$expected_go_version"
  [ ! -e "$test_root/prefix" ] || { echo 'plan wrote its prefix' >&2; exit 1; }
  [ ! -e "$test_root/cache" ] || { echo 'plan wrote its cache' >&2; exit 1; }
}

check_target Darwin x86_64 darwin-amd64 d9f46e17bf8ed722ef2d635332e0513f6fc158a3bcff63f20ec8555b3da86b04 7d3a7a82c32add64558348fc5184ab7a3b772846b6bd078dff162100d381c6ab go1.26.7
check_target Darwin arm64 darwin-arm64 8c5b66085523410171ea13e5f813745292f88bc7513bca8d2769b9800c6c6ba6 fe0998c77a79bcd4018ee1bd14bd2e74a3afb15f1a84d71d4afee0e038138a5d go1.26.5
check_target Linux x86_64 linux-amd64 0fef42246115fc84cc14cef029f46f8347fb6f1737548eadf0da15b1ce9af9d4 153f8e02122c8b115fd4adf50f186851b442d2909a72b8b0a31f5b278bb3a951 go1.26.7
check_target Linux aarch64 linux-arm64 6621771d68d1c52dc7939b6db1d994de0a11d186cc2757a8b0edace3a6983965 29c65de3166e294eb43dcf0978547b9e01c782e783f4863a7ef76258e487c408 go1.26.7

if BENCH_TEST_OS=Windows_NT BENCH_TEST_ARCH=x86_64 PATH=$temporary/fake-bin:$PATH \
  "$installer" plan --prefix "$temporary/windows-prefix" --cache "$temporary/windows-cache" \
  >/dev/null 2>&1; then
  echo 'unsupported platform was accepted' >&2
  exit 1
fi

if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
  "$installer" download --prefix "$temporary/unapproved-prefix" --cache "$temporary/unapproved-cache" \
  >/dev/null 2>&1; then
  echo 'download ran without approval' >&2
  exit 1
fi
[ ! -e "$temporary/unapproved-cache" ] || { echo 'unapproved download wrote a cache' >&2; exit 1; }

nonregular_cache=$temporary/nonregular-cache
mkdir -p "$nonregular_cache/bench-suite-0.13.0-darwin-arm64.tar.gz"
if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
  "$installer" download --approve --cache "$nonregular_cache" >/dev/null 2>&1; then
  echo 'download accepted a non-regular archive cache target' >&2
  exit 1
fi

if "$installer" plan --prefix '' --cache "$temporary/empty-prefix-cache" >/dev/null 2>&1; then
  echo 'installer accepted an explicitly empty prefix' >&2
  exit 1
fi
if "$doctor" --prefix '' >/dev/null 2>&1; then
  echo 'doctor accepted an explicitly empty prefix' >&2
  exit 1
fi

if [ -n "$suite_prefix" ]; then
  case $suite_prefix in /*) ;; *) echo 'suite prefix must be absolute' >&2; exit 2 ;; esac
  verify_output=$("$installer" verify --prefix "$suite_prefix" --cache "$temporary/unused-cache")
  assert_contains "$verify_output" 'cage-backend=discovered-available-and-complete'
  assert_contains "$verify_output" 'cage-proof=not-performed'
  assert_contains "$verify_output" 'milestone=SUITE-INSTALLED'
  cage_plan_output=$("$installer" cage-plan --prefix "$suite_prefix" --cache "$temporary/unused-cache")
  assert_contains "$cage_plan_output" 'writes-on-check=isolated-temporary-fixtures-only'
  assert_contains "$cage_plan_output" 'network-on-check=loopback-only'
  mkdir -p "$temporary/platform-proof"
  cage_record_plan=$("$installer" cage-plan --prefix "$suite_prefix" \
    --evidence-out "$temporary/platform-proof/CAGE-CHECK.txt")
  physical_proof_parent=$(CDPATH= cd -P -- "$temporary/platform-proof" && pwd)
  assert_contains "$cage_record_plan" "proof-record=$physical_proof_parent/CAGE-CHECK.txt"
  assert_contains "$cage_record_plan" 'host-command='
  [ ! -e "$temporary/platform-proof/CAGE-CHECK.txt" ] || { echo 'cage-plan wrote its proof record' >&2; exit 1; }
  doctor_output=$("$doctor" --prefix "$suite_prefix")
  assert_contains "$doctor_output" 'cage-status=available'
  assert_contains "$doctor_output" 'cage-proof=not-performed'
  assert_contains "$doctor_output" 'milestone=SUITE-INSTALLED'

  ln -s "$model_probe" "$temporary/fake-bin/probe-model-access.sh"
  model_plan=$(PATH=$temporary/fake-bin:$PATH probe-model-access.sh plan --ask "$suite_prefix/bin/ask" \
    --model audit-validation/no-model-call --out "$temporary/model-readiness" --timeout-seconds 5)
  assert_contains "$model_plan" 'provider-calls-on-run=one'
  assert_contains "$model_plan" 'claim=current-process-provider-route-only'
  assert_contains "$model_plan" "resume-command='$model_probe'"
  [ ! -e "$temporary/model-readiness" ] || { echo 'model probe plan wrote output' >&2; exit 1; }
  if "$model_probe" plan --ask "$suite_prefix/bin/ask" --model invalid-model \
    --out "$temporary/model-invalid" >/dev/null 2>&1; then
    echo 'model probe accepted a model without provider/name form' >&2
    exit 1
  fi
  if "$model_probe" run --ask "$suite_prefix/bin/ask" \
    --model audit-validation/no-model-call --out "$temporary/model-unapproved" \
    --timeout-seconds 5 >/dev/null 2>&1; then
    echo 'model probe ran without approval' >&2
    exit 1
  fi
  [ ! -e "$temporary/model-unapproved" ] || { echo 'unapproved model probe wrote output' >&2; exit 1; }
  if model_failure=$("$model_probe" run --approve --ask "$suite_prefix/bin/ask" \
    --model audit-validation/no-model-call --out "$temporary/model-readiness" \
    --timeout-seconds 5); then
    echo 'no-model-call fixture unexpectedly earned MODEL-READY' >&2
    exit 1
  fi
  assert_contains "$model_failure" 'milestone=MODEL-ACCESS-REQUIRED'
  if find "$temporary/model-readiness" -type d -name sessions -print | grep . >/dev/null; then
    echo 'failed model probe retained a raw Ask session' >&2
    exit 1
  fi
  if find "$temporary/model-readiness" -type f -name response.json -print | grep . >/dev/null; then
    echo 'failed model probe retained a raw provider response' >&2
    exit 1
  fi

  bench=$suite_prefix/bin/bench
  mkdir -p "$temporary/agents"
  "$bench" home new "$temporary/agents/demo" 'fixture Agent home' >/dev/null
  "$bench" home check "$temporary/agents/demo" >/dev/null
  "$bench" home tick "$temporary/agents/demo" >/dev/null
  "$bench" home history "$temporary/agents/demo" check >/dev/null
  manifest_hash=$(sha256_file_for_test "$suite_prefix/lib/bench-suite/0.13.0/SHA256SUMS")
  proof_host=$(hostname)
  write_cage_fixture "$temporary/cage-check-mac.txt" Darwin/arm64 "$manifest_hash" "$proof_host"
  write_cage_fixture "$temporary/cage-check-linux.txt" Linux/x86_64 "$manifest_hash" "$proof_host"
  write_cage_fixture "$temporary/cage-check-wrong-manifest.txt" Darwin/arm64 deadbeef "$proof_host"
  write_cage_fixture "$temporary/cage-check-wrong-platform.txt" Linux/x86_64 "$manifest_hash" "$proof_host"
  write_cage_fixture "$temporary/agents/demo/CAGE-CHECK-inside-home.txt" Darwin/arm64 "$manifest_hash" "$proof_host"
  ln -s "$temporary/cage-check-mac.txt" "$temporary/cage-check-symlink.txt"

  mkdir -p "$temporary/incomplete-prefix/bin"
  ln -s "$suite_prefix/lib/bench-suite/0.13.0/bin/bench" "$temporary/incomplete-prefix/bin/bench"
  if "$doctor" --prefix "$temporary/incomplete-prefix" >/dev/null 2>&1; then
    echo 'doctor accepted a prefix missing seventeen public commands' >&2
    exit 1
  fi

  if "$operator" plan --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/agents/demo/controller-files" --name demo >/dev/null 2>&1; then
    echo 'operator accepted controller output inside the Agent home' >&2
    exit 1
  fi
  newline_out="$temporary/bad
output"
  if "$operator" plan --bench "$bench" --home "$temporary/agents/demo" \
    --out "$newline_out" --name demo >/dev/null 2>&1; then
    echo 'operator accepted a newline-containing output path' >&2
    exit 1
  fi

  plan_output=$(BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" plan --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops-plan" --name demo --model anthropic/fixture --minutes 60)
  assert_contains "$plan_output" 'runtime=bench-suite-agent'
  assert_contains "$plan_output" 'scheduler=launchd'
  assert_contains "$plan_output" 'schedule-checkpoint=none-distinct-runs'
  assert_contains "$plan_output" 'model-identifier=anthropic/fixture'
  assert_contains "$plan_output" 'cage-proof=pending'
  [ ! -e "$temporary/ops-plan" ] || { echo 'operator plan wrote output' >&2; exit 1; }

  if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops-no-cage-proof" --name demo >/dev/null 2>&1; then
    echo 'operator rendered without a Cage enforcement proof choice' >&2
    exit 1
  fi
  [ ! -e "$temporary/ops-no-cage-proof" ] || { echo 'failed Cage gate wrote operator output' >&2; exit 1; }

  if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops-no-model" --name demo --cage-proved \
    --cage-evidence "$temporary/cage-check-mac.txt" >/dev/null 2>&1; then
    echo 'operator rendered a fresh schedule without a model identifier' >&2
    exit 1
  fi
  [ ! -e "$temporary/ops-no-model" ] || { echo 'missing-model gate wrote operator output' >&2; exit 1; }
  if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops-invalid-model" --name demo --model invalid \
    --cage-proved --cage-evidence "$temporary/cage-check-mac.txt" >/dev/null 2>&1; then
    echo 'operator accepted a model without provider/name form' >&2
    exit 1
  fi
  [ ! -e "$temporary/ops-invalid-model" ] || { echo 'invalid-model gate wrote operator output' >&2; exit 1; }

  for rejected_record in \
    "$temporary/cage-check-wrong-manifest.txt" \
    "$temporary/cage-check-wrong-platform.txt" \
    "$temporary/cage-check-symlink.txt" \
    "$temporary/agents/demo/CAGE-CHECK-inside-home.txt"; do
    rejected_name=$(basename -- "$rejected_record")
    if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
      "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
      --out "$temporary/rejected-$rejected_name" --name demo --cage-proved \
      --cage-evidence "$rejected_record" >/dev/null 2>&1; then
      echo "operator accepted unsafe Cage evidence: $rejected_name" >&2
      exit 1
    fi
    [ ! -e "$temporary/rejected-$rejected_name" ] || { echo 'rejected Cage evidence wrote output' >&2; exit 1; }
  done

  cp "$temporary/cage-check-mac.txt" "$temporary/cage-check-oversized.txt"
  dd if=/dev/zero bs=65537 count=1 2>/dev/null >> "$temporary/cage-check-oversized.txt"
  if BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/rejected-oversized" --name demo --cage-proved \
    --cage-evidence "$temporary/cage-check-oversized.txt" >/dev/null 2>&1; then
    echo 'operator accepted oversized Cage evidence' >&2
    exit 1
  fi
  [ ! -e "$temporary/rejected-oversized" ] || { echo 'oversized Cage evidence wrote output' >&2; exit 1; }

  mac_render=$(BENCH_TEST_OS=Darwin BENCH_TEST_ARCH=arm64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops-mac" --name demo --model anthropic/fixture --minutes 60 \
    --cage-proved --cage-evidence "$temporary/cage-check-mac.txt")
  assert_contains "$mac_render" 'schedule-active=no'
  assert_contains "$mac_render" 'cage-proof=operator-attested-same-host-record'
  grep -F '<string>tick</string>' "$temporary/ops-mac/io.bench.agent.demo.plist" >/dev/null
  grep -F "$suite_prefix/lib/bench-suite/0.13.0/bin/bench</string>" "$temporary/ops-mac/io.bench.agent.demo.plist" >/dev/null
  grep -F '<string>home</string>' "$temporary/ops-mac/io.bench.agent.demo.plist" >/dev/null
  if grep -F '<string>-checkpoint</string>' "$temporary/ops-mac/io.bench.agent.demo.plist" >/dev/null; then
    echo 'default schedule unexpectedly carries a checkpoint' >&2
    exit 1
  fi
  grep -F -- '-m anthropic/fixture -home' "$temporary/ops-mac/LOCAL-RUNBOOK.md" >/dev/null
  [ "$(permission_mode "$temporary/ops-mac")" = 700 ] || { echo 'operator output is not mode 0700' >&2; exit 1; }
  [ "$(permission_mode "$temporary/ops-mac/logs")" = 700 ] || { echo 'operator logs are not mode 0700' >&2; exit 1; }
  [ "$(permission_mode "$temporary/ops-mac/CAGE-CHECK.txt")" = 600 ] || { echo 'Cage proof is not mode 0600' >&2; exit 1; }
  archived_cage_hash=$(sha256_file_for_test "$temporary/ops-mac/CAGE-CHECK.txt")
  printf '%s\n' 'changed after render' >> "$temporary/cage-check-mac.txt"
  [ "$(sha256_file_for_test "$temporary/ops-mac/CAGE-CHECK.txt")" = "$archived_cage_hash" ] || {
    echo 'archived Cage proof changed after its source record changed' >&2
    exit 1
  }
  if command -v plutil >/dev/null 2>&1; then
    plutil -lint "$temporary/ops-mac/io.bench.agent.demo.plist" >/dev/null
  fi

  BENCH_TEST_OS=Linux BENCH_TEST_ARCH=x86_64 PATH=$temporary/fake-bin:$PATH \
    "$operator" render --approve --bench "$bench" --home "$temporary/agents/demo" \
    --out "$temporary/ops linux" --name demo --model anthropic/fixture \
    --checkpoint scheduled-demo --minutes 60 --cage-proved \
    --cage-evidence "$temporary/cage-check-linux.txt" >/dev/null
  grep -F 'Type=oneshot' "$temporary/ops linux/bench-agent-demo.service" >/dev/null
  grep -F ' home -C ' "$temporary/ops linux/bench-agent-demo.service" >/dev/null
  grep -F ' tick -q -checkpoint ' "$temporary/ops linux/bench-agent-demo.service" >/dev/null
  grep -F 'Restart=no' "$temporary/ops linux/bench-agent-demo.service" >/dev/null
  grep -F 'StandardOutput="append:' "$temporary/ops linux/bench-agent-demo.service" >/dev/null
  grep -F 'Persistent=false' "$temporary/ops linux/bench-agent-demo.timer" >/dev/null
  if grep -ER 'API_KEY|sh -c|-no-cage|-net([[:space:]]|<|$)' "$temporary/ops-mac" "$temporary/ops linux" >/dev/null; then
    echo 'rendered operations contain a forbidden credential or authority pattern' >&2
    exit 1
  fi

  wake=$temporary/agents/demo/bin/wake
  printf '%s\n' '# Fixture heartbeat' 'Check one bounded fixture.' > "$temporary/agents/demo/HEARTBEAT.md"
  sed 's/^exit 0$/exit 1/' "$wake" > "$wake.next"
  mv "$wake.next" "$wake"
  chmod +x "$wake"
  wake_stdout=$temporary/wake.stdout
  wake_stderr=$temporary/wake.stderr
  if env -i PATH=/usr/bin:/bin "$suite_prefix/lib/bench-suite/0.13.0/bin/bench" \
    home -C "$temporary/agents" tick -q -m audit-validation/no-model-call demo \
    >"$wake_stdout" 2>"$wake_stderr"; then
    wake_status=0
  else
    wake_status=$?
  fi
  wake_output=$(sed -n '1,160p' "$wake_stdout"; sed -n '1,160p' "$wake_stderr")
  assert_contains "$wake_output" 'wake check found work'
  assert_not_contains "$wake_output" 'ply is required'
  assert_not_contains "$wake_output" 'ask is required'
  assert_not_contains "$wake_output" 'brief is required'
  assert_not_contains "$wake_output" 'cage is required'
  [ "$wake_status" -ne 127 ] || { echo 'sanitized schedule smoke could not execute Bench' >&2; exit 1; }
fi

printf 'bench-system-builder tests: ok\n'
