#!/usr/bin/env python3
"""Write a meeting brief through Brief and Ask; publish only on success."""
import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("notes", nargs="?", type=Path, default=Path("notes.txt"))
    parser.add_argument("output", nargs="?", type=Path, default=Path("brief.md"))
    args = parser.parse_args()
    temporary = None
    try:
        if args.notes.resolve() == args.output.resolve():
            raise ValueError("notes and output must be different files")
        if not args.output.parent.is_dir():
            raise ValueError("output directory does not exist")
        notes = args.notes.read_text(encoding="utf-8")
        if not notes.strip():
            raise ValueError("notes are empty")
        skill = Path(__file__).resolve().parent / "skills" / "meeting-brief"
        subprocess.run(["brief", "lint", "-strict", str(skill)], check=True, timeout=10)
        procedure = subprocess.run(["brief", "cat", str(skill)], check=True,
                                   stdout=subprocess.PIPE, text=True, timeout=10).stdout
        system = subprocess.run(["ask", "system"], check=True,
                                stdout=subprocess.PIPE, text=True, timeout=10).stdout
        run = args.output.parent / "runs" / uuid.uuid4().hex
        run.mkdir(parents=True)
        print(f"Conversation: {run / 'session.jsonl'}", file=sys.stderr)
        result = subprocess.run(
            ["ask", "-q", "-f", str(run / "session.jsonl"),
             "Write a meeting brief from the supplied notes using the procedure."],
            input=notes, text=True, stdout=subprocess.PIPE, timeout=90,
            env=dict(os.environ, ASK_SYSTEM=system.rstrip() + "\n" + procedure),
        )
        (run / "candidate.md").write_text(result.stdout, encoding="utf-8")
        if result.returncode:
            raise ValueError(f"Ask exited {result.returncode}; output was not replaced")
        if not result.stdout.strip():
            raise ValueError("Ask returned an empty answer; output was not replaced")
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8",
                                         dir=args.output.parent, prefix=".brief-",
                                         suffix=".tmp", delete=False) as out:
            temporary = Path(out.name)
            out.write(result.stdout)
        os.replace(temporary, args.output)
        temporary = None
    except (OSError, UnicodeError, ValueError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        return 1
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    print(f"Wrote {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
