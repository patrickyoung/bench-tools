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

# Targeted extension uses the SAME entry, query boundary and fake member contract.
cp "$root/tests/target-job.json" "$t/target.json"
expect 0 "$entry" "$t/target.json" "$t/target run"
expect 0 "$check" "$t/target run"
jq -e --rawfile goal "$t/target run/control/vector-goal.txt" '
 all(.styles[].target_ids[]; tojson as $id | $goal | contains($id))' "$t/target.json" >/dev/null
jq -e '.variants[0].target_ids==["caption"] and
 .variants[0].text_ids_allowed_to_change==["caption"] and
 .variants[1].target_ids==["sun","cloud","rain"] and
 .variants[1].text_ids_allowed_to_change==[]' "$t/target run/manifest.json" >/dev/null
jq -e '.targets[0].measured_box=={x:10,y:10,width:80,height:20} and
 .selected_wording==[{id:"caption",text:"TEST"}]' "$t/target run/control/targets/headline.json" >/dev/null
jq -e '.targets|map(.edit_box)==[
 {x:100,y:50,width:16,height:16},{x:126,y:50,width:16,height:16},{x:152,y:50,width:16,height:16}]'  "$t/target run/control/targets/icons.json" >/dev/null
jq -e '.style | contains("\"text\":\"TEST\"")' "$t/target run/styles/headline/work/brief.json" >/dev/null
cmp "$t/target run/styles/headline/work/inputs/source.png" "$t/target run/styles/icons/work/inputs/source.png"
find "$t/target run" -type f -exec shasum -a 256 {} \; | sort > "$t/before"
expect 0 "$check" "$t/target run"
find "$t/target run" -type f -exec shasum -a 256 {} \; | sort > "$t/after"
cmp "$t/before" "$t/after"
for alteration in \
 '.styles[0].target_ids=null' \
 '.styles[0].target_ids=[]' \
 '.styles[0].target_ids="caption"' \
 '.styles[0].target_ids=["caption","caption"]' \
 '.styles[0].target_ids=["bad/id"]' \
 '.styles[0].target_ids=[1]' \
 '.styles[0].target_ids=["bad\"xpath"]' \
 '.styles[0].target_ids=[range(0;33)|"id"+tostring]' \
 '.styles[0].target_ids=["a"*65]' \
 '.styles[0].target_padding=null' \
 '.styles[0].target_padding=-1' \
 '.styles[0].target_padding=33' \
 '.styles[0].target_padding=0.5' \
 '.styles[0].target_padding=true' \
 '.styles[0] |= (del(.target_ids) | .target_padding=0)'
do
    jq "$alteration" "$t/target.json" > "$t/bad-target.json"
    : > "$FIX_EVENTS"
    expect 1 "$entry" "$t/bad-target.json" "$t/invalid-target-$n"
    [ ! -s "$FIX_EVENTS" ]
done
# Bad SECOND style must stop even the valid headline from generating.
for mode in missing empty short duplicate bad-duplicate zero negative nan infinite bad outside negative-origin intersect; do
    : > "$FIX_EVENTS"
    expect 1 env FIX_TARGET_BOUNDS="$mode" "$entry" "$t/target.json" "$t/target-bounds-$mode"
    ! grep 'bitmap:' "$FIX_EVENTS"
done
for mode in missing duplicate non-svg; do
    : > "$FIX_EVENTS"
    expect 1 env FIX_TARGET_XML="$mode" "$entry" "$t/target.json" "$t/target-xml-$mode"
    ! grep 'bitmap:' "$FIX_EVENTS"
done
for mode in missing duplicate; do
    : > "$FIX_EVENTS"
    expect 1 env FIX_TARGET_FINAL="$mode" "$entry" "$t/target.json" "$t/target-final-$mode"
    ! grep 'bitmap:' "$FIX_EVENTS"
done
# Fresh native queries, not saved coordinates, govern independent reacceptance.
expect 1 env FIX_TARGET_BOUNDS=drift "$check" "$t/target run"
expect 1 env FIX_TARGET_BOUNDS=missing "$check" "$t/target run"
for rel in control/targets/headline.json styles/headline/work/brief.json manifest.json; do
    file="$t/target run/$rel"
    cp "$file" "$t/saved"
    chmod u+w "$file"
    # Valid JSON tampering, not merely malformed syntax.
    jq 'if has("targets") then .targets[0].edit_box.width+=1
        elif has("style") then .style+=" tamper" else .variants[0].target_ids=["sun"] end'       "$file" > "$t/changed"
    cp "$t/changed" "$file"
    expect 1 "$check" "$t/target run"
    cp "$t/saved" "$file"
done
# Overlap is safe when unrelated text does not intersect the union.
jq '.styles=[{id:"overlap",style:"Overlapping targets",target_ids:["sun","overlap"]}] |
 .required_text=[.required_text[0]]' "$t/target.json" > "$t/overlap.json"
expect 0 "$entry" "$t/overlap.json" "$t/overlap run"
# Target-only work still requires a query. Padding is clipped and fractional
# native boxes are rounded outward to integer pixel edges.
jq '.required_text=[] | .preserve_regions=[] |
 .styles=[{id:"edge",style:"Edge target",target_ids:["edge"],target_padding:3}]'  "$t/target.json" > "$t/edge.json"
expect 0 "$entry" "$t/edge.json" "$t/edge run"
jq -e '.targets[0].edit_box=={x:0,y:0,width:8,height:9}' "$t/edge run/control/targets/edge.json" >/dev/null
jq '.styles[0].target_ids=["sun"]' "$t/edge.json" > "$t/no-text.json"
: > "$FIX_EVENTS"
expect 1 env FIX_TARGET_BOUNDS=missing "$entry" "$t/no-text.json" "$t/no-text run"
! grep 'bitmap:' "$FIX_EVENTS"
jq '.styles[1].target_padding=5' "$t/target.json" > "$t/padded-text.json"
: > "$FIX_EVENTS"
expect 1 "$entry" "$t/padded-text.json" "$t/padded-text run"
! grep 'bitmap:' "$FIX_EVENTS"
# Explicitly targeted text must STILL satisfy baseline global protection.
jq '.preserve_regions=[]' "$t/target.json" > "$t/unprotected.json"
expect 1 "$entry" "$t/unprotected.json" "$t/unprotected run"
# Full union rejection (two boxes rather than a trivial single full-frame box).
jq '.required_text=[] | .preserve_regions=[] |
 .styles=[{id:"full",style:"No protection",target_ids:["left","right"]}]'  "$t/target.json" > "$t/full.json"
: > "$FIX_EVENTS"
expect 1 "$entry" "$t/full.json" "$t/full run"
! grep 'bitmap:' "$FIX_EVENTS"
grep 'target union covers entire frame' "$t/stderr" >/dev/null
# Exact caller text is never shortened to accommodate a suffix.
jq '.styles[0].style=("x"*8000)' "$t/target.json" > "$t/long-style.json"
: > "$FIX_EVENTS"
expect 1 "$entry" "$t/long-style.json" "$t/long-style run"
! grep 'bitmap:' "$FIX_EVENTS"
grep '8000-byte style limit' "$t/stderr" >/dev/null
# Mixed global/target styles retain the old brief for the global variant.
jq '.styles += [{id:"global",style:"Unchanged literal global \"style\""}]' "$t/target.json" > "$t/mixed.json"
expect 0 "$entry" "$t/mixed.json" "$t/mixed run"
jq -e '.variants[2] | has("target_ids") | not' "$t/mixed run/manifest.json" >/dev/null
expect 0 "$check" "$t/target run"
# A lattice of 15 vertical + 15 horizontal targets leaves exactly 256
# disconnected protected regions. Sixteen each leaves 289: never truncate.
jq '.required_text=[] | .preserve_regions=[] |
 .styles=[{id:"grid",style:"Grid treatment",
 target_ids:([range(0;15),range(16;31)] | map("grid"+tostring))}]' \
 "$t/target.json" > "$t/grid.json"
expect 0 env FIX_GRID=1 "$entry" "$t/grid.json" "$t/grid run"
jq -e '.preserve_regions|length==256' "$t/grid run/control/targets/grid.json" >/dev/null
jq '.styles[0].target_ids=[range(0;32)|"grid"+tostring]' "$t/grid.json" > "$t/over-limit.json"
: > "$FIX_EVENTS"
expect 1 env FIX_GRID=1 "$entry" "$t/over-limit.json" "$t/over-limit run"
! grep 'bitmap:' "$FIX_EVENTS"
grep 'exceeds 256 preserve regions' "$t/stderr" >/dev/null
# JSON-escaped control characters fit the literal 8000-byte style budget,
# but the derived regions plus encoding exceed the member 64 KiB brief limit.
jq '.styles[0].style=("x"+("\u0001"*7999))' "$t/grid.json" > "$t/byte-limit.json"
: > "$FIX_EVENTS"
expect 1 env FIX_GRID=1 "$entry" "$t/byte-limit.json" "$t/byte-limit run"
! grep 'bitmap:' "$FIX_EVENTS"
grep 'oversized file' "$t/stderr" >/dev/null
# A failed targeted member outcome is not rescued by target admission.
expect 42 env FIX_BITMAP_FAIL=icons "$entry" "$t/target.json" "$t/target bitmap fail"
[ ! -f "$t/target bitmap fail/manifest.json" ]
# Boundary-touching text is safe (half-open boxes); padding 4 ends at y=70.
jq '.styles=[.styles[1]] | .styles[0].target_ids=["sun"] |
 .styles[0].target_padding=4' "$t/target.json" > "$t/touch.json"
expect 0 "$entry" "$t/touch.json" "$t/touch run"
jq -ne -L "$root/expert/lib" -f "$root/tests/complement.jq"
echo "PASS $n total offline process/check contracts (42 legacy plus targeted extension; fixture media only)"
