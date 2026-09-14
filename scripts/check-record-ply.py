#!/usr/bin/env python3
"""Prove recording at Ply's public interpreter seam, beyond presentation caps."""
import argparse
import base64
import json
import os
from pathlib import Path
import runpy
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
support = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
invoke, require, fixture = (support[x] for x in ("invoke", "require", "model_fixture"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    with tempfile.TemporaryDirectory(prefix="bench-record-ply-") as tmp:
        work = Path(tmp).resolve()
        records = work / "records"
        records.mkdir()
        env = {"PATH": str(bins) + os.pathsep + "/usr/bin:/bin:/usr/sbin:/sbin",
               "HOME": str(work), "ASK_LIVE": "", "RECORD_DIR": str(records),
               "RECORD_BIN": str(bins / "record"), "RECORD_ASK": str(bins / "ask")}
        adapter = ROOT / "tools/record/examples/ply-shell"
        script = "i=0; while [ $i -lt 512 ]; do printf '0123456789abcdef'; i=$((i+1)); done; printf 'separate-error\\n' >&2"
        answers = iter(["```sh\n" + script + "\n```", "Recorded the command."])
        with fixture(env, lambda _: next(answers)) as (model_env, calls):
            result = invoke([bins / "ply", "-sh", "-C", work, "-f", work / "ply.jsonl",
                             "-m", "openai/fixture", "-shell", adapter, "-cap", "1024", "-turns", "3",
                             "Print the fixture."], cwd=work, env=model_env)
            require(len(calls) == 2, "Ply made unexpected model requests")
            require(result.stdout == b"Recorded the command.\n", "Ply's answer contract changed")
        sessions = sorted(records.glob("*/session.jsonl"))
        require(len(sessions) == 1, "expected exactly one recorded interpreter invocation")
        file = sessions[0]
        stdout = invoke([bins / "record", "replay", "-f", file, "-stream", "stdout"], cwd=work, env=env)
        stderr = invoke([bins / "record", "replay", "-f", file, "-stream", "stderr"], cwd=work, env=env)
        require(stdout.stdout == b"0123456789abcdef" * 512, "Ply cap shortened retained output")
        require(stderr.stdout == b"separate-error\n", "Ply recording merged the streams")
        raw = invoke([bins / "record", "replay", "-f", file, "-json"], cwd=work, env=env)
        receipt = json.loads(raw.stdout)
        require([base64.b64decode(x).decode() for x in receipt["intent"]["argv"]]
                == ["/bin/sh", "-c", script], "literal interpreter/script selection changed")
        invoke([bins / "ask", "replay", "-check", work / "ply.jsonl"], cwd=work, env=env)
        trail = invoke([bins / "trail", "check", file.parent], cwd=work, env=env)
        checks = [json.loads(line) for line in trail.stdout.splitlines()]
        require(len(checks) == 1 and checks[0].get("ok") is True, "Trail did not check the recording")
        # The provider fixture is now closed. Replay must remain offline.
        replay = invoke([bins / "record", "replay", "-f", file], cwd=work, env=env)
        require(replay.stdout == stdout.stdout and replay.stderr == stderr.stdout, "offline replay changed bytes")
    print("ok Record -> Ply interpreter: full separate streams beyond model cap, literal script, offline replay")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit(str(error))
