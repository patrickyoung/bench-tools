#!/bin/sh
set -eu

prefix=${HOME}/.local

usage() {
	cat <<'EOF'
usage: ./install.sh [-prefix DIR]

Build oauth from this checkout and install it under DIR/bin.
The default prefix is $HOME/.local.
EOF
}

while test "$#" -gt 0; do
	case $1 in
		-prefix)
			shift
			test "$#" -gt 0 || { usage >&2; exit 2; }
			prefix=$1
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			usage >&2
			exit 2
			;;
	esac
	shift
done

command -v go >/dev/null 2>&1 || {
	echo 'install.sh: Go 1.26 or newer is required' >&2
	exit 2
}

here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/oauth-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

(cd "$here" && go test ./...)
(cd "$here" && go build -trimpath -o "$tmp/oauth" ./cmd/oauth)

mkdir -p "$prefix/bin"
install -m 0755 "$tmp/oauth" "$prefix/bin/oauth"

echo "installed oauth in $prefix/bin" >&2
