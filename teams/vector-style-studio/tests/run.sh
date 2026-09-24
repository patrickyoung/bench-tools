#!/bin/sh
# Synthetic wiring only. No real worker, model or image backend is invoked.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/studio-contract.XXXXXXXX")
tmp=$(CDPATH= cd -- "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' 0
trap 'exit 130' INT
mkdir "$tmp/literal paths with spaces"
t="$tmp/literal paths with spaces"
# Strip assembled-only content from a disposable copy, never the source.
cp -R "$root/expert" "$t/template"
rm -rf "$t/template/agents" "$t/template/bin/workers"
cp -R "$t/template" "$t/expert"
mkdir -p "$t/expert/agents/vector/bin" "$t/expert/agents/bitmap/bin" "$t/commands"
cp "$root/tests/fake-agent" "$t/commands/agent"
cp "$root/tests/fake-vector-check" "$t/expert/agents/vector/bin/check"
cp "$root/tests/fake-stylize" "$t/expert/agents/bitmap/bin/stylize"
cp "$root/tests/fake-bitmap-check" "$t/expert/agents/bitmap/bin/check-output"
cp "$root/tests/fake-inkscape" "$t/commands/inkscape"
printf '#!/bin/sh\nexit 88\n' > "$t/commands/unused-capability"
chmod +x "$t/commands/"* "$t/expert/agents/"*/bin/*
export PATH="$t/commands:$PATH"
export INKSCAPE="$t/commands/inkscape"
export BITMAP_HELPER="$t/commands/unused-capability" BITMAP_GENERATOR="$t/commands/unused-capability"
export VECTOR_STYLE_MODEL=fixture-vector BITMAP_AUTHOR_MODEL=fixture-bitmap
export AGENT_ASK=fixture-inherited-ask PRESENT_ASK=fixture-inherited-present
export FIX_EVENTS="$t/events"
: > "$FIX_EVENTS"
entry=$t/expert/bin/studio
check=$t/expert/bin/check
cp "$root/tests/job.json" "$t/job.json"
# Optional source admitted explicitly; no implicit old output.
printf '<svg xmlns="http://www.w3.org/2000/svg"/>\n' > "$t/selected source.svg"
jq --arg source "$t/selected source.svg" '.source_svg=$source' "$t/job.json" > "$t/source job.json"
n=0
expect() {
    want=$1; shift
    n=$((n+1))
    if "$@" > "$t/stdout" 2> "$t/stderr"; then got=0; else got=$?; fi
    if [ "$got" != "$want" ]; then
        echo "FAIL $n expected=$want actual=$got argv=$*" >&2
        cat "$t/stderr" >&2
        exit 1
    fi
}
expect 0 "$entry" "$t/source job.json" "$t/good run"
jq -e '.variants|length==2' "$t/stdout" >/dev/null
cmp "$t/stdout" "$t/good run/manifest.json"
printf 'vector\nvector-check\nbitmap:first\nbitmap:second\nvector-check\n' > "$t/expected-events"
cmp "$FIX_EVENTS" "$t/expected-events"
cmp "$t/good run/result/original.png" "$t/good run/vector/work/output/preview.png"
cmp "$t/good run/styles/first/work/inputs/source.png" "$t/good run/styles/second/work/inputs/source.png"
# Read-only relative to run files, not merely a zero exit.
find "$t/good run" -type f -exec shasum -a 256 {} \; | sort > "$t/before"
expect 0 "$check" "$t/good run"
find "$t/good run" -type f -exec shasum -a 256 {} \; | sort > "$t/after"
cmp "$t/before" "$t/after"
expect 1 "$entry" "$t/source job.json" "$t/good run"
: > "$FIX_EVENTS"
expect 75 env FIX_VECTOR_STATUS=75 "$entry" "$t/job.json" "$t/vector fail"
[ "$(cat "$FIX_EVENTS")" = vector ]
[ "$(cat "$t/vector fail/control/vector.exit")" = 75 ]
[ ! -f "$t/vector fail/manifest.json" ]
: > "$FIX_EVENTS"
expect 42 env FIX_BITMAP_FAIL=first "$entry" "$t/job.json" "$t/bitmap fail"
[ ! -d "$t/bitmap fail/styles/second" ]
[ "$(cat "$t/bitmap fail/styles/first/stylize.exit")" = 42 ]
[ -f "$t/bitmap fail/result/original.png" ]
expect 42 env FIX_BITMAP_FAIL=second "$entry" "$t/job.json" "$t/second fail"
[ -f "$t/second fail/result/first.png" ]
for alteration in \
 '.styles[1].id=.styles[0].id' \
 '.styles[0].id="../escape"' \
 '.styles[0].id="original"' \
 '.required_text += .required_text' \
 '.required_text[0].id="bad\"xpath"' \
 '.vector_request.width=true' \
 '.styles=[]' \
 '.styles[0].extra="unknown"' \
 '.preserve_regions=[{x:0,y:0,width:128,height:256,label:"a"},{x:128,y:0,width:128,height:256,label:"b"}]' \
 '.preserve_regions[0].x=-1'
do
    jq "$alteration" "$t/job.json" > "$t/bad.json"
    : > "$FIX_EVENTS"
    expect 1 "$entry" "$t/bad.json" "$t/invalid-$n"
    [ ! -s "$FIX_EVENTS" ]
done
printf '{"version":1,"version":1}\n' > "$t/bad.json"
expect 1 "$entry" "$t/bad.json" "$t/duplicate key"
ln -s "$t/job.json" "$t/job-link.json"
expect 1 "$entry" "$t/job-link.json" "$t/link run"
ln -s "$t" "$tmp/link"
expect 1 "$entry" "$t/job.json" "$tmp/link/escape"
for mode in missing mismatch; do
    : > "$FIX_EVENTS"
    expect 1 env FIX_TEXT="$mode" "$entry" "$t/job.json" "$t/text-$mode"
    ! grep 'bitmap:' "$FIX_EVENTS"
done
for mode in missing invalid nan duplicate uncovered; do
    : > "$FIX_EVENTS"
    expect 1 env FIX_BOUNDS="$mode" "$entry" "$t/job.json" "$t/bounds-$mode"
    ! grep 'bitmap:' "$FIX_EVENTS"
done
expect 1 env FIX_MUTATE=request "$entry" "$t/job.json" "$t/mutated request"
expect 1 env FIX_MUTATE=source "$entry" "$t/source job.json" "$t/mutated source"
# Stale/single-file tampering must fail, without refreshing trusted receipts.
for rel in result/original.png result/first.png styles/first/control/final.png \
 styles/second/work/inputs/source.png styles/first/work/brief.json \
 vector/work/output/render.json control/source.svg control/job.json \
 manifest.json
do
    file="$t/good run/$rel"
    cp "$file" "$t/saved"
    chmod u+w "$file"
    printf 'tamper\n' >> "$file"
    expect 1 "$check" "$t/good run"
    cp "$t/saved" "$file"
done
# Same-source check rejects a chain even if the fake member snapshots agree.
file="$t/good run/styles/second/work/inputs/source.png"
cp "$file" "$t/saved"
cp "$t/good run/result/first.png" "$file"
cp "$file" "$t/good run/styles/second/control/source.png"
expect 1 "$check" "$t/good run"
cp "$t/saved" "$file"
cp "$t/saved" "$t/good run/styles/second/control/source.png"
# Explicit art-only case needs neither lettering nor protected rectangles.
jq '.required_text=[] | .preserve_regions=[] | .styles=[.styles[0]]' "$t/job.json" > "$t/art.json"
expect 0 "$entry" "$t/art.json" "$t/art-only"
expect 0 "$check" "$t/good run"
expect 1 "$check" "$t/missing run"
# Source template must not invent child capabilities.
expect 2 "$t/template/bin/studio" "$t/job.json" "$t/unassembled"
grep 'missing assembled member' "$t/stderr" >/dev/null
echo "PASS $n offline process/check contracts (fixture media only)"
