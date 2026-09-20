#!/usr/bin/env python3
"""Run curated WorldWeaver fixtures in retained external scratch, never source."""
import argparse, os, shutil, subprocess, sys, tempfile
from pathlib import Path
p=argparse.ArgumentParser()
p.add_argument('profile',choices=['core','browser'])
p.add_argument('--node-modules',type=Path)
a=p.parse_args()
source=Path(__file__).resolve().parents[1]
scratch=Path(tempfile.mkdtemp(prefix='worldweaver-tests-'))
for folder in ('expert','tests'):
    shutil.copytree(source/folder,scratch/folder,ignore=shutil.ignore_patterns('__pycache__','*.pyc'))
env=dict(os.environ,WW_TEST_HOME=str(scratch/'expert'),PYTHONDONTWRITEBYTECODE='1')
node=shutil.which('node');assert node,'Node 22+ is required'
print('Retained fixture workspace:',scratch,flush=True)
if a.profile=='core':
    commands=[[node,'--test','tests/core.test.mjs']]
    commands += [[sys.executable,'tests/'+name] for name in ('check_test.py','review_test.py','breadth_test.py')]
    commands.append([sys.executable,'tests/hardening_test.py'])
    commands += [[node,'tests/browser-cleanup-test.mjs'],[node,'tests/browser-vertical-test.mjs',str(scratch/'vertical')],[node,'tests/browser-deadline-test.mjs',str(scratch/'deadline')]]
else:
    if not a.node_modules:p.error('--node-modules selects installed Three/Playwright/esbuild')
    deps=str(a.node_modules.resolve())
    commands=[[node,'tests/browser-core.mjs',deps,str(scratch/'browser-core')],
              [node,'tests/browser-nearmiss.mjs',deps,str(scratch/'browser-nearmiss')],
              [node,'tests/browser-clock.mjs',deps,str(scratch/'browser-nearmiss/moving-scene'),str(scratch/'clock')]]
for argv in commands:
    print('Executing:',repr(argv),flush=True)
    subprocess.run(argv,cwd=scratch,env=env,check=True,timeout=360)
