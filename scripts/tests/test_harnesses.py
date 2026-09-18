"""Fresh package failures must be detected before optional native host checks."""
import json
from pathlib import Path
import runpy
import shutil
import tempfile
import unittest
import zipfile


ROOT = Path(__file__).resolve().parents[2]
CHECK = runpy.run_path(str(ROOT / "scripts/check-harnesses.py"))


class HarnessPackages(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="bench-harness-test-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / "source with spaces"
        for name in (*CHECK["PACKAGE_FILES"], *CHECK["MARKETPLACE_FILES"], *CHECK["PLUGIN_FILES"]):
            destination = self.root / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, destination)
        shutil.copytree(ROOT / ".agents/skills", self.root / ".agents/skills")

    def change(self, name, mutation):
        path = self.root / name
        value = json.loads(path.read_text())
        mutation(value)
        path.write_text(json.dumps(value))

    def test_current_packages_and_complete_relocated_zip(self):
        CHECK["package_metadata"](self.root)
        archive, relocated = self.base / "plugin.zip", self.base / "relocated package"
        CHECK["portable_package"](self.root, archive, relocated)
        expected = {p.relative_to(self.root): p.read_bytes()
                    for p in (self.root / ".agents/skills").rglob("*") if p.is_file()}
        for relative, data in expected.items():
            self.assertEqual((relocated / relative).read_bytes(), data)
        with zipfile.ZipFile(archive) as package:
            self.assertEqual(set(package.namelist()),
                             {*CHECK["PACKAGE_FILES"], *(p.as_posix() for p in expected)})

    def test_mismatched_release_is_rejected(self):
        self.change("package.json", lambda value: value.update(version="0.0.0"))
        with self.assertRaisesRegex(RuntimeError, "versions disagree"):
            CHECK["package_metadata"](self.root)

    def test_marketplace_cannot_point_outside_relocated_package(self):
        for name in CHECK["MARKETPLACE_FILES"][:2]:
            original = (self.root / name).read_text()
            with self.subTest(name=name):
                self.change(name, lambda value: value["plugins"][0].update(source="../another-plugin"))
                with self.assertRaisesRegex(RuntimeError, "repository root plugin"):
                    CHECK["package_metadata"](self.root)
            (self.root / name).write_text(original)

    def test_absolute_skill_path_cannot_pass_before_relocation(self):
        self.change(".codex-plugin/plugin.json",
                    lambda value: value.update(skills=str(self.root / ".agents/skills")))
        with self.assertRaisesRegex(RuntimeError, "relative and relocatable"):
            CHECK["package_metadata"](self.root)

    def test_linked_reference_cannot_leak_into_uploaded_zip(self):
        outside = self.base / "private.txt"
        outside.write_text("not part of the reusable skill")
        (self.root / ".agents/skills/bench/references/private.txt").symlink_to(outside)
        with self.assertRaisesRegex(RuntimeError, "symbolic link"):
            CHECK["portable_package"](self.root, self.base / "plugin.zip", self.base / "extracted")
        self.assertFalse((self.base / "plugin.zip").exists())

    def test_linked_ancestor_cannot_supply_unbundled_skills(self):
        shutil.move(self.root / ".agents", self.base / "external-agents")
        (self.root / ".agents").symlink_to(self.base / "external-agents", target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, "linked skill directory"):
            CHECK["skill_files"](self.root)


if __name__ == "__main__":
    unittest.main()
