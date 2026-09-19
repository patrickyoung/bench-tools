#!/usr/bin/env python3
"""Offline public Hire/Agent compatibility for optional Weigh authoring.

Predetermined loopback responses exercise process contracts, not builder judgment
or model quality. No Weigh executable, credentials or authoring schema is needed.
"""
import argparse
import json
import os
from pathlib import Path
import runpy
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SUPPORT = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
require, invoke = SUPPORT["require"], SUPPORT["invoke"]


def write_action(files):
    return ("python3 - <<'HIRE_FIXTURE'\nimport json\nfrom pathlib import Path\nfiles = json.loads("
            + repr(json.dumps(files)) + ")\nfor name, body in files.items():\n"
            "    path = Path(name)\n    path.parent.mkdir(parents=True, exist_ok=True)\n"
            "    path.write_text(body)\n    if body.startswith('#!'): path.chmod(0o700)\nHIRE_FIXTURE\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    parser.add_argument("--hire", type=Path, help="separately built Hire override")
    parser.add_argument("--agent", type=Path, help="separately built Agent override")
    args = parser.parse_args()
    bins, hire = args.bin_dir.resolve(), (args.hire or args.bin_dir / "hire").resolve()
    agent = (args.agent or bins / "agent").resolve()
    for binary in (hire, agent, *(bins / name for name in ("ask", "brief", "ply", "cage", "record"))):
        require(binary.is_file() and os.access(binary, os.X_OK), "missing executable: " + str(binary))
    with tempfile.TemporaryDirectory(prefix="hire-optional-weigh-") as tmp:
        root = Path(tmp).resolve()
        workspace, evidence = root / "authoring", root / "evidence"
        workspace.mkdir()
        env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "BENCH_REPLAY_RECORD_DIR", "BENCH_REPLAY_BIN_DIR") if key in os.environ}
        env.update(HOME=str(root / "home"), PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
                   HIRE_AGENT=str(agent), PYTHONDONTWRITEBYTECODE="1")
        env.update({"AGENT_" + name.upper(): str(bins / name) for name in ("ask", "brief", "ply", "cage", "record")})
        marker = root / "generated-check-executed"
        initial = {
            "expert/AGENTS.md": "Read the selected input and produce the requested candidate.\n",
            "expert/README.md": "Input and output are caller-selected files. The check requires candidate.txt; its presence alone does not establish factual quality.\n",
            "expert/bin/check": "#!" + sys.executable + "\nfrom pathlib import Path\nPath(" + repr(str(marker)) + ").touch()\nraise SystemExit(1)\n",
        }
        turns = []
        stages = ['test "$BENCH_WEIGH" = 0\n' + write_action(initial), None]
        def respond(request):
            if "You are choosing which skill" in request.get("instructions", ""):
                return "none"
            turns.append(request)
            require(len(turns) <= len(stages), "ordinary build required unexpected authoring work")
            stage = stages[len(turns) - 1]
            return "```ply\n" + stage + "```" if stage else "Definition ready."
        command = [hire, "build", "-no-cage", "-C", workspace, "-evidence", evidence,
                   "-m", "openai/fixture", "--"]
        with SUPPORT["model_fixture"](env, respond) as (fixture_env, _):
            built = invoke(command + ["Build a simple candidate worker."], cwd=root, env=fixture_env)
        require(built.stdout == b"Definition ready.\n" and len(turns) == 2, "ordinary build did not finish")
        definition = workspace / "expert"
        require(not (workspace / "build.json").exists() and not (workspace / "evaluation").exists(),
                "ordinary build required Weigh metadata or evaluation files")
        require(not marker.exists(), "builder verification executed generated code")

        # An existing custom optional filter is source to preserve, not a reason
        # for an unrelated default-off revision to migrate the worker.
        adapter = definition / "bin/choose-fix"
        adapter.write_text('#!/bin/sh\nset -eu\ncase "${BENCH_WEIGH:-0}" in\n  1) exec weigh "$@" ;;\n  *) exit 2 ;;\nesac\n')
        adapter.chmod(0o700)
        instructions = definition / "AGENTS.md"
        instructions.write_text(instructions.read_text() + "When explicitly selected, bin/choose-fix composes Weigh; the caller owns action dispatch and independent acceptance.\n")
        before = {p.relative_to(definition): p.read_bytes() for p in definition.rglob("*") if p.is_file()}
        turns.clear()
        stages[:] = ['test "$BENCH_WEIGH" = 0\nprintf "\\nDocumentation correction.\\n" >> expert/README.md\n', None]
        with SUPPORT["model_fixture"](env, respond) as (fixture_env, _):
            revised = invoke(command + ["Correct only the README. Preserve existing dependencies and runtime source."], cwd=root, env=fixture_env)
        require(revised.stdout == b"Definition ready.\n" and len(turns) == 2, "unrelated default-off revision did not finish")
        after = {p.relative_to(definition): p.read_bytes() for p in definition.rglob("*") if p.is_file()}
        require(after.pop(Path("README.md")) == before.pop(Path("README.md")) + b"\nDocumentation correction.\n"
                and after == before, "unrelated revision changed the existing Weigh dependency or worker source")
        for setting in ("0", "1", "invalid"):
            invoke([hire, "verify", definition], cwd=root, env=dict(env, BENCH_WEIGH=setting))
        require(not marker.exists(), "portable verification executed generated code")
        require(not (workspace / "build.json").exists() and not (workspace / "evaluation").exists(),
                "unrelated revision introduced mandatory Weigh authoring files")
        for session in (evidence / "runs").glob("*.jsonl"):
            invoke([bins / "ask", "replay", "-check", session], cwd=root, env=env)
        print("ok Hire -> Agent: ordinary default-off build and existing-Weigh documentation revision need no metadata; dependency preserved")
        print("ok Hire verify: independent of Weigh preference, no generated code executed; loopback fixtures establish process behavior only")


if __name__ == "__main__":
    main()
