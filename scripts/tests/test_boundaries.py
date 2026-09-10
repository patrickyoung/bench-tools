"""Mutate isolated fixtures to prove the architecture gate catches regressions."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("boundaries", SCRIPTS / "check-boundaries.py")
boundaries = importlib.util.module_from_spec(spec)
spec.loader.exec_module(boundaries)


class Boundaries(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.runtime = tempfile.TemporaryDirectory(prefix="bench-boundary-parser-")
        cls.helper = Path(cls.runtime.name) / "parse-go"
        subprocess.run(["go", "build", "-o", str(cls.helper), str(boundaries.HELPER)],
                       env=boundaries.GO_ENV, check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.runtime.cleanup()

    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory(prefix="bench-boundary-test-")
        self.addCleanup(self.scratch.cleanup)
        self.root = Path(self.scratch.name)
        self.components = []
        for name in ("alpha", "beta"):
            leaf = self.root / "tools" / name
            leaf.mkdir(parents=True)
            module = f"example.test/{name}"
            (leaf / "go.mod").write_text(f"module {module}\n\ngo 1.26\n")
            (leaf / "main.go").write_text('package main\nimport "fmt"\nfunc main() { fmt.Println("ok") }\n')
            self.components.append({"name": name, "path": f"tools/{name}",
                                    "module": module,
                                    "commands": [{"name": name, "package": "."}]})
        self.save_manifest()
        self.write("docs/review/inventory.json", json.dumps([
            {"name": "excluded-app", "module": "module example.test/excluded-app"}
        ]))

    def save_manifest(self):
        (self.root / "components.json").write_text(json.dumps({"schema": 1, "components": self.components}))

    def write(self, path, data):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(data)
        return target

    def check(self, baseline=False):
        return boundaries.check(self.root, baseline=baseline, helper=self.helper)

    def refuses(self, fragment):
        errors = self.check()
        self.assertTrue(any(fragment in error for error in errors), errors)

    def test_independent_tools_and_own_internal_imports_pass(self):
        self.write("tools/alpha/internal/local/local.go", 'package local\nimport "strings"\n')
        self.write("tools/alpha/main.go", 'package main\nimport _ "example.test/alpha/internal/local"\n')
        self.assertEqual(self.check(), [])

    def test_comments_and_strings_are_not_import_edges(self):
        self.write("tools/alpha/main.go", '''package main
// import "example.test/beta"
var help = `import "example.test/beta"`
func main() {}
''')
        self.assertEqual(self.check(), [])

    def test_cross_tool_import_in_untracked_platform_file_is_refused(self):
        self.write("tools/alpha/backend_windows.go", '''//go:build windows

package main
import (
  _ "example.test/beta/internal/data"
)
''')
        self.refuses("cross-tool Go import from beta")

    def test_raw_quoted_and_escaped_imports_cannot_bypass_parser(self):
        for imported in ('`example.test/beta`', '"example.test/\\u0062eta"'):
            with self.subTest(imported=imported):
                self.write("tools/alpha/hidden.go", f"package main\nimport _ {imported}\n")
                self.refuses("cross-tool Go import from beta")

    def test_cross_tool_test_import_is_also_refused(self):
        self.write("tools/alpha/coupled_test.go", 'package main\nimport _ "example.test/beta"\n')
        self.refuses("cross-tool Go import from beta")

    def test_unused_sibling_requirement_is_refused(self):
        self.write("tools/alpha/go.mod", "module example.test/alpha\ngo 1.26\nrequire example.test/beta v1.0.0\n")
        self.refuses("cross-tool module requirement on beta")

    def test_excluded_reviewed_application_is_still_a_tool_boundary(self):
        self.write("tools/alpha/adapter.go", 'package main\nimport _ "example.test/excluded-app/client"\n')
        self.refuses("cross-tool Go import from excluded-app")
        self.write("tools/alpha/go.mod", "module example.test/alpha\ngo 1.26\nrequire example.test/excluded-app v1.0.0\n")
        self.refuses("cross-tool module requirement on excluded-app")

    def test_reviewed_family_inventory_cannot_silently_disappear(self):
        (self.root / "docs/review/inventory.json").unlink()
        self.refuses("cannot establish the reviewed tool family")

    def test_local_replacement_is_refused_even_for_external_module(self):
        self.write("tools/alpha/go.mod", "module example.test/alpha\ngo 1.26\nreplace example.test/library => ../beta\n")
        self.refuses("local module replacement")

    def test_remote_replacement_cannot_alias_sibling_code(self):
        self.write("tools/alpha/go.mod", "module example.test/alpha\ngo 1.26\nreplace example.test/library => example.test/beta v1.0.0\n")
        self.refuses("cross-tool module replacement involving beta")

    def test_root_module_workspace_and_nested_modules_are_refused(self):
        for path, text, expected in (
            ("go.mod", "module example.test/suite\n", "module outside a declared component root"),
            ("go.work", "go 1.26\nuse ./tools/alpha\n", "Go workspaces are forbidden"),
            ("tools/alpha/go.work", "go 1.26\nuse .\n", "Go workspaces are forbidden"),
            ("tools/alpha/nested/go.mod", "module example.test/nested\n", "module outside a declared component root"),
        ):
            with self.subTest(path=path):
                target = self.write(path, text)
                self.refuses(expected)
                target.unlink()

    def test_cross_tool_instruction_and_directory_symlinks_are_refused(self):
        self.write("tools/beta/AGENTS.md", "Beta instructions\n")
        for path, target in (("tools/alpha/AGENTS.md", "../beta/AGENTS.md"),
                             ("tools/alpha/library", "../beta")):
            with self.subTest(path=path):
                link = self.root / path
                link.symlink_to(target)
                self.refuses("symlink escapes component alpha")
                link.unlink()

    def test_dangling_and_external_links_fail_without_reading_target(self):
        link = self.root / "tools/alpha/link"
        link.symlink_to("missing")
        self.refuses("unresolved symlink")
        link.unlink()
        link.symlink_to(self.root / "components.json")
        self.refuses("symlink escapes component alpha")

    def test_same_tool_symlink_is_allowed(self):
        self.write("tools/alpha/AGENTS.md", "Alpha instructions\n")
        (self.root / "tools/alpha/CLAUDE.md").symlink_to("AGENTS.md")
        self.assertEqual(self.check(), [])

    def test_nested_git_markers_are_refused(self):
        marker = self.write("tools/alpha/.git", "gitdir: elsewhere\n")
        self.refuses("nested Git marker")
        marker.unlink()
        marker.mkdir()
        self.refuses("nested Git marker")

    def test_undeclared_tool_or_shared_go_source_is_refused(self):
        self.write("tools/common/common.go", "package common\n")
        self.refuses("unlisted component or shared source")
        self.write("internal/common/common.go", "package common\n")
        self.refuses("shared runtime is forbidden")

    def test_manifest_rejects_colliding_commands_and_wrong_module(self):
        self.components[1]["commands"][0]["name"] = "alpha"
        self.save_manifest()
        self.refuses("command collision for alpha")
        self.components[1]["commands"][0]["name"] = "beta"
        self.components[1]["module"] = "example.test/not-beta"
        self.save_manifest()
        self.refuses("module path differs")

    def test_missing_or_wrong_main_package_and_omitted_cmd_are_refused(self):
        self.components[0]["commands"][0]["package"] = "./cmd/alpha"
        self.save_manifest()
        self.refuses("main package directory is missing")
        self.write("tools/alpha/cmd/alpha/main.go", "package library\n")
        self.refuses("does not name an actual main package")
        self.write("tools/alpha/cmd/alpha/main.go", "package main\n")
        self.write("tools/alpha/cmd/legacy/main.go", "package main\n")
        self.refuses("command main package missing from manifest")
        self.components[0]["commands"].append({"name": "alpha-legacy", "package": "./cmd/legacy"})
        self.save_manifest()
        self.assertEqual(self.check(), [])

    def test_examples_need_not_be_installed_commands(self):
        self.write("tools/alpha/examples/hello/main.go", "package main\n")
        self.assertEqual(self.check(), [])

    def test_command_paths_cannot_escape(self):
        self.components[0]["commands"][0]["package"] = "../beta"
        self.save_manifest()
        self.refuses("expected one local package path")

    def test_shell_entry_must_be_executable_and_local(self):
        leaf = self.root / "tools/alpha"
        (leaf / "go.mod").unlink()
        (leaf / "main.go").unlink()
        self.components[0]["module"] = None
        self.components[0]["commands"] = [{"name": "alpha", "entry": "bin/alpha"}]
        self.save_manifest()
        entry = self.write("tools/alpha/bin/alpha", "#!/bin/sh\nexit 0\n")
        self.refuses("entry must be an executable file")
        entry.chmod(0o755)
        self.assertEqual(self.check(), [])

    def prepare_baseline(self):
        env = dict(boundaries.GO_ENV, GIT_AUTHOR_NAME="Boundary test",
                   GIT_AUTHOR_EMAIL="test@example.invalid", GIT_COMMITTER_NAME="Boundary test",
                   GIT_COMMITTER_EMAIL="test@example.invalid")

        def git(*args, input=None):
            return subprocess.run(["git", *args], cwd=self.root, env=env, input=input,
                                  text=True, capture_output=True, check=True).stdout.strip()

        git("init", "-q")
        for component in self.components:
            entries = []
            for path in sorted((self.root / component["path"]).iterdir()):
                oid = git("hash-object", "-w", str(path))
                entries.append(f"100644 blob {oid}\t{path.name}\n")
            tree = git("mktree", input="".join(entries))
            commit = git("commit-tree", tree, input="Original tool source\n")
            component["source"] = {"tree": tree, "commit": commit}
        self.save_manifest()

    def test_baseline_is_optional_and_detects_content_mode_and_untracked_files(self):
        self.prepare_baseline()
        self.assertEqual(self.check(baseline=True), [])
        main = self.write("tools/alpha/main.go", "package main\nfunc main() {}\n")
        self.assertEqual(self.check(), [], "legitimate source edits must remain possible")
        self.assertTrue(any("content or executable mode differs" in e for e in self.check(baseline=True)))
        untouched = self.root / "tools/beta/main.go"
        untouched.chmod(0o755)
        self.assertTrue(any("beta/main.go: content or executable mode differs" in e
                            for e in self.check(baseline=True)), "mode alone must be checked")
        untouched.chmod(0o644)
        extra = self.write("tools/beta/untracked.txt", "Not in the source tree\n")
        errors = self.check(baseline=True)
        self.assertTrue(any("extra file outside source baseline" in e for e in errors), errors)
        extra.unlink()
        (self.root / "tools/beta/main.go").unlink()
        self.assertTrue(any("missing from source baseline" in e for e in self.check(baseline=True)))


if __name__ == "__main__":
    unittest.main()
