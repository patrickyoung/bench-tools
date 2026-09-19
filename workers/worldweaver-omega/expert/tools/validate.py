#!/usr/bin/env python3
import argparse,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'lib'))
from contracts import *
p=argparse.ArgumentParser(description='Read-only request or blueprint/lighting validation (no semantic acceptance).')
g=p.add_mutually_exclusive_group(required=True)
g.add_argument('--request',type=Path)
g.add_argument('--phase',nargs=2,type=Path)
a=p.parse_args()
try:
    if a.request: request(load(a.request),a.request.resolve().parent)
    else: blueprint(load(a.phase[0])); lighting(load(a.phase[1]))
    print('Contract valid; semantic/runtime review still required.')
except (Reject,KeyError,TypeError,AttributeError) as e: print(e,file=sys.stderr); sys.exit(1)
