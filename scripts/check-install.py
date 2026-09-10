#!/usr/bin/env python3
"""Exercise actual built packages in a disposable, relocated user prefix."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build-dir", type=Path, default=ROOT / ".build")
    args = parser.parse_args()
    components = json.loads((ROOT / "components.json").read_text())["components"]
    with tempfile.TemporaryDirectory(prefix="bench-install-check-") as temporary:
        scratch = Path(temporary).resolve()
        prefix, home, work = scratch / "original", scratch / "home", scratch / "work"
        home.mkdir(); work.mkdir()
        subprocess.run([sys.executable, ROOT / "scripts/install", "--from-build", args.build_dir.resolve(),
                        "--prefix", prefix], check=True)
        relocated = scratch / "relocated prefix with spaces"
        prefix.rename(relocated)
        prefix = relocated
        env = {"HOME": str(home), "PATH": str(prefix / "bin") + ":/usr/bin:/bin:/usr/sbin:/sbin",
               "TMPDIR": str(scratch), "LANG": "C", "LC_ALL": "C",
               "BRIEF_PATH": str(prefix / "lib/bench-tools/draft/skills"),
               "VOUCH": str(scratch / "absent-vouch"), "WEB": str(scratch / "absent-web")}

        def run(argv):
            result = subprocess.run(list(map(str, argv)), cwd=work, env=env, capture_output=True, text=True)
            if result.returncode:
                raise RuntimeError(f"{argv}: exit {result.returncode}\n{result.stdout}\n{result.stderr}")
            return result

        for component in components:
            for command in component["commands"]:
                for option in ("version", "help"):
                    result = run([prefix / "bin" / command["name"], option])
                    if not (result.stdout or result.stderr).strip():
                        raise RuntimeError(f"empty {option} output from {command['name']}")
        if (prefix / "bin/agent-action-shell").exists():
            raise RuntimeError("private Agent helper was exposed as a public command")
        helper = prefix / "lib/bench-tools/agent/bin/agent-action-shell"
        if not helper.is_file() or not os.access(helper, os.X_OK):
            raise RuntimeError("Agent private helper is missing")
        run(["agent", "new", work / "agent home"])
        run(["agent", "check", work / "agent home"])
        run(["agent", "show", work / "agent home"])
        # Draft replaces its generated reference using the caller's umask.
        # A private shell must remain upgradeable without widening permissions.
        reference = prefix / "lib/bench-tools/draft/skills/draft/references/tools.md"
        reference.write_text("Stale reference for the installation fixture.\n")
        previous_umask = os.umask(0o077)
        try:
            run(["draft", "sync"])
        finally:
            os.umask(previous_umask)
        run(["brief", "cat", "draft"])
        run(["brief", "lint", "-strict", prefix / "lib/bench-tools/draft/skills/draft"])
        run(["draft", "new", work / "draft project"])
        template = prefix / "lib/bench-tools/draft/skills/draft/references/template.md"
        if (work / "draft project/DESIGN.md").read_bytes() != template.read_bytes():
            raise RuntimeError("Draft did not use its packaged template")
        generated = reference.read_bytes()
        reference_mode = reference.stat().st_mode & 0o777
        if reference_mode != 0o600:
            raise RuntimeError("Draft sync did not exercise its restrictive-umask replacement")
        subprocess.run([sys.executable, ROOT / "scripts/install", "--from-build", args.build_dir.resolve(),
                        "--prefix", prefix], check=True)
        if reference.read_bytes() != generated:
            raise RuntimeError("reinstall replaced Draft's generated reference")
        if reference.stat().st_mode & 0o777 != reference_mode:
            raise RuntimeError("reinstall changed Draft's generated reference permissions")
        # Unrelated prefix contents survive removal of all managed programs.
        unrelated = prefix / "bin/unrelated"
        unrelated.write_text("keep me\n")
        subprocess.run([sys.executable, ROOT / "scripts/uninstall", "--prefix", prefix], check=True)
        if unrelated.read_text() != "keep me\n":
            raise RuntimeError("uninstall changed unrelated content")
        for component in components:
            for command in component["commands"]:
                if os.path.lexists(prefix / "bin" / command["name"]):
                    raise RuntimeError("uninstall left a managed command")
    print("Install check passed: 20 commands, relocated assets, Draft refresh, repeat install, clean uninstall.")


if __name__ == "__main__":
    main()
