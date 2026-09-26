#!/bin/sh
set -eu

prefix=${HOME}/.local
while test "$#" -gt 0; do
    case $1 in
        -prefix)
            shift
            test "$#" -gt 0 || { echo 'install.sh: -prefix needs a directory' >&2; exit 2; }
            prefix=$1
            ;;
        -h|--help)
            echo 'usage: ./install.sh [-prefix DIR]'
            exit 0
            ;;
        *) echo 'usage: ./install.sh [-prefix DIR]' >&2; exit 2 ;;
    esac
    shift
done

here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/moniker-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
(cd "$here" && GOWORK=off go test ./...)
(cd "$here" && GOWORK=off go build -trimpath -o "$tmp/moniker" .)
mkdir -p "$prefix/bin" "$prefix/share/moniker/mcp" "$prefix/share/man/man1"
install -m 0755 "$tmp/moniker" "$prefix/bin/moniker"
install -m 0644 "$here/mcp/manifest.json" "$prefix/share/moniker/mcp/manifest.json"
install -m 0644 "$here/moniker.1" "$prefix/share/man/man1/moniker.1"
echo "installed moniker in $prefix/bin" >&2
