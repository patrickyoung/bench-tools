"""Exercise each independently shipped publication contract, not the team copy.

Synthetic text tests cover spec discrimination; rendered-media quality is separate.
"""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
ROLES = ('editorial-director', 'information-designer', 'executive-writer',
         'presentation-designer', 'publication-reviewer')


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value))


def content(role):
    ref = {'fact_ids': ['decision'], 'message_ids': ['m0', 'm1']}
    story = dict(title='Conditional choice', subtitle='Synthetic case', audience='Decision makers',
                 thesis='Further evidence is required.',
                 messages=[dict(id=f'm{i}', text='A supported finding', qualification='Within supplied limits',
                                fact_ids=['decision']) for i in range(3)],
                 required_message_ids=['m0', 'm1'], terminology=['Conditional'],
                 design=dict(direction='Clear evidence', font='Arial', ink='#112233', accent='#456789',
                             secondary='#345678', paper='#FFFFFF'),
                 report_arc=['Evidence'] * 5, deck_arc=['Evidence'] * 7,
                 status_label='Conditional — review required', eligibility_labels={})
    if role == 'editorial-director':
        return story
    if role == 'information-designer':
        return dict(title='Comparison', workbook_intro='Qualified results', visuals=[
            dict(id=f'v{i}', kind=kind, title='Evidence', subtitle='Qualified', caption='Read limits',
                 alt='Accessible description', **ref,
                 steps=[dict(heading='Validate', detail='Resolve the gap', **ref)] * 3
                 if kind == 'decision_path' else [])
            for i, kind in enumerate(['score_bounds', 'decision_path', 'coverage'])])
    if role == 'executive-writer':
        # Deliberately repetitive text exercises length/identity, not prose quality.
        return dict(title='Decision report', subtitle='Synthetic', executive_summary='Supported finding. ' * 30,
                    sections=[dict(heading=f'Evidence {i}', paragraphs=['Qualified finding. ' * 60] * 2,
                                   visual_id=None, **ref) for i in range(5)])
    if role == 'presentation-designer':
        return dict(title='Decision', slides=[dict(kind=kind, title='Qualified finding', lead='Next evidence',
                    body=['Keep uncertainty visible'], notes='Synthetic speaker notes', **ref)
                    for kind in ['cover', 'message', 'score_chart', 'comparison_table', 'tradeoff', 'message', 'decision']])
    return dict(verdict='publish', summary='All outputs checked', findings=[],
                rubric={k: 4 for k in ['evidence_fidelity', 'narrative_coherence', 'audience_usefulness',
                                      'writing_quality', 'visual_craft', 'accessibility']},
                cross_format_checks=['Synthetic consistency observation'] * 5)


class PublicationWorkers(unittest.TestCase):
    def case(self, role):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        root = Path(tmp.name).resolve()
        expert = ROOT / 'workers' / role / 'expert'
        spec = importlib.util.spec_from_file_location('contract_' + role.replace('-', '_'),
                                                       expert / 'lib/publication_contract.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        write(root / 'request.json', {'role': role, 'production': 'controller'})
        write(root / 'inputs/source.json', {'facts': [{'id': 'decision'}], 'statistics': {'comparisons': []}})
        if role != 'editorial-director':
            write(root / 'inputs/story.json', {'content': content('editorial-director')})
        if role == 'executive-writer':
            write(root / 'inputs/design.json', {'content': content('information-designer')})
        if role == 'publication-reviewer':
            write(root / 'inputs/visual-review.json', {'verdict': 'pass'})
        write(root / 'output/spec.json', {**module.bindings(root, role), 'content': content(role)})
        return root, expert

    def check(self, root, expert, expected, phrase='', authoring=True):
        env = dict(os.environ)
        env.pop('PUBLICATION_AUTHORING_ONLY', None)
        if authoring:
            env['PUBLICATION_AUTHORING_ONLY'] = '1'
        p = subprocess.run([str(expert / 'bin/check')], cwd=root, env=env, capture_output=True,
                           text=True, timeout=20)
        self.assertEqual(p.returncode, expected, p.stdout + p.stderr)
        self.assertIn(phrase, p.stdout + p.stderr)

    def mutate(self, root, fn):
        p = root / 'output/spec.json'
        data = json.loads(p.read_text())
        fn(data)
        write(p, data)

    def test_each_shipped_worker_accepts_a_valid_spec(self):
        for role in ROLES:
            with self.subTest(role=role):
                self.check(*self.case(role), 0)

    def test_each_worker_rejects_stale_request_and_source(self):
        for role in ROLES:
            for file in ['request.json', 'inputs/source.json']:
                with self.subTest(role=role, file=file):
                    root, expert = self.case(role)
                    self.check(root, expert, 0)
                    with (root / file).open('a') as f:
                        f.write(' ')
                    self.check(root, expert, 1, 'Stale')

    def test_each_worker_rejects_missing_output_and_wrong_role(self):
        for role in ROLES:
            with self.subTest(role=role):
                root, expert = self.case(role)
                self.mutate(root, lambda s: s.update(role='invented'))
                self.check(root, expert, 1)
                (root / 'output/spec.json').unlink()
                self.check(root, expert, 1)

    def test_source_reference_near_miss_in_every_authoring_role(self):
        for role, collection in [('editorial-director', 'messages'), ('information-designer', 'visuals'),
                                 ('executive-writer', 'sections'), ('presentation-designer', 'slides')]:
            with self.subTest(role=role):
                root, expert = self.case(role)
                self.mutate(root, lambda s: s['content'][collection][0].update(fact_ids=['plausible-but-absent']))
                self.check(root, expert, 1, 'Unknown fact')

    def test_production_roles_require_real_artifacts(self):
        for role in ['information-designer', 'executive-writer', 'presentation-designer']:
            with self.subTest(role=role):
                root, expert = self.case(role)
                self.check(root, expert, 0)
                self.check(root, expert, 1, authoring=False)
                # Opt-in is insufficient unless the request assigns rendering to the controller.
                write(root / 'request.json', {'role': role})
                self.mutate(root, lambda s: s.update(request_sha256=hashlib.sha256((root/'request.json').read_bytes()).hexdigest()))
                self.check(root, expert, 1)

    def test_narrative_omission_and_dense_slides(self):
        for role, collection in [('executive-writer', 'sections'), ('presentation-designer', 'slides')]:
            with self.subTest(role=role):
                root, expert = self.case(role)
                self.mutate(root, lambda s: [x.update(message_ids=['m0']) for x in s['content'][collection]])
                self.check(root, expert, 1, 'Required narrative')
        root, expert = self.case('presentation-designer')
        self.mutate(root, lambda s: s['content']['slides'][2].update(body=['word ' * 29]))
        self.check(root, expert, 1, '28 words')

    def test_review_cannot_publish_a_material_defect_but_can_request_revision(self):
        root, expert = self.case('publication-reviewer')
        self.mutate(root, lambda s: s['content'].update(findings=[dict(severity='material', artifact='deck',
                    location='slide 2', issue='Missing uncertainty', correction='Restore uncertainty')]))
        self.check(root, expert, 1, 'quality gate')
        self.mutate(root, lambda s: s['content'].update(verdict='revise'))
        self.check(root, expert, 0)


if __name__ == '__main__':
    unittest.main()
