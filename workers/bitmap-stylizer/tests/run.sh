#!/bin/sh
# Offline public-process fixtures, built and run outside reusable source.
set -eu
test_source=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
expert=${1:-"$test_source/../expert"}
expert=$(CDPATH= cd -- "$expert" && pwd -P)
for command in go ask record; do
    command -v "$command" >/dev/null || { echo "required command missing: $command" >&2; exit 2; }
done
test_root=$(mktemp -d "${TMPDIR:-/tmp}/bitmap-contracts.XXXXXXXX")
test_root=$(CDPATH= cd -- "$test_root" && pwd -P)
trap 'rm -rf "$test_root"' 0
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$test_root/install/bin" "$test_root/install/fixtures" "$test_root/tests/fake" "$test_root/tmp"
cp -R "$expert" "$test_root/expert"
cp "$test_source/go.mod" "$test_source/bitmap_test.go" "$test_root/tests/"
cp "$test_source/fake/main.go" "$test_root/tests/fake/"
export TMPDIR="$test_root/tmp"
export GOCACHE="$test_root/go-cache"
export BITMAP_TEST_ROOT="$test_root"
(cd "$test_root/expert/bitmap" && go build -o "$test_root/install/bin/bitmap" . && go vet ./...)
(cd "$test_root/tests" && go build -o "$test_root/install/fixtures/generator" ./fake)
cp "$test_root/install/fixtures/generator" "$test_root/install/fixtures/agent"
(cd "$test_root/tests" && go test -count=1 -v ./...)
