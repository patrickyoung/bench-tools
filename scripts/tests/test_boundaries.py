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

    def register_worker_helper(self):
        paths = ["workers/painter/expert/bitmap/go.mod",
                 "workers/painter/expert/bitmap/main.go"]
        self.write("workers/painter/worker.json", json.dumps({"id": "painter", "files": paths}))
        self.write(paths[0], "module bench.local/painter\ngo 1.22\n")
        self.write(paths[1], 'package main\nimport "image/png"\n')
        return paths

    def test_registered_worker_standard_library_helpers_and_tests_pass(self):
        self.register_worker_helper()
        self.write("workers/painter/tests/go.mod", "module bench.local/painter-tests\ngo 1.22\n")
        self.write("workers/painter/tests/contracts_test.go", 'package tests\nimport "testing"\n')
        self.assertEqual(self.check(), [])

    def test_worker_helpers_cannot_import_tools_or_other_workers(self):
        paths = self.register_worker_helper()
        for imported in ("example.test/alpha", "bench.local/other-worker", "./local", "C"):
            with self.subTest(imported=imported):
                self.write(paths[1], f'package main\nimport _ "{imported}"\n')
                self.refuses("worker helpers may import only the standard library")

    def test_worker_modules_cannot_require_or_replace_dependencies(self):
        paths = self.register_worker_helper()
        for directive in ("require example.test/alpha v1.0.0", "replace example.test/x => ../../../../tools/alpha"):
            with self.subTest(directive=directive):
                self.write(paths[0], "module bench.local/painter\ngo 1.22\n" + directive + "\n")
                self.refuses("worker helper modules must use only the standard library")

    def test_worker_export_inventory_and_local_module_are_required(self):
        paths = self.register_worker_helper()
        extra = self.write("workers/painter/expert/bitmap/unlisted.go", "package main\n")
        self.refuses("shared runtime is forbidden")
        extra.unlink()
        (self.root / paths[0]).unlink()
        self.refuses("shared runtime is forbidden")

    def test_tools_cannot_import_or_require_worker_helpers(self):
        self.register_worker_helper()
        self.write("tools/alpha/main.go", 'package main\nimport _ "bench.local/painter"\n')
        self.refuses("cross-tool Go import from worker painter")
        self.write("tools/alpha/go.mod", "module example.test/alpha\ngo 1.26\nrequire bench.local/painter v1.0.0\n")
        self.refuses("cross-tool module requirement on worker painter")

    def test_worker_module_identity_cannot_shadow_a_tool(self):
        paths = self.register_worker_helper()
        self.write(paths[0], "module example.test/alpha\ngo 1.22\n")
        self.refuses("worker module collides with tool module")

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

    def add_interface(self, name="alpha"):
        component = {"name": name, "path": f"interfaces/{name}",
                     "module": f"example.test/{name}-ui",
                     "commands": [{"name": f"{name}-ui", "package": "."}]}
        self.write(f"interfaces/{name}/go.mod", f"module {component['module']}\n\ngo 1.26\n")
        self.write(f"interfaces/{name}/main.go", "package main\nfunc main() {}\n")
        self.save_interfaces([component])
        return component

    def save_interfaces(self, components):
        self.write("interfaces/manifest.json", json.dumps({"schema": 1, "interfaces": components}))

    def test_interfaces_have_independent_namespace_and_own_internal_imports(self):
        self.add_interface()
        self.write("interfaces/README.md", "Independent applications\n")
        self.write("interfaces/AGENTS.md", "Preserve boundaries\n")
        self.write("interfaces/alpha/internal/view/view.go", "package view\n")
        self.write("interfaces/alpha/main.go", 'package main\nimport _ "example.test/alpha-ui/internal/view"\nfunc main() {}\n')
        self.assertEqual(self.check(), [])

    def test_cross_interface_and_tool_imports_are_refused_both_ways(self):
        alpha = self.add_interface()
        beta = self.add_interface("beta")
        self.save_interfaces([alpha, beta])
        for path, module, expected in (
            ("interfaces/alpha/coupled_test.go", "example.test/alpha", "from alpha"),
            ("tools/alpha/coupled_test.go", "example.test/alpha-ui", "from interfaces/alpha"),
            ("interfaces/beta/coupled_windows.go", "example.test/alpha-ui/internal/view", "from interfaces/alpha"),
        ):
            with self.subTest(path=path):
                file = self.write(path, f'package main\nimport _ "{module}"\n')
                self.refuses("cross-tool Go import " + expected)
                file.unlink()

    def test_interface_requirements_and_replacements_share_tool_boundaries(self):
        self.add_interface()
        for path, module, peer, expected in (
            ("interfaces/alpha/go.mod", "example.test/alpha-ui", "example.test/alpha", "alpha"),
            ("tools/alpha/go.mod", "example.test/alpha", "example.test/alpha-ui", "interfaces/alpha"),
        ):
            for directive, fragment in (
                (f"require {peer} v1.0.0", "cross-tool module requirement on " + expected),
                (f"replace example.test/library => {peer} v1.0.0", "cross-tool module replacement involving " + expected),
                ("replace example.test/library => ../local", "local module replacement"),
            ):
                with self.subTest(path=path, directive=directive):
                    self.write(path, f"module {module}\ngo 1.26\n{directive}\n")
                    self.refuses(fragment)
            self.write(path, f"module {module}\ngo 1.26\n")

    def test_interface_manifest_cannot_bypass_roots_modules_or_command_uniqueness(self):
        interface = self.add_interface()
        for key, value, expected in (
            ("path", "tools/alpha", "own interfaces/alpha directory"),
            ("module", "example.test/alpha", "duplicate module path"),
            ("commands", [{"name": "alpha", "package": "."}], "command collision for alpha"),
            ("commands", [{"name": "alpha-ui", "package": "../../tools/alpha"}], "expected one local package path"),
        ):
            with self.subTest(key=key):
                self.save_interfaces([dict(interface, **{key: value})])
                self.refuses(expected)
        self.save_interfaces([interface, interface])
        self.refuses("duplicate component name")

    def test_interface_modules_and_main_packages_are_checked(self):
        self.add_interface()
        self.write("interfaces/alpha/go.mod", "module example.test/wrong\ngo 1.26\n")
        self.refuses("module path differs")
        self.write("interfaces/alpha/go.mod", "module example.test/alpha-ui\ngo 1.26\n")
        self.write("interfaces/alpha/main.go", "package library\n")
        self.refuses("does not name an actual main package")
        self.write("interfaces/alpha/main.go", "package main\n")
        undeclared = self.write("interfaces/alpha/cmd/hidden/main.go", "package main\n")
        self.refuses("command main package missing from manifest")
        undeclared.unlink()
        self.write("interfaces/alpha/nested/go.mod", "module example.test/nested\n")
        self.refuses("module outside a declared component root")

    def test_interface_symlinks_cannot_cross_tool_or_interface_boundaries(self):
        self.add_interface()
        for path, target, expected in (
            ("interfaces/alpha/library", "../../tools/alpha", "interfaces/alpha"),
            ("tools/alpha/library", "../../interfaces/alpha", "alpha"),
        ):
            with self.subTest(path=path):
                link = self.root / path
                link.symlink_to(target)
                self.refuses("symlink escapes component " + expected)
                link.unlink()

    def test_interface_manifest_is_required_and_cannot_be_symlinked(self):
        self.add_interface()
        manifest = self.root / "interfaces/manifest.json"
        manifest.unlink()
        self.refuses("interfaces/manifest.json")
        outside = self.write("other-manifest.json", '{"schema":1,"interfaces":[]}')
        manifest.symlink_to(outside)
        self.refuses("manifest must not be a symlink")

    def test_interface_parent_shared_source_and_unlisted_leaves_are_refused(self):
        self.add_interface()
        self.write("interfaces/shared.go", "package shared\n")
        self.refuses("unlisted component or shared source")
        self.write("interfaces/hidden/go.mod", "module example.test/hidden\n")
        self.refuses("module outside a declared component root")

    def test_interface_parent_and_manifest_shape_are_validated(self):
        self.add_interface()
        for manifest in ({"schema": 2, "interfaces": []},
                         {"schema": 1, "interfaces": {}},
                         {"schema": 1, "interfaces": ["not an object"]}):
            with self.subTest(manifest=manifest):
                self.write("interfaces/manifest.json", json.dumps(manifest))
                self.refuses("interfaces/manifest.json")
        parent = self.root / "interfaces"
        moved = self.root / "moved-interfaces"
        parent.rename(moved)
        parent.symlink_to(moved, target_is_directory=True)
        self.refuses("interfaces: component parent must not be a symlink")

    def test_interface_source_does_not_change_original_tool_provenance(self):
        self.add_interface()
        self.prepare_baseline()
        self.assertEqual(self.check(baseline=True), [])
        self.write("interfaces/alpha/main.go", 'package main\nfunc main() { println("new UI") }\n')
        self.assertEqual(self.check(baseline=True), [])

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
