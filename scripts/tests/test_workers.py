"""Synthetic release-boundary cases; no models, credentials or real artifacts."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'workers'


class WorkerLibrary(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'repo'
        (self.root / 'scripts').mkdir(parents=True)
        (self.root / 'scripts/workers').write_bytes(SCRIPT.read_bytes())
        self.expert = self.root / 'workers/tiny/expert'
        (self.expert / 'bin').mkdir(parents=True)
        for name in ('AGENTS.md', 'README.md', 'LICENSE'):
            (self.expert / name).write_text('Synthetic source fixture.\n')
        (self.expert / 'bin/check').write_text('#!/bin/sh\nexit 1\n')
        (self.expert / 'bin/check').chmod(0o755)
        self.metadata = {'id': 'tiny', 'description': 'Synthetic test worker',
                         'owner': 'fixture', 'status': 'active',
                         'requires': {'fixture-runtime': '1'},
                         'files': sorted(p.relative_to(self.root).as_posix()
                                         for p in self.expert.rglob('*') if p.is_file())}
        self.save_metadata()
        self.git('init', '-q')
        self.git('config', 'user.name', 'Fixture')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('remote', 'add', 'origin', 'https://github.com/example/fixture.git')
        self.commit = self.commit_source()
        self.dest = Path(self.temp.name) / 'export'

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE).decode().strip()

    def save_metadata(self):
        (self.expert.parent / 'worker.json').write_text(json.dumps(self.metadata))

    def commit_source(self):
        self.git('add', '.')
        self.git('commit', '-qm', 'Synthetic source')
        return self.git('rev-parse', 'HEAD')

    def call(self, *args, ok=True):
        result = subprocess.run([sys.executable, str(self.root / 'scripts/workers'), *args],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0 if ok else 1, result.stderr)
        return result

    def export(self, ref=None, ok=True, extra=()):
        return self.call('export', 'tiny', str(self.dest), '--ref', ref or self.commit, *extra, ok=ok)

    def make_team(self):
        self.team = self.root / 'teams/site/expert'
        (self.team / 'bin').mkdir(parents=True)
        for name in ('AGENTS.md', 'README.md', 'LICENSE'):
            (self.team / name).write_text('Synthetic team wiring.\n')
        (self.team / 'bin/check').write_text('#!/bin/sh\ntouch EXPORT_EXECUTED_CODE\nexit 1\n')
        (self.team / 'bin/check').chmod(0o755)
        self.team_metadata = {'id': 'site', 'description': 'Synthetic team',
            'owner': 'fixture', 'status': 'active', 'requires': {'fixture': '1'},
            'members': {'first': {'worker': 'tiny'}, 'second': {'worker': 'tiny'}},
            'files': sorted(p.relative_to(self.root).as_posix() for p in self.team.rglob('*') if p.is_file())}
        self.save_team()
        return self.commit_source()

    def save_team(self):
        (self.team.parent / 'team.json').write_text(json.dumps(self.team_metadata))

    def export_team(self, ref, ok=True, extra=()):
        return self.call('export-team', 'site', str(self.dest), '--ref', ref, *extra, ok=ok)

    def test_team_assembles_pinned_worker_copies_and_records_ownership(self):
        commit = self.make_team()
        (self.expert / 'AGENTS.md').write_text('UNCOMMITTED JOB CONTENT')
        self.team_metadata['members'] = {'changed': {'worker': 'absent'}}
        self.save_team()
        self.export_team(commit)
        lock = json.loads((self.dest / 'team.lock.json').read_text())
        self.assertEqual(lock['schema'], 'bench.team-lock/v1')
        self.assertEqual(len(lock['files']), 12)
        self.assertEqual(set(lock['members']), {'first', 'second'})
        for slot, member in lock['members'].items():
            self.assertEqual(member['worker'], 'tiny')
            self.assertEqual(member['source']['commit'], commit)
            self.assertEqual(member['source']['path'], 'workers/tiny/expert')
            self.assertEqual(member['destination'], 'expert/agents/' + slot)
            definition = self.dest / member['destination']
            self.assertEqual((definition / 'AGENTS.md').read_text(), 'Synthetic source fixture.\n')
            self.assertTrue(os.access(definition / 'bin/check', os.X_OK))
        (self.dest / 'expert/agents/first/AGENTS.md').write_text('Local adaptation')
        self.assertEqual((self.dest / 'expert/agents/second/AGENTS.md').read_text(), 'Synthetic source fixture.\n')
        self.assertFalse((self.root / 'EXPORT_EXECUTED_CODE').exists())
        self.assertFalse((self.dest / 'EXPORT_EXECUTED_CODE').exists())
        self.assertFalse((self.dest / 'worker.lock.json').exists())

    def test_worker_and_team_catalogs_are_separate(self):
        self.make_team()
        self.call('check')
        self.assertEqual(json.loads(self.call('list').stdout)['id'], 'tiny')
        self.assertEqual(json.loads(self.call('list', '--teams').stdout)['id'], 'site')

    def test_team_rejects_retired_member_without_creating_destination(self):
        original = self.make_team()
        self.metadata.update(status='retired', reason='Synthetic retirement')
        self.save_metadata()
        self.export_team(self.commit_source(), ok=False, extra=('--allow-experimental',))
        self.assertFalse(self.dest.exists())
        # Historical pins remain usable and do not imply remote revocation.
        self.export_team(original)

    def test_team_experimental_opt_in_applies_to_members(self):
        self.make_team()
        self.metadata['status'] = 'experimental'
        self.save_metadata()
        commit = self.commit_source()
        self.export_team(commit, ok=False)
        self.export_team(commit, extra=('--allow-experimental',))

    def test_retired_team_rejects_even_active_members(self):
        self.make_team()
        self.team_metadata.update(status='retired', reason='Synthetic retirement')
        self.save_team()
        self.export_team(self.commit_source(), ok=False, extra=('--allow-experimental',))

    def test_team_missing_member_rejects(self):
        self.make_team()
        self.team_metadata['members']['second'] = {'worker': 'absent'}
        self.save_team()
        self.call('check', ok=False)
        self.export_team(self.commit_source(), ok=False)
        self.assertFalse(self.dest.exists())

    def test_team_cannot_hide_bundled_worker_definitions(self):
        self.make_team()
        nested = self.team / 'agents/first/AGENTS.md'
        nested.parent.mkdir(parents=True)
        nested.write_text('Stale bundled worker')
        self.team_metadata['files'].append(nested.relative_to(self.root).as_posix())
        self.save_team()
        self.call('check', ok=False)
        self.export_team(self.commit_source(), ok=False)

    def test_team_worker_undeclared_files_and_archive_changes_reject(self):
        self.make_team()
        (self.expert / 'leftover.txt').write_text('Prior job content')
        self.export_team(self.commit_source(), ok=False)
        (self.expert / 'leftover.txt').unlink()
        (self.root / '.gitattributes').write_text('workers/tiny/expert/README.md export-ignore\n')
        self.export_team(self.commit_source(), ok=False)
        self.assertFalse(self.dest.exists())

    def test_team_roster_path_escape_rejects(self):
        self.make_team()
        for members in [{'../escape': {'worker': 'tiny'}}, {'first': {'worker': '../../tiny'}}, {}, {'first': None}]:
            self.team_metadata['members'] = members
            self.save_team()
            self.call('check', ok=False)
            self.export_team(self.commit_source(), ok=False)

    def test_team_output_does_not_overwrite(self):
        commit = self.make_team()
        self.dest.mkdir()
        (self.dest / 'keep').write_text('Existing work')
        self.export_team(commit, ok=False)
        self.assertEqual((self.dest / 'keep').read_text(), 'Existing work')

    def test_team_root_rejects_unregistered_source(self):
        self.make_team()
        (self.root / 'teams/old-job').mkdir()
        self.call('check', ok=False)

    def test_roster_alone_selects_copied_executable_bindings(self):
        self.make_team()
        adapter = self.team / 'bin/adapter'
        adapter.write_text('#!/bin/sh\nprintf "%s\\n" "${0##*/}"\n')
        adapter.chmod(0o755)
        self.team_metadata['files'].append(adapter.relative_to(self.root).as_posix())
        self.team_metadata['members'] = {'first': {'worker': 'tiny', 'adapter': 'bin/adapter'}}
        self.save_team()
        self.export_team(self.commit_source())
        binding = self.dest / 'expert/bin/workers/first'
        self.assertEqual(binding.read_bytes(), adapter.read_bytes())
        self.assertEqual(subprocess.check_output([str(binding)], text=True).strip(), 'first')
        self.assertFalse((self.dest / 'expert/bin/workers/second').exists())
        self.assertFalse((self.dest / 'expert/agents/second').exists())
        lock = json.loads((self.dest / 'team.lock.json').read_text())
        self.assertEqual(lock['members']['first']['adapter']['source'], 'teams/site/expert/bin/adapter')

    def test_team_adapter_must_be_approved_executable_source(self):
        self.make_team()
        self.team_metadata['members']['first']['adapter'] = '../outside'
        self.save_team()
        self.export_team(self.commit_source(), ok=False)
        self.team_metadata['members']['first']['adapter'] = 'README.md'
        self.save_team()
        self.export_team(self.commit_source(), ok=False)

    def test_export_uses_commit_and_preserves_modes_and_lock(self):
        original = (self.expert / 'AGENTS.md').read_bytes()
        (self.expert / 'AGENTS.md').write_text('UNCOMMITTED CONTENT')
        (self.expert / 'notes.txt').write_text('UNTRACKED CONTENT')
        self.export()
        self.assertEqual((self.dest / 'expert/AGENTS.md').read_bytes(), original)
        self.assertFalse((self.dest / 'expert/notes.txt').exists())
        self.assertTrue(os.access(self.dest / 'expert/bin/check', os.X_OK))
        lock = json.loads((self.dest / 'worker.lock.json').read_text())
        self.assertEqual(lock['source']['commit'], self.commit)
        self.assertEqual(len(lock['files']), 4)
        self.assertFalse((self.dest / '.git').exists())

    def test_current_check_sees_untracked_ignored_files(self):
        self.call('check')
        (self.root / '.gitignore').write_text('leftover.txt\n')
        (self.expert / 'leftover.txt').write_text('Synthetic leftover')
        self.call('check', ok=False)

    def test_unregistered_library_directory_rejected(self):
        (self.root / 'workers/leftover-app').mkdir()
        self.call('check', ok=False)

    def test_committed_undeclared_file_rejected(self):
        (self.expert / 'leftover.txt').write_text('Synthetic leftover')
        self.export(self.commit_source(), ok=False)
        self.assertFalse(self.dest.exists())

    def test_even_declared_content_is_rejected(self):
        (self.expert / 'result.html').write_text('<p>Synthetic forbidden output</p>')
        self.metadata['files'].append('workers/tiny/expert/result.html')
        self.save_metadata()
        self.export(self.commit_source(), ok=False)

    def test_binary_payload_inside_markdown_is_rejected(self):
        (self.expert / 'AGENTS.md').write_text('data:image/png;base64,AAAA')
        self.call('check', ok=False)
        self.export(self.commit_source(), ok=False)

    def test_symlink_rejected_in_worktree_and_commit(self):
        (self.expert / 'README.md').unlink()
        (self.expert / 'README.md').symlink_to('/tmp/unrelated')
        self.call('check', ok=False)
        self.export(self.commit_source(), ok=False)

    def test_no_overwrite(self):
        self.dest.mkdir()
        (self.dest / 'keep').write_text('keep')
        self.export(ok=False)
        self.assertEqual((self.dest / 'keep').read_text(), 'keep')

    def test_no_floating_revision(self):
        self.export('HEAD', ok=False)
        self.export(self.commit[:8], ok=False)

    def test_export_stays_outside_source(self):
        self.dest = self.root / 'installed'
        self.export(ok=False)
        self.assertFalse(self.dest.exists())

    def test_binary_and_local_configuration_are_not_source(self):
        (self.expert / 'AGENTS.md').write_bytes(b'\0binary')
        self.call('check', ok=False)
        self.export(self.commit_source(), ok=False)
        (self.expert / 'AGENTS.md').write_text('Synthetic source fixture.\n')
        (self.expert / '.env.local').write_text('SYNTHETIC_LOCAL=1')
        self.metadata['files'].append('workers/tiny/expert/.env.local')
        self.save_metadata()
        self.export(self.commit_source(), ok=False)

    def test_lifecycle(self):
        for state in ('experimental', 'deprecated', 'retired'):
            self.metadata.update(status=state, reason='Synthetic transition')
            self.save_metadata()
            commit = self.commit_source()
            self.export(commit, ok=False)
            if state == 'experimental':
                self.export(commit, extra=('--allow-experimental',))
                import shutil
                shutil.rmtree(self.dest)
            else:
                self.export(commit, ok=False, extra=('--allow-experimental',))
            self.assertEqual(self.call('list').stdout, '')
            self.assertIn(state, self.call('list', '--all').stdout)

    def test_invalid_metadata(self):
        for field, value in [('files', ['../../escape']), ('files', self.metadata['files'] * 2),
                             ('id', 'other'), ('status', 'unknown'), ('owner', '')]:
            original = self.metadata[field]
            self.metadata[field] = value
            self.save_metadata()
            self.call('check', ok=False)
            self.metadata[field] = original

    def test_missing_license_and_runtime_tree(self):
        self.metadata['files'].remove('workers/tiny/expert/LICENSE')
        self.save_metadata()
        self.call('check', ok=False)
        self.metadata['files'].append('workers/tiny/expert/LICENSE')
        self.save_metadata()
        (self.expert / 'node_modules').mkdir()
        self.call('check', ok=False)

    def test_git_attributes_cannot_omit_declared_files(self):
        (self.root / '.gitattributes').write_text('workers/tiny/expert/README.md export-ignore\n')
        self.export(self.commit_source(), ok=False)

    def test_git_attributes_cannot_substitute_committed_bytes(self):
        (self.expert / 'README.md').write_text('$Format:%H$\n')
        (self.root / '.gitattributes').write_text('workers/tiny/expert/README.md export-subst\n')
        self.export(self.commit_source(), ok=False)

    def test_credentials_not_persisted(self):
        self.git('remote', 'set-url', 'origin', 'https://fixture:synthetic@example.invalid/repo.git')
        result = self.export(ok=False)
        self.assertNotIn('fixture:synthetic', result.stderr)
        self.assertFalse(self.dest.exists())


if __name__ == '__main__':
    unittest.main()
