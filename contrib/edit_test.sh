#!/bin/sh
# edit's tests. Not part of ply's `go test`, because contrib is not part of
# ply -- but a program that rewrites files in place earns a test suite
# whoever it belongs to, and this one is a shell script for the same reason
# the thing it tests is a program.
#
#     sh contrib/edit_test.sh
set -u

EDIT="$(cd "$(dirname "$0")" && pwd)/edit"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
pass=0
fail=0

ok() {
	pass=$((pass + 1))
}

no() {
	fail=$((fail + 1))
	printf 'FAIL: %s\n' "$1" >&2
	[ $# -gt 1 ] && printf '  %s\n' "$2" >&2
}

# want <name> <expected-exit> <actual-exit>
want() {
	if [ "$2" = "$3" ]; then ok; else no "$1" "exit $3, want $2"; fi
}

# holds <name> <file> <text>
holds() {
	if grep -qF -- "$3" "$2"; then ok; else no "$1" "$2 does not contain: $3"; fi
}

# lacks <name> <file> <text>
lacks() {
	if grep -qF -- "$3" "$2"; then no "$1" "$2 still contains: $3"; else ok; fi
}

# says <name> <text> <output>
says() {
	case "$3" in
	*"$2"*) ok ;;
	*) no "$1" "message was: $3" ;;
	esac
}

ring() {
	printf 'package ring\n\nfunc (r *Ring) Push(v int) {\n\tr.buf[r.head] = v\n\tr.head = r.head + 1 // BUG\n\tr.n++\n}\n' >"$TMP/ring.go"
}

# --- argv form -------------------------------------------------------------

ring
$EDIT "$TMP/ring.go" 'r.head + 1 // BUG' '(r.head + 1) % len(r.buf)' >/dev/null
want "argv edit applies" 0 $?
holds "argv edit applies" "$TMP/ring.go" '% len(r.buf)'

# --- the three diagnoses ---------------------------------------------------

out=$($EDIT "$TMP/ring.go" 'r.head + 1 // BUG' '(r.head + 1) % len(r.buf)' 2>&1)
want "already applied" 1 $?
says "already applied" "already been applied" "$out"

out=$($EDIT "$TMP/ring.go" '    r.n++' '    r.n += 1' 2>&1)
want "tabs against spaces" 1 $?
says "tabs against spaces" "tabs:" "$out"

printf 'a\nb\nc\nb\n' >"$TMP/dup.txt"
out=$($EDIT "$TMP/dup.txt" 'b' 'B' 2>&1)
want "ambiguous refuses" 1 $?
says "ambiguous refuses" "matches 2 places, at lines 2, 4" "$out"
lacks "ambiguous refuses" "$TMP/dup.txt" 'B'

printf 'hello world\n' >"$TMP/gone.txt"
out=$($EDIT "$TMP/gone.txt" 'nowhere near this' 'x' 2>&1)
want "absent text refuses" 1 $?
says "absent text refuses" "does not appear anywhere" "$out"

# --- SEARCH/REPLACE --------------------------------------------------------

ring
$EDIT "$TMP/ring.go" >/dev/null <<'EOF'
<<<<<<< SEARCH
package ring
=======
package ring // fixed
>>>>>>> REPLACE
<<<<<<< SEARCH
	r.head = r.head + 1 // BUG
=======
	r.head = (r.head + 1) % len(r.buf)
>>>>>>> REPLACE
EOF
want "two blocks apply" 0 $?
holds "two blocks apply" "$TMP/ring.go" 'package ring // fixed'
holds "two blocks apply" "$TMP/ring.go" '% len(r.buf)'

printf 'alpha beta gamma\n' >"$TMP/ov.txt"
out=$($EDIT "$TMP/ov.txt" 2>&1 <<'EOF'
<<<<<<< SEARCH
alpha beta
=======
X
>>>>>>> REPLACE
<<<<<<< SEARCH
beta gamma
=======
Y
>>>>>>> REPLACE
EOF
)
want "overlap refuses" 1 $?
says "overlap refuses" "overlaps" "$out"
holds "overlap refuses" "$TMP/ov.txt" 'alpha beta gamma'

# Escalation: the text being edited holds a line of seven equals signs.
printf 'Title\n=======\nbody\n' >"$TMP/md.txt"
$EDIT "$TMP/md.txt" >/dev/null <<'EOF'
<<<<<<<< SEARCH
Title
=======
body
========
New
>>>>>>>>
EOF
want "eight markers escape seven" 0 $?
holds "eight markers escape seven" "$TMP/md.txt" 'New'

out=$($EDIT "$TMP/md.txt" 2>&1 <<'EOF'
<<<<<<< SEARCH
New
=======
Newer
EOF
)
want "unterminated block is an error" 2 $?
says "unterminated block is an error" "open the block with 8" "$out"

# --- the patch dialect -----------------------------------------------------

ring
$EDIT >/dev/null <<EOF
*** Begin Patch
*** Update File: $TMP/ring.go
@@
 func (r *Ring) Push(v int) {
 	r.buf[r.head] = v
-	r.head = r.head + 1 // BUG
+	r.head = (r.head + 1) % len(r.buf)
 	r.n++
 }
*** End Patch
EOF
want "patch hunk applies" 0 $?
holds "patch hunk applies" "$TMP/ring.go" '% len(r.buf)'

ring
out=$($EDIT 2>&1 <<EOF
*** Begin Patch
*** Update File: $TMP/ring.go
@@
+// orphan
*** End Patch
EOF
)
want "addition with no context is an error" 2 $?
says "addition with no context is an error" "nothing to find" "$out"

out=$($EDIT 2>&1 <<EOF
*** Begin Patch
*** Add File: $TMP/brand-new.go
+package x
*** End Patch
EOF
)
want "Add File is refused" 2 $?
says "Add File is refused" "Creating a file is" "$out"

# --- atomicity across files ------------------------------------------------

printf 'one\ntwo\n' >"$TMP/f1.txt"
printf 'three\nfour\n' >"$TMP/f2.txt"
out=$($EDIT 2>&1 <<EOF
--- $TMP/f1.txt
<<<<<<< SEARCH
one
=======
ONE
>>>>>>> REPLACE
--- $TMP/f2.txt
<<<<<<< SEARCH
not in this file
=======
whatever
>>>>>>> REPLACE
EOF
)
want "one bad edit rolls back every file" 1 $?
holds "one bad edit rolls back every file" "$TMP/f1.txt" 'one'
lacks "one bad edit rolls back every file" "$TMP/f1.txt" 'ONE'

# --- what it will not do ---------------------------------------------------

out=$($EDIT "$TMP/nosuch.txt" a b 2>&1)
want "missing file is broken, not refused" 2 $?
says "missing file is broken, not refused" '`>` makes them' "$out"

out=$($EDIT "$TMP/f1.txt" 'one' 'one' 2>&1)
want "a no-op edit is refused" 1 $?

# --- -n changes nothing ----------------------------------------------------

out=$($EDIT -n "$TMP/f1.txt" 'one' 'ONE' 2>&1)
want "-n exits 0" 0 $?
says "-n prints a diff" "+ONE" "$out"
holds "-n writes nothing" "$TMP/f1.txt" 'one'
lacks "-n writes nothing" "$TMP/f1.txt" 'ONE'

# --- bytes are preserved ---------------------------------------------------

printf 'a\r\nb\r\n' >"$TMP/crlf.txt"
chmod 755 "$TMP/crlf.txt"
$EDIT "$TMP/crlf.txt" 'b' 'B' >/dev/null
want "CRLF edit applies" 0 $?
if od -c "$TMP/crlf.txt" | grep -q '\\r'; then ok; else no "CRLF survives" "line endings were rewritten"; fi
case "$(ls -l "$TMP/crlf.txt")" in
-rwxr-xr-x*) ok ;;
*) no "mode survives" "$(ls -l "$TMP/crlf.txt")" ;;
esac

# ---------------------------------------------------------------------------

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
