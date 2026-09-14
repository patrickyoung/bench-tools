#!/usr/bin/env python3
"""Require the real public Ask/Record contract; no silently skipped test."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source",type=Path,required=True)
    parser.add_argument("--ask",type=Path,required=True)
    args=parser.parse_args()
    env=dict(os.environ,RECORD_TEST_ASK=str(args.ask.resolve()))
    result=subprocess.run(["go","test","-count=1","-timeout","180s","-json","-run","^TestAskRecordContract$","."],
                          cwd=args.source,env=env,capture_output=True,text=True,timeout=210)
    events=[json.loads(line) for line in result.stdout.splitlines()]
    if result.returncode or any(e.get("Action")=="skip" for e in events) or not any(
            e.get("Test")=="TestAskRecordContract" and e.get("Action")=="pass" for e in events):
        raise SystemExit(result.stdout+result.stderr+"\nRequired Ask/Record contract did not pass without skips")
    print("ok Ask/Record executable contract: passed, no skips")


if __name__=="__main__":
    main()
