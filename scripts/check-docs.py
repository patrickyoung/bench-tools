#!/usr/bin/env python3
"""Check local destinations and heading anchors in the toolkit's entry guides."""
import re
from pathlib import Path
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
PUBLIC = tuple("https://github.com/patrickyoung/bench-tools/" + kind + "/main/"
               for kind in ("blob", "tree"))


def prose(text):
    return re.sub(r"^(`{3,}|~{3,})[^\n]*\n.*?^\1\s*$", "", text,
                  flags=re.MULTILINE | re.DOTALL)


def anchors(path):
    found, counts = set(), {}
    for heading in re.findall(r"^#{1,6}\s+(.+?)\s*#*\s*$", prose(path.read_text()), re.MULTILINE):
        heading = re.sub(r"\[([^]]+)\]\([^)]*\)", r"\1", heading).lower()
        slug = re.sub(r"[^\w\- ]", "", heading).replace(" ", "-")
        count = counts.get(slug, 0)
        found.add(slug + ("-" + str(count) if count else ""))
        counts[slug] = count + 1
    return found


def main():
    paths = [ROOT / "README.md", ROOT / "scripts/README.md"]
    paths += sorted((ROOT / "docs").glob("*.md"))
    paths += sorted((ROOT / "examples").rglob("*.md"))
    paths += sorted((ROOT / "tools").glob("*/README.md"))
    errors, checked = [], 0
    for path in paths:
        text = re.sub(r"(`+).*?\1", "", prose(path.read_text()), flags=re.DOTALL)
        for match in re.finditer(r"\[[^\]\n]*\]\((?:<([^>]+)>|([^\s)]+))(?:\s+\"[^\"]*\")?\)", text):
            link = match.group(1) or match.group(2)
            public = next((prefix for prefix in PUBLIC if link.startswith(prefix)), None)
            parts = urlsplit(link[len(public):] if public else link)
            if parts.scheme or parts.netloc:
                continue
            target = ((ROOT if public else path.parent) / unquote(parts.path)).resolve() if parts.path else path
            label = str(path.relative_to(ROOT)) + ": " + link
            if ROOT != target and ROOT not in target.parents:
                errors.append(label + " (outside repository)")
            elif not target.exists():
                errors.append(label + " (missing destination)")
            elif parts.fragment:
                document = target / "README.md" if target.is_dir() else target
                if document.suffix == ".md" and document.exists() and unquote(parts.fragment) not in anchors(document):
                    errors.append(label + " (missing heading)")
            checked += 1
    if errors:
        print("Documentation links failed:\n" + "\n".join(errors), file=sys.stderr)
        return 1
    print(f"Documentation links passed: {checked} links across {len(paths)} entry guides.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
