"""The extracted runner retains the existing executable acceptance contracts."""
import importlib.util
from pathlib import Path
import shutil
from unittest.mock import patch

import test_process as fixtures

RUNNER = fixtures.APP.with_name('runner.py')
spec = importlib.util.spec_from_file_location('team_runner',RUNNER)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class ExtractedRunnerTests(fixtures.TendIntegrationTests):
    def setUp(self):
        replacement = patch.object(fixtures,'p',runner)
        replacement.start(); self.addCleanup(replacement.stop)
        super().setUp()

    def test_presentation_changes_do_not_change_execution_identity(self):
        deployed = self.base/'application'; deployed.mkdir()
        copied = deployed/'runner.py'; shutil.copy2(RUNNER,copied)
        viewer = deployed/'viewer.html'; viewer.write_text('<p>Original presentation</p>')
        module_spec = importlib.util.spec_from_file_location('deployed_runner',copied)
        selected = importlib.util.module_from_spec(module_spec); module_spec.loader.exec_module(selected)
        instance = selected.admit(self.export,self.request,self.root,fixtures.TEND)
        binding = selected.load(instance/'admission.json')
        self.assertEqual(binding['application']['path'],str(copied))
        viewer.write_text('<p>New calendar layout and board colors</p>')
        selected.submit(instance); self.tend('work')
        self.assertEqual(selected.status(instance,'2035-01-01T00:00:00Z')['acceptance'],'accepted')
