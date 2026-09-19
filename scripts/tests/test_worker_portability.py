"""Portable source projections preserve the canonical, independently usable expert.

Synthetic Git fixtures only: no host CLI, model, credentials, or job evaluation.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
TARGETS = ('skill', 'codex', 'claude-code', 'cowork', 'pi', 'openclaw', 'hermes')
SOURCE_ROOT = 'references/bench'


class WorkerPortability(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='bench-portability-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'repo'
        (self.root / 'scripts').mkdir(parents=True)
        for name in ('workers', 'worker_portability.py'):
            shutil.copyfile(SCRIPTS / name, self.root / 'scripts' / name)
        self.expert = self.root / 'workers/tiny/expert'
        self.write('AGENTS.md', 'Read skills/taught/SKILL.md. Produce the requested candidate.\n')
        self.write('README.md', 'Synthetic worker; bin/check reads the final candidate on stdin.\n')
        self.write('LICENSE', 'MIT synthetic fixture\n')
        self.write('SOUL.md', 'Use concise, factual prose.\n')
        self.write('MEMORY.md', 'Curated synthetic rule, unrelated to a private job.\n')
        self.write('GOAL.md', 'An explicit current goal replaces this fixture default.\n')
        self.write('PLAN.md', 'Before drafting, account for every supplied input.\n')
        self.write('skills/taught/SKILL.md',
                   '---\nname: taught\ndescription: Use for a synthetic checked repair.\n---\n'
                   '# Taught procedure\n\nRead [rubric](references/rubric.md).\n\n'
                   '## Lessons\n\nRetain the supplied units.\n'
                   '<!-- hone checked-source wording-source -->\n')
        self.write('skills/taught/references/rubric.md', 'Preserve source units and unknowns.\n')
        self.write('tools/fixture', '#!/bin/sh\n'
                   'test -z "$EXPORT_MUST_NOT_EXECUTE" || touch "$EXPORT_MUST_NOT_EXECUTE"\n', 0o755)
        self.write('bin/check', '#!/usr/bin/env python3\n'
                   'import os, pathlib, sys\n'
                   'if os.environ.get("EXPORT_MUST_NOT_EXECUTE"):\n'
                   ' pathlib.Path(os.environ["EXPORT_MUST_NOT_EXECUTE"]).touch()\n'
                   'root = pathlib.Path(__file__).resolve().parents[1]\n'
                   'assert pathlib.Path(os.environ["AGENT_HOME"]).resolve() == root\n'
                   'assert pathlib.Path(os.environ["AGENT_WORK"]).resolve() == pathlib.Path.cwd()\n'
                   'assert pathlib.Path(os.environ["AGENT_STATE"]).is_dir()\n'
                   'expected = pathlib.Path("expected.txt").read_bytes()\n'
                   'sys.exit(0 if sys.stdin.buffer.read() == expected else 1)\n', 0o755)
        self.metadata = {
            'id': 'tiny', 'description': 'A synthetic worker with a learned procedure.',
            'owner': 'fixture', 'status': 'active',
            'requires': {'python': '>=3.9', 'bench-tools': {'commands': ['agent', 'brief']}},
            'files': sorted(p.relative_to(self.root).as_posix()
                            for p in self.expert.rglob('*') if p.is_file()),
        }
        (self.expert.parent / 'worker.json').write_text(json.dumps(self.metadata))
        self.git('init', '-q')
        self.git('config', 'user.name', 'Fixture')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('remote', 'add', 'origin', 'https://github.com/example/fixture.git')
        self.commit = self.commit_source()
        self.sentinel = self.base / 'export-executed-source'

    def write(self, relative, text, mode=0o644):
        path = self.expert / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        path.chmod(mode)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args],
                                       stderr=subprocess.PIPE).decode().strip()

    def commit_source(self):
        self.git('add', '.')
        self.git('commit', '-qm', 'Synthetic portable source')
        return self.git('rev-parse', 'HEAD')

    def call(self, destination, *, kind='worker', name=None, ref=None, flags=(), ok=True):
        command, default_name = ('export-team', 'site') if kind == 'team' else ('export', 'tiny')
        name = name or default_name
        result = subprocess.run(
            [sys.executable, str(self.root / 'scripts/workers'), command, name,
             str(destination), '--ref', ref or self.commit, *flags],
            cwd=self.base, capture_output=True, text=True,
            env=dict(os.environ, EXPORT_MUST_NOT_EXECUTE=str(self.sentinel)), timeout=30)
        if ok:
            self.assertEqual(result.returncode, 0, result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertFalse(self.sentinel.exists(), 'export executed reusable source')
        return result

    def export(self, destination, target='skill', execution='native', **kwargs):
        return self.call(destination, flags=('--target', target, '--execution', execution), **kwargs)

    def make_team(self):
        expert = self.root / 'teams/site/expert'
        (expert / 'bin').mkdir(parents=True)
        for name in ('AGENTS.md', 'README.md', 'LICENSE'):
            (expert / name).write_text('Use bin/site-team to run the existing team wiring.\n')
        for name in ('check', 'site-team'):
            path = expert / 'bin' / name
            path.write_text('#!/bin/sh\n'
                            'test -z "$EXPORT_MUST_NOT_EXECUTE" || touch "$EXPORT_MUST_NOT_EXECUTE"\n'
                            'exit 1\n')
            path.chmod(0o755)
        metadata = {
            'id': 'site', 'description': 'Synthetic fixed team composition.',
            'owner': 'fixture', 'status': 'active', 'requires': {'bench-tools': 'fixture'},
            'members': {'first': {'worker': 'tiny'}, 'second': {'worker': 'tiny'}},
            'files': sorted(p.relative_to(self.root).as_posix()
                            for p in expert.rglob('*') if p.is_file()),
        }
        (expert.parent / 'team.json').write_text(json.dumps(metadata))
        return self.commit_source()

    def snapshot(self, directory):
        return {p.relative_to(directory).as_posix():
                (p.read_bytes(), stat.S_IMODE(p.stat().st_mode))
                for p in directory.rglob('*') if p.is_file()}

    def test_native_projection_preserves_original_expert_and_lock_bytes(self):
        plain, portable = self.base / 'plain', self.base / 'portable'
        self.call(plain)
        # Only the pinned reviewed source may enter either kind of export.
        self.write('AGENTS.md', 'Uncommitted current job data must not be exported.\n')
        self.write('current-job.txt', 'Untracked current job data.\n')
        self.export(portable)
        self.assertEqual({p.name for p in plain.iterdir()}, {'expert', 'worker.lock.json'})
        self.assertEqual(self.snapshot(portable / SOURCE_ROOT), self.snapshot(plain))
        self.assertFalse((portable / SOURCE_ROOT / 'expert/current-job.txt').exists())
        self.assertEqual({p.name for p in portable.iterdir()},
                         {'SKILL.md', 'PORTABILITY.md', 'portability.json', 'references'})
        self.assertEqual(set(self.snapshot(portable)) -
                         {SOURCE_ROOT + '/' + p for p in self.snapshot(plain)},
                         {'SKILL.md', 'PORTABILITY.md', 'portability.json'})

    def test_every_target_and_execution_has_an_explicit_projection_identity(self):
        for target in TARGETS:
            for execution in ('native', 'bench'):
                with self.subTest(target=target, execution=execution):
                    destination = self.base / (target + '-' + execution)
                    self.export(destination, target=target, execution=execution)
                    manifest = json.loads((destination / 'portability.json').read_text())
                    self.assertEqual(manifest['schema'], 'bench.portability/v1')
                    self.assertEqual(manifest['target'], target)
                    self.assertEqual(manifest['execution'], execution)
                    self.assertEqual(manifest['kind'], 'worker')
                    self.assertEqual(manifest['name'], 'bench-worker-tiny-' + execution)
                    self.assertEqual(manifest['source_root'], SOURCE_ROOT)
                    self.assertEqual(manifest['source_lock'], SOURCE_ROOT + '/worker.lock.json')
                    self.assertEqual(manifest['requirements'], self.metadata['requires'])
                    self.assertTrue(manifest['limitations'])
                    skill = (destination / 'SKILL.md').read_text()
                    self.assertTrue(skill.startswith('---\n'))
                    self.assertIn('bench-worker-tiny-' + execution, skill.split('---', 2)[1])

    def test_manifest_binds_generated_documents_without_relabeling_source(self):
        destination = self.base / 'projected'
        self.export(destination)
        manifest = json.loads((destination / 'portability.json').read_text())
        files = manifest['files']
        self.assertEqual(set(files), {'SKILL.md', 'PORTABILITY.md'})
        for relative, record in files.items():
            path = destination / relative
            self.assertEqual(record['sha256'], hashlib.sha256(path.read_bytes()).hexdigest())
            self.assertEqual(record['mode'], '100644')
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o644)
        lock = json.loads((destination / SOURCE_ROOT / 'worker.lock.json').read_text())
        self.assertEqual(lock['adaptations'], [])
        self.assertFalse(set(files) & set(lock['files']))

    def test_native_guidance_preserves_skill_routing_and_explicit_check_contract(self):
        destination = self.base / 'native'
        self.export(destination)
        guidance = '\n'.join((destination / path).read_text()
                             for path in ('SKILL.md', 'PORTABILITY.md'))
        for token in ('AGENTS.md', 'SOUL.md', 'MEMORY.md', 'GOAL.md', 'skills',
                      'SKILL.md', 'AGENT_HOME', 'AGENT_WORK', 'AGENT_STATE', 'bin/check'):
            self.assertIn(token, guidance)
        self.assertIn('stdin', guidance.lower())
        self.assertRegex(guidance.lower(), r'workspace|working directory')
        self.assertRegex(guidance.lower(), r'select|rout|relevant')
        self.assertNotIn(str(self.base), guidance, 'projection must survive relocation')
        links = re.findall(r'\]\(([^)]+)\)', guidance)
        self.assertIn(SOURCE_ROOT + '/expert/AGENTS.md', links)
        self.assertIn(SOURCE_ROOT + '/worker.lock.json', links)
        for link in links:
            if not re.match(r'[a-z]+://', link):
                self.assertTrue((destination / link).is_file(), link)

    def test_relocated_native_export_keeps_original_stdin_checker_usable(self):
        destination, relocated = self.base / 'before', self.base / 'after with spaces'
        self.export(destination)
        destination.rename(relocated)
        work, state = self.base / 'work', self.base / 'state'
        work.mkdir()
        state.mkdir()
        (work / 'expected.txt').write_bytes(b'accepted candidate\n')
        env = dict(os.environ, AGENT_HOME=str(relocated / SOURCE_ROOT / 'expert'),
                   AGENT_WORK=str(work), AGENT_STATE=str(state))
        env.pop('EXPORT_MUST_NOT_EXECUTE', None)
        check = relocated / SOURCE_ROOT / 'expert/bin/check'
        accepted = subprocess.run([str(check)], cwd=work, env=env, input=b'accepted candidate\n',
                                  capture_output=True, timeout=10)
        self.assertEqual(accepted.returncode, 0, accepted.stderr)
        rejected = subprocess.run([str(check)], cwd=work, env=env, input=b'different candidate\n',
                                  capture_output=True, timeout=10)
        self.assertEqual(rejected.returncode, 1, rejected.stderr)

    def test_native_projection_loads_unreferenced_standing_plan_as_evidence(self):
        destination = self.base / 'with-plan'
        self.assertNotIn('PLAN.md', (self.expert / 'AGENTS.md').read_text())
        self.export(destination)
        self.assertEqual((destination / SOURCE_ROOT / 'expert/PLAN.md').read_bytes(),
                         (self.expert / 'PLAN.md').read_bytes())
        guidance = (destination / 'SKILL.md').read_text()
        self.assertIn(SOURCE_ROOT + '/expert/PLAN.md', re.findall(r'\]\(([^)]+)\)', guidance))
        normalized = ' '.join(guidance.split())
        self.assertRegex(normalized, r'(?:Read|Load|Include) [^.]*PLAN\.md[^.]*standing[^.]*evidence')

    def test_native_response_contract_delivers_exact_checked_bytes(self):
        destination = self.base / 'response-contract'
        self.export(destination)
        guidance = ' '.join((destination / 'SKILL.md').read_text().split()).lower()
        self.assertRegex(guidance, r'(?:return|deliver) exactly the (?:accepted|checked) candidate bytes')
        self.assertRegex(guidance, r'(?:separate|external)[^.]*diagnostic channel')
        self.assertRegex(guidance, r'never append[^.]*summary[^.]*wrap[^.]*markdown')

        work, state = self.base / 'response-work', self.base / 'response-state'
        work.mkdir()
        state.mkdir()
        candidate = b'{"result":"accepted"}\n'
        (work / 'expected.txt').write_bytes(candidate)
        env = dict(os.environ, AGENT_HOME=str(destination / SOURCE_ROOT / 'expert'),
                   AGENT_WORK=str(work), AGENT_STATE=str(state))
        env.pop('EXPORT_MUST_NOT_EXECUTE', None)
        for response, expected_status in (
                (candidate, 0),
                (candidate + b'Check passed.\n', 1),
                (b'```json\n' + candidate + b'```\n', 1)):
            with self.subTest(response=response):
                checked = subprocess.run([str(destination / SOURCE_ROOT / 'expert/bin/check')],
                                         cwd=work, env=env, input=response,
                                         capture_output=True, timeout=10)
                self.assertEqual(checked.returncode, expected_status, checked.stderr)

    def test_learning_and_relative_skill_resources_export_without_rewriting(self):
        destination = self.base / 'learned'
        self.export(destination, target='hermes')
        for relative in ('skills/taught/SKILL.md', 'skills/taught/references/rubric.md'):
            self.assertEqual((destination / SOURCE_ROOT / 'expert' / relative).read_bytes(),
                             (self.expert / relative).read_bytes())
        self.assertIn(b'<!-- hone checked-source wording-source -->',
                      (destination / SOURCE_ROOT / 'expert/skills/taught/SKILL.md').read_bytes())

    def test_bench_team_retains_roster_wiring_and_original_lock(self):
        ref = self.make_team()
        plain, projected = self.base / 'team-plain', self.base / 'team-projected'
        self.call(plain, kind='team', ref=ref)
        self.export(projected, target='cowork', execution='bench', kind='team', ref=ref)
        self.assertEqual({p.name for p in plain.iterdir()}, {'expert', 'team.lock.json'})
        self.assertEqual(self.snapshot(projected / SOURCE_ROOT), self.snapshot(plain))
        self.assertEqual({p.name for p in projected.iterdir()},
                         {'SKILL.md', 'PORTABILITY.md', 'portability.json', 'references'})
        manifest = json.loads((projected / 'portability.json').read_text())
        self.assertEqual(manifest['kind'], 'team')
        self.assertEqual(manifest['name'], 'bench-team-site-bench')
        self.assertEqual(manifest['source_root'], SOURCE_ROOT)
        self.assertEqual(manifest['source_lock'], SOURCE_ROOT + '/team.lock.json')
        self.assertEqual(manifest['requirements'], {'bench-tools': 'fixture'})
        self.assertEqual(set(json.loads((projected / SOURCE_ROOT / 'team.lock.json').read_text())['members']),
                         {'first', 'second'})

    def test_native_team_refuses_before_creating_destination(self):
        ref = self.make_team()
        for target in TARGETS:
            destination = self.base / ('native-team-' + target)
            self.export(destination, kind='team', ref=ref, target=target, ok=False)
            self.assertFalse(destination.exists())

    def test_partial_or_unknown_portability_arguments_refuse(self):
        cases = (('--target', 'codex'), ('--execution', 'native'),
                 ('--target', 'unknown', '--execution', 'native'),
                 ('--target', 'skill', '--execution', 'unknown'))
        for index, flags in enumerate(cases):
            destination = self.base / ('invalid-' + str(index))
            self.call(destination, flags=flags, ok=False)
            self.assertFalse(destination.exists())

    def test_oversized_generated_name_refuses_before_creating_destination(self):
        name = 'a' * 45
        self.assertEqual(len('bench-worker-' + name + '-native'), 65)
        renamed = self.root / 'workers' / name
        self.expert.parent.rename(renamed)
        self.expert = renamed / 'expert'
        self.metadata['id'] = name
        self.metadata['files'] = sorted(p.relative_to(self.root).as_posix()
                                        for p in self.expert.rglob('*') if p.is_file())
        (renamed / 'worker.json').write_text(json.dumps(self.metadata))
        ref = self.commit_source()
        # This is valid Bench source; only the generated skill name is too long.
        self.call(self.base / 'plain-long-name', name=name, ref=ref)
        destination = self.base / 'uncreated-parent' / 'projected'
        rejected = self.export(destination, name=name, ref=ref, ok=False)
        self.assertRegex(rejected.stderr, r'portable skill name.*64')
        self.assertFalse(destination.exists())
        self.assertFalse(destination.parent.exists())

    def test_projection_does_not_overwrite_existing_directory_or_symlink(self):
        existing = self.base / 'existing'
        existing.mkdir()
        (existing / 'keep.txt').write_text('Caller-owned files.\n')
        self.export(existing, ok=False)
        self.assertEqual(list(existing.iterdir()), [existing / 'keep.txt'])
        self.assertEqual((existing / 'keep.txt').read_text(), 'Caller-owned files.\n')
        link = self.base / 'link'
        link.symlink_to(existing, target_is_directory=True)
        self.export(link, ok=False)
        self.assertTrue(link.is_symlink())
        self.assertEqual(list(existing.iterdir()), [existing / 'keep.txt'])


if __name__ == '__main__':
    unittest.main()
