#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
plugin="$root/plugins/bench-system-builder"
downloads="$root/docs/downloads"
version=0.2.0

command -v zip >/dev/null 2>&1 || {
  printf '%s\n' 'package-skill-downloads: zip is required' >&2
  exit 2
}
command -v shasum >/dev/null 2>&1 || {
  printf '%s\n' 'package-skill-downloads: shasum is required' >&2
  exit 2
}

stage=$(mktemp -d "${TMPDIR:-/tmp}/bench-skills.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM

mkdir -p "$downloads"

package_dir() {
  source_dir=$1
  archive_name=$2
  base_name=$(basename "$source_dir")
  package_root="$stage/$archive_name"
  archive="$downloads/$archive_name-$version.zip"

  rm -rf "$package_root"
  mkdir -p "$package_root"
  cp -R "$source_dir" "$package_root/$base_name"
  find "$package_root" -exec touch -t 202609010000 {} +

  rm -f "$archive"
  (
    cd "$package_root"
    LC_ALL=C find "$base_name" -print | LC_ALL=C sort | zip -X -q "$archive" -@
  )
}

package_dir "$plugin" bench-system-builder
package_dir "$plugin/skills/building-with-bench" building-with-bench
package_dir "$plugin/skills/operating-bench-agents" operating-bench-agents
package_dir "$plugin/skills/stewarding-bench-platform" stewarding-bench-platform

checksum_file="$downloads/SHA256SUMS"
rm -f "$checksum_file"
for archive in \
  "$downloads/bench-system-builder-$version.zip" \
  "$downloads/building-with-bench-$version.zip" \
  "$downloads/operating-bench-agents-$version.zip" \
  "$downloads/stewarding-bench-platform-$version.zip"
do
  archive_base=$(basename "$archive")
  archive_hash=$(shasum -a 256 "$archive" | awk '{print $1}')
  printf '%s  %s\n' "$archive_hash" "$archive_base" >> "$checksum_file"
  printf '%s  %s\n' "$archive_hash" "$archive_base" > "$archive.sha256"
done

printf 'packaged skill downloads in %s\n' "$downloads"
