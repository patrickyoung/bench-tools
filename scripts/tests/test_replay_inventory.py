"""Catch missing command coverage before the expensive executable replay suite."""
import json
from pathlib import Path
import runpy
import unittest


class ReplayInventory(unittest.TestCase):
    def test_every_registered_command_has_a_replay_boundary(self):
        root = Path(__file__).resolve().parents[2]
        components = json.loads((root / "components.json").read_text())
        commands = {command["name"] for component in components["components"]
                    for command in component["commands"]}
        boundaries = runpy.run_path(str(root / "scripts/check-replay-inventory.py"))["BOUNDARIES"]
        self.assertEqual(commands, set(boundaries))


if __name__ == "__main__":
    unittest.main()
