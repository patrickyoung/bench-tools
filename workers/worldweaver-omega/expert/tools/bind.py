#!/usr/bin/env python3
import argparse,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'lib'))
from contracts import bindings,Reject
p=argparse.ArgumentParser(description='Bind current input/reference/output bytes; does not grant acceptance.')
p.add_argument('workspace',type=Path)
a=p.parse_args()
try:
    d=bindings(a.workspace.resolve())
    (a.workspace/'manifest.json').write_text(json.dumps(d,indent=2)+'\n')
    print(d['candidateHash'])
except Reject as e: print(e,file=sys.stderr); sys.exit(1)
