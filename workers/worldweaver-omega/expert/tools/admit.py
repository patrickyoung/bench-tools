#!/usr/bin/env python3
"""Controller-only: capture ORIGINAL input hashes before starting Agent."""
import argparse,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'lib'))
from contracts import *
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('workspace',type=Path)
p.add_argument('output',type=Path,help='NEW protected external admission file')
a=p.parse_args()
try:
    root=a.workspace.resolve()
    need(a.output.is_absolute() and not a.output.resolve().is_relative_to(root),'admission must be external')
    original=admission(root)
    with a.output.open('x') as out: out.write(json.dumps(original,indent=2)+'\n')
    need(admission(root)==original,'input changed during admission; discard and investigate')
    print(original['admissionHash'])
except (Reject,OSError) as e: print(e,file=sys.stderr);sys.exit(1)
