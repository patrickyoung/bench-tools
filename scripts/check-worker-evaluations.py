#!/usr/bin/env python3
"""Run the worker/team evaluation catalog against current source, without models.

Repository verification only: literal public commands and each suite's existing
contracts. No provider clients, worker loops, installation or runtime authority.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / 'scripts/worker-evaluations.json'
PROFILES = ('core', 'analysis', 'native', 'browser', 'workbook-visual', 'workbook-render', 'workbook-process')
SELECTORS = ('INKSCAPE', 'PLAYWRIGHT_BROWSERS_PATH', 'WORKBOOK_PYTHON',
             'WORKBOOK_VISUAL_SOFFICE', 'WORKBOOK_VISUAL_PDFTOPPM', 'WORKBOOK_VISUAL_CAGE',
             'WORKBOOK_NODE', 'WORKBOOK_NODE_MODULES')


def catalog(root=ROOT):
    data = json.loads((root/'scripts/worker-evaluations.json').read_text())
    entries = {f'{kind}/{p.parent.name}' for kind, name in [('workers','worker.json'), ('teams','team.json')]
               for p in (root/kind).glob('*/'+name)}
    covered, ids = set(), set()
    for suite in data['suites']:
        if suite['id'] in ids or not re.fullmatch('[a-z0-9-]+', suite['id']):
            raise ValueError('duplicate/invalid suite ID: '+suite['id'])
        ids.add(suite['id'])
        if suite['profile'] not in PROFILES or not suite['covers'] or not suite['commands']:
            raise ValueError('suite needs a profile, coverage and commands: '+suite['id'])
        if set(suite['covers']) - entries:
            raise ValueError('unknown library entries in '+suite['id'])
        covered.update(suite['covers'])
        for command in suite['commands']:
            if not command or not all(isinstance(arg, str) for arg in command):
                raise ValueError('commands must be literal argv lists')
        for file in suite['tests']:
            p = Path(file)
            if p.is_absolute() or '..' in p.parts or not (root/p).is_file():
                raise ValueError('missing/unsafe test: '+file)
    if entries != covered:
        raise ValueError('library entries without evaluation suites: '+', '.join(sorted(entries-covered)))
    quality = data['quality_cases']
    if set(quality) != entries:
        raise ValueError('quality case coverage must match every worker and team')
    for entry, cases in quality.items():
        for key in ('ordinary', 'missing', 'near_miss', 'transfer', 'rubric'):
            if not cases.get(key):
                raise ValueError(entry+' missing quality case '+key)
    # Fail when a new executable test is not wired, instead of letting discovery drift.
    mapped = {p for suite in data['suites'] for p in suite['tests']}
    exceptions = data.get('manual_tests', {})
    for base in ('workers', 'teams'):
        for p in (root/base).glob('*/tests/*'):
            if p.suffix in ('.py', '.mjs') and (p.name.startswith('test_') or p.name in ('contracts.py','browser.mjs')):
                rel = p.relative_to(root).as_posix()
                if rel not in mapped and rel not in exceptions:
                    raise ValueError('unmapped test: '+rel)
    return data


def snapshot(root, destination):
    """Retain exact current tracked/new source, excluding ignored runtime files."""
    raw = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z',
                                   '--', 'workers', 'teams', 'scripts'], cwd=root)
    files = {}
    for name in sorted(set(os.fsdecode(n) for n in raw.split(b'\0') if n)):
        source = root/name
        if not source.exists():
            continue
        if source.is_symlink() or not source.is_file():
            raise ValueError('source must be a regular file: '+name)
        target = destination/name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)
        files[name] = {'sha256': hashlib.sha256(target.read_bytes()).hexdigest(),
                       'mode': source.stat().st_mode & 0o777}
    return files


def assemble(stage):
    """Test staging only; public exports still use scripts/workers and a Git pin."""
    for metadata in (stage/'teams').glob('*/team.json'):
        team = json.loads(metadata.read_text())
        expert = metadata.parent/'expert'
        for role, member in team['members'].items():
            worker = stage/'workers'/member['worker']
            definition = json.loads((worker/'worker.json').read_text())
            target = expert/'agents'/role
            for source in definition['files']:
                relative = Path(source).relative_to(Path('workers')/member['worker']/'expert')
                dest = target/relative
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(stage/source, dest)
            if member.get('adapter'):
                dest = expert/'bin/workers'/role
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(expert/member['adapter'], dest)


def environment(stage, run, python):
    home, tmp, binary = run/'home', run/'tmp', run/'bin'
    for p in (home, tmp, binary):
        p.mkdir()
    (binary/'python3').symlink_to(python)
    env = {'PATH': str(binary)+os.pathsep+os.environ.get('PATH', '/usr/bin:/bin'),
           'HOME': str(home), 'TMPDIR': str(tmp), 'LANG': 'en_US.UTF-8',
           'PYTHONDONTWRITEBYTECODE': '1', 'PYTHONNOUSERSITE': '1',
           'XDG_CONFIG_HOME': str(home/'.config'), 'XDG_CACHE_HOME': str(home/'.cache')}
    for key in SELECTORS:
        if key in os.environ:
            env[key] = os.environ[key]
    for name in ('product-owner', 'product-manager', 'enterprise-architect', 'visual-artist', 'polars-analyst'):
        env[name.upper().replace('-', '_')+'_EXPERT'] = str(stage/'workers'/name/'expert')
    env['COMPARISON_TEAM_EXPERT'] = str(stage/'teams/vendor-comparison-team/expert')
    env['POLARS_PYTHON'] = str(python)
    env.setdefault('WORKBOOK_PYTHON', str(python))
    return env


def execute(argv, cwd, env, log, timeout):
    start = time.monotonic()
    with log.open('wb') as output:
        try:
            proc = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                    stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        except OSError as exc:
            output.write(str(exc).encode())
            return {'exit': None, 'status': 'error', 'seconds': time.monotonic()-start}
        try:
            code = proc.wait(timeout=timeout)
        except (subprocess.TimeoutExpired, KeyboardInterrupt) as exc:
            os.killpg(proc.pid, signal.SIGTERM)
            try:
                proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
            if isinstance(exc, KeyboardInterrupt):
                raise
            return {'exit': None, 'status': 'timeout', 'seconds': time.monotonic()-start}
    return {'exit': code, 'status': 'passed' if code == 0 else 'failed', 'seconds': time.monotonic()-start}


def prerequisites(suite, env, python, stage):
    missing = []
    for command in suite.get('prerequisites', {}).get('commands', []):
        if not shutil.which(command, path=env['PATH']):
            missing.append(command+' on PATH')
    for name in suite.get('prerequisites', {}).get('selectors', []):
        if not env.get(name) or not Path(env[name]).exists():
            missing.append(name+' must select an existing path')
    modules = suite.get('prerequisites', {}).get('modules', [])
    if modules:
        probe = subprocess.run([str(python), '-c',
            'import importlib.util,sys;sys.exit(any(importlib.util.find_spec(m) is None for m in sys.argv[1:]))',
            *modules], env=env, capture_output=True, timeout=20)
        if probe.returncode:
            missing.append('selected Python modules: '+', '.join(modules))
    if suite['profile'] == 'native' and not env.get('INKSCAPE') and not shutil.which('inkscape', path=env['PATH']):
        missing.append('INKSCAPE or inkscape on PATH')
    if suite['profile'] == 'browser':
        installed = stage/'teams/page-team/expert/node_modules/playwright/package.json'
        expected = json.loads((stage/'teams/page-team/expert/package.json').read_text())['dependencies']['playwright']
        if not installed.is_file() or json.loads(installed.read_text()).get('version') != expected:
            missing.append('--node-modules with the page-team pinned Playwright '+expected)
    return missing


def quality_plan(data, entries, destination):
    destination = destination.expanduser().resolve()
    if destination == ROOT or ROOT in destination.parents:
        raise ValueError('quality plan must be outside the source checkout')
    destination.mkdir(parents=True, exist_ok=False)
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    lines = ['# Library quality evaluation plan', '', 'Status: NOT RUN. This command made no model calls.', '',
             'Source checkout: '+str(ROOT), 'Baseline commit: '+revision,
             'Working-tree adaptations must be pinned or hashed separately before model runs.', '',
             'For each case, prepare current inputs following the entry README; use a fresh workspace and',
             'the existing Agent/team entry command. Freeze independent expected outcomes before running.',
             'These cases are visible regressions, not hidden tests. See docs/WORKER-EVALUATIONS.md.', '',
             'Retain invocation, source/input/output hashes, model, limits, exit status, records, checker',
             'feedback, independent review, elapsed time, and measured usage/cost (unknown if unavailable).',
             'Record pass, reject, unfinished, or error for every attempt; never erase failed attempts.', '']
    for entry in sorted(entries):
        lines += ['## '+entry, '', 'Definition: '+str(ROOT/entry/'expert/README.md'), '']
        for key, value in data['quality_cases'][entry].items():
            lines += ['### '+key.replace('_', ' ').title(), '', value, '',
                      'Evidence / independent verdict: NOT RUN.', '']
    (destination/'EVALUATION.md').write_text('\n'.join(lines))
    (destination/'cases.json').write_text(json.dumps({e:data['quality_cases'][e] for e in sorted(entries)}, indent=2)+'\n')
    print(destination/'EVALUATION.md')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--list', action='store_true', help='show coverage and prerequisite groups; execute nothing')
    parser.add_argument('--quality-plan', type=Path, help='write an external, unrun quality case plan; no model calls')
    parser.add_argument('--profile', choices=(*PROFILES, 'all'), action='append', help='repeatable; default core + analysis')
    parser.add_argument('--entry', action='append', help='worker/team ID or workers/ID / teams/ID; repeatable')
    parser.add_argument('--out', type=Path, help='new evidence directory outside the checkout; default new /tmp directory')
    parser.add_argument('--python', default=sys.executable, help='Python interpreter for suites and their python3 subprocesses')
    parser.add_argument('--node-modules', type=Path, help='explicit installed page-team node_modules for browser profile')
    parser.add_argument('--bin-dir', type=Path, help='explicit Bench binaries for workbook-process fixtures')
    parser.add_argument('--ask', type=Path, help='explicit public Ask executable for workbook-process fixtures')
    parser.add_argument('--timeout', type=int, default=600, help='timeout in seconds per command (default 600)')
    args = parser.parse_args(argv)
    data = catalog()
    profiles = set(args.profile or ['core', 'analysis'])
    if 'all' in profiles:
        profiles = set(PROFILES)
    wanted = set()
    entries = set(data['quality_cases'])
    for name in args.entry or []:
        matches = {e for e in entries if e == name or e.split('/')[1] == name}
        if not matches:
            parser.error('unknown entry: '+name)
        wanted.update(matches)
    if args.quality_plan:
        if args.list or args.out or args.profile:
            parser.error('--quality-plan is separate from suite execution/listing; only --entry may filter it')
        quality_plan(data, wanted or entries, args.quality_plan)
        return 0
    suites = [s for s in data['suites'] if s['profile'] in profiles and
              (not wanted or wanted.intersection(s['covers']))]
    if args.list:
        for s in suites:
            print(f"{s['id']} [{s['profile']}] {', '.join(s['covers'])}\n  {s['proves']}\n  needs: {s['requires']}")
        return 0
    if not suites:
        parser.error('selection has no suites; use --list --profile all')
    if args.timeout < 1:
        parser.error('--timeout must be positive')
    python = Path(shutil.which(args.python) or args.python).absolute()
    if not python.is_file():
        parser.error('Python interpreter does not exist')
    run = args.out.expanduser().resolve() if args.out else Path(tempfile.mkdtemp(prefix='bench-evaluations-')).resolve()
    if run == ROOT or ROOT in run.parents:
        parser.error('--out must be outside the source checkout')
    if args.out:
        run.mkdir(parents=True, exist_ok=False)
    report = {'schema': 'bench.library-evaluation/v1', 'status': 'running',
              'started': datetime.now(timezone.utc).isoformat(),
              'source_root': str(ROOT), 'revision': subprocess.check_output(['git','rev-parse','HEAD'], cwd=ROOT, text=True).strip(),
              'source_kind': 'current working tree; not a committed export',
              'profiles': sorted(profiles), 'selected_entries': sorted(wanted or entries),
              'not_selected': [s['id'] for s in data['suites'] if s not in suites], 'results': []}
    def save():
        (run/'summary.json').write_text(json.dumps(report, indent=2)+'\n')
    save()
    print('Evidence: '+str(run), flush=True)
    try:
        stage = run/'source'
        files = snapshot(ROOT, stage)
        (run/'source-manifest.json').write_text(json.dumps(files, indent=2)+'\n')
        # Check the unassembled library first, including source inventory rules.
        check = subprocess.run([str(python), str(stage/'scripts/workers'), 'check'],
                               capture_output=True, text=True, timeout=30,
                               env={'PATH': os.environ.get('PATH', '/usr/bin:/bin'), 'PYTHONDONTWRITEBYTECODE':'1'})
        (run/'library-check.log').write_text(check.stdout+check.stderr)
        if check.returncode:
            raise ValueError('source inventory check failed; see library-check.log')
        assemble(stage)
        if args.node_modules:
            modules = args.node_modules.expanduser().resolve()
            if not (modules/'playwright/package.json').is_file():
                raise ValueError('--node-modules must contain the explicitly installed Playwright dependency')
            (stage/'teams/page-team/expert/node_modules').symlink_to(modules, target_is_directory=True)
        env = environment(stage, run, python)
        report['runtime'] = {'python': str(python), 'selectors': {k:env[k] for k in SELECTORS if k in env},
                             'node_modules': str(args.node_modules.resolve()) if args.node_modules else None}
        probe = subprocess.run([str(python), '-c',
            'import importlib.metadata,json,sys;names=("numpy","polars","scipy");'
            'installed={d.metadata["Name"].lower():d.version for d in importlib.metadata.distributions()};'
            'print(json.dumps({"python":sys.version,"packages":{n:installed.get(n) for n in names}}))'],
            env=env, capture_output=True, text=True, timeout=20)
        if probe.returncode:
            raise ValueError('cannot identify selected Python runtime: '+probe.stderr)
        report['runtime']['versions'] = json.loads(probe.stdout)
        node = shutil.which('node', path=env['PATH'])
        if node:
            report['runtime']['node'] = {'path':node, 'version':subprocess.check_output([node,'--version'],env=env,text=True,timeout=10).strip()}
        (run/'logs').mkdir()
        values = {'python': str(python), 'stage': str(stage), 'run': str(run)}
        for key, path in [('bin_dir', args.bin_dir), ('ask', args.ask)]:
            values[key] = str(path.expanduser().resolve()) if path else ''
        for suite in suites:
            result = {'suite': suite['id'], 'covers': suite['covers'], 'status': 'passed', 'commands': []}
            missing = prerequisites(suite, env, python, stage)
            if suite['profile'] == 'workbook-process':
                if not args.bin_dir or not all((args.bin_dir/p).is_file() for p in ('ask','record','cage')):
                    missing.append('--bin-dir containing ask, record and cage')
                if not args.ask or not args.ask.is_file():
                    missing.append('--ask selecting a public Ask executable')
            if missing:
                result.update(status='unavailable', missing=missing)
                report['results'].append(result)
                save()
                print('UNAVAILABLE '+suite['id']+': '+'; '.join(missing), flush=True)
                continue
            for index, command in enumerate(suite['commands']):
                argv = [arg.format(**values) for arg in command]
                log = run/'logs'/f"{suite['id']}-{index+1}.log"
                outcome = execute(argv, stage, env, log, args.timeout)
                result['commands'].append({'argv': argv, 'cwd': str(stage), 'log': str(log), **outcome})
                if outcome['status'] != 'passed':
                    result['status'] = outcome['status']
            report['results'].append(result)
            save()
            print(f"{result['status'].upper():7} {suite['id']}", flush=True)
        report['status'] = 'passed' if all(r['status']=='passed' for r in report['results']) else 'failed'
    except KeyboardInterrupt:
        report['status'] = 'interrupted'
    except Exception as exc:
        report['status'] = 'error'
        report['error'] = str(exc)
        print(str(exc), file=sys.stderr)
    finally:
        report['finished'] = datetime.now(timezone.utc).isoformat()
        save()
    print(report['status'].upper()+': '+str(run/'summary.json'))
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
