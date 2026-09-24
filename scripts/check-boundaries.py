#!/usr/bin/env python3
"""Check independent source boundaries; optionally prove the original import.

Reads the current filesystem, including untracked and ignored source files.
Uses Go's parser for imports on every platform, not the current GOOS package set.
This is an architectural guard, not a sandbox or a proof of dynamic behavior.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import stat
import subprocess
import sys


HERE = Path(__file__).resolve().parent
HELPER = HERE / "go-boundaries" / "main.go"
NAME = re.compile(r"[a-z][a-z0-9-]*\Z")
GO_ENV = dict(os.environ, GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", GOFLAGS="")


def inside(path: Path, root: Path) -> bool:
    return path == root or root in path.parents


def run(argv: list[str], *, cwd: Path, input: str | None = None) -> str:
    result = subprocess.run(argv, cwd=cwd, env=GO_ENV, input=input,
                            text=True, capture_output=True)
    if result.returncode:
        raise ValueError(f"{' '.join(argv[:3])}: {result.stderr.strip() or result.stdout.strip()}")
    return result.stdout


def module_owner(value: str, modules: dict[str, str]) -> str | None:
    # Longest prefix permits distinct /v2 modules without an accidental match.
    for module in sorted(modules, key=len, reverse=True):
        if value == module or value.startswith(module + "/"):
            return modules[module]
    return None


def safe_relative(value: object) -> bool:
    if not isinstance(value, str) or not value or "\\" in value:
        return False
    path = PurePosixPath(value)
    return not path.is_absolute() and ".." not in path.parts


def inventory(root: Path) -> list[Path]:
    """Do not follow symlink directories, or skip ignored/untracked files."""
    result = []
    for current, dirs, files in os.walk(root, followlinks=False):
        directory = Path(current)
        if directory == root and ".git" in dirs:
            dirs.remove(".git")
        for name in list(dirs):
            path = directory / name
            if path.is_symlink() or name == ".git":
                result.append(path)
                dirs.remove(name)
        for name in files:
            path = directory / name
            if path != root / ".git":
                result.append(path)
    return sorted(result)


def parse_go(paths: list[Path], root: Path, helper: Path | None) -> list[dict]:
    if not paths:
        return []
    if helper is None:
        argv = ["go", "run", str(HELPER)]
    else:
        argv = [str(helper)]
    return json.loads(run(argv, cwd=root, input=json.dumps([str(p) for p in paths])))


def worker_go_sources(root: Path, files: list[Path], errors: list[str]) -> tuple[dict[Path, Path], dict[str, str]]:
    """Admit private, standard-library-only worker helpers, never shared code.

    Exported source must be inventoried by that worker; synthetic test source
    stays under its tests/. Each file needs an admitted module in the same
    expert or tests subtree. The library check owns the remaining metadata.
    """
    admitted = {}
    identities = {}
    for metadata in (root / "workers").glob("*/worker.json"):
        leaf = metadata.parent
        if metadata.is_symlink() or leaf.is_symlink():
            continue
        try:
            worker = json.loads(metadata.read_text())
            if worker.get("id") != leaf.name or not NAME.fullmatch(leaf.name):
                continue
            approved = worker.get("files", [])
            if not isinstance(approved, list) or not all(safe_relative(p) for p in approved):
                continue
        except (OSError, ValueError, TypeError, AttributeError):
            continue
        candidates = []
        for path in files:
            if path.is_symlink() or not inside(path, leaf):
                continue
            if path.suffix != ".go" and path.name != "go.mod":
                continue
            relative = path.relative_to(root).as_posix()
            if ((inside(path, leaf / "expert") and relative in approved)
                    or inside(path, leaf / "tests")):
                candidates.append(path)
        modules = {p for p in candidates if p.name == "go.mod"}
        for modfile in modules:
            try:
                model = json.loads(run(["go", "mod", "edit", "-json", str(modfile)], cwd=root))
                if not model.get("Module", {}).get("Path"):
                    raise ValueError("missing module declaration")
                identity = model["Module"]["Path"]
                if identity in identities:
                    errors.append(f"{modfile.relative_to(root)}: duplicate worker module identity")
                identities[identity] = f"worker {leaf.name}"
                if any(model.get(key) for key in ("Require", "Replace", "Exclude", "Tool")):
                    errors.append(f"{modfile.relative_to(root)}: worker helper modules must use only the standard library")
            except (OSError, ValueError) as exc:
                errors.append(f"{modfile.relative_to(root)}: invalid worker helper module: {exc}")
        for path in candidates:
            scope = leaf / ("expert" if inside(path, leaf / "expert") else "tests")
            if any(inside(modfile, scope) and inside(path, modfile.parent) for modfile in modules):
                admitted[path] = scope
    return admitted, identities


def baseline_errors(root: Path, components: list[dict], files: list[Path]) -> list[str]:
    errors = []
    for component in components:
        name, relative = component["name"], component["path"]
        leaf = root / relative
        source = component.get("source", {})
        tree, commit = source.get("tree"), source.get("commit")
        if not all(isinstance(v, str) and re.fullmatch(r"[0-9a-f]{40}", v)
                   for v in (tree, commit)):
            errors.append(f"{name}: missing original source commit/tree for baseline")
            continue
        try:
            observed = run(["git", "rev-parse", f"{commit}^{{tree}}"], cwd=root).strip()
            if observed != tree:
                errors.append(f"{name}: source commit/tree provenance disagrees")
                continue
            raw = run(["git", "ls-tree", "-r", "-z", tree], cwd=root)
        except ValueError as exc:
            errors.append(f"{name}: cannot inspect baseline: {exc}")
            continue
        expected = {}
        for record in raw.split("\0"):
            if record:
                metadata, path = record.split("\t", 1)
                mode, kind, oid = metadata.split()
                if kind != "blob":
                    errors.append(f"{name}/{path}: unsupported source object {kind}")
                expected[path] = (mode, oid)
        actual = {}
        for path in files:
            if not inside(path, leaf):
                continue
            key = path.relative_to(leaf).as_posix()
            info = path.lstat()
            if stat.S_ISLNK(info.st_mode):
                data, mode = os.fsencode(os.readlink(path)), "120000"
            elif stat.S_ISREG(info.st_mode):
                data = path.read_bytes()
                mode = "100755" if info.st_mode & 0o111 else "100644"
            else:
                errors.append(f"{name}/{key}: unsupported baseline file type")
                continue
            oid = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
            actual[key] = (mode, oid)
        for path in sorted(expected.keys() - actual.keys()):
            errors.append(f"{name}/{path}: missing from source baseline")
        for path in sorted(actual.keys() - expected.keys()):
            errors.append(f"{name}/{path}: extra file outside source baseline")
        for path in sorted(expected.keys() & actual.keys()):
            if expected[path] != actual[path]:
                errors.append(f"{name}/{path}: content or executable mode differs from source baseline")
    return errors


def check(root: Path, *, baseline: bool = False, helper: Path | None = None) -> list[str]:
    root = root.resolve()
    errors: list[str] = []
    try:
        manifest = json.loads((root / "components.json").read_text())
        components = manifest["components"]
        if manifest.get("schema") != 1 or not isinstance(components, list) or not components:
            raise ValueError("expected schema 1 and a nonempty components list")
    except (OSError, ValueError, KeyError) as exc:
        return [f"components.json: {exc}"]

    names, modules, commands = set(), {}, {}
    leaves: dict[str, Path] = {}
    for component in components:
        if not isinstance(component, dict):
            errors.append("components.json: each component must be an object")
            continue
        name, relative, module = component.get("name"), component.get("path"), component.get("module")
        if not isinstance(name, str) or not NAME.fullmatch(name) or name in names:
            errors.append(f"components.json: invalid or duplicate component name {name!r}")
            continue
        names.add(name)
        if relative != f"tools/{name}":
            errors.append(f"{name}: component must have its own tools/{name} directory")
            continue
        leaf = root / relative
        leaves[name] = leaf
        if leaf.is_symlink() or not leaf.is_dir():
            errors.append(f"{name}: missing or symlinked component directory")
        if module is not None:
            if not isinstance(module, str) or not module or module in modules:
                errors.append(f"{name}: invalid or duplicate module path")
            else:
                modules[module] = name
        entries = component.get("commands")
        if not isinstance(entries, list) or not entries:
            errors.append(f"{name}: commands must be a nonempty list")
            continue
        for command in entries:
            if not isinstance(command, dict):
                errors.append(f"{name}: command must be an object")
                continue
            binary = command.get("name")
            if not isinstance(binary, str) or not NAME.fullmatch(binary):
                errors.append(f"{name}: invalid command name {binary!r}")
                continue
            if binary in commands:
                errors.append(f"{name}: command collision for {binary} with {commands[binary]}")
            commands[binary] = name
            key = "package" if module is not None else "entry"
            other = "entry" if module is not None else "package"
            target = command.get(key)
            if other in command or not safe_relative(target):
                errors.append(f"{name}/{binary}: expected one local {key} path")
                continue
            target_path = leaf / target
            if not inside(target_path.resolve(), leaf.resolve()):
                errors.append(f"{name}/{binary}: command escapes its component")
            elif key == "entry":
                if not target_path.is_file() or not target_path.stat().st_mode & 0o111:
                    errors.append(f"{name}/{binary}: entry must be an executable file")
            elif not target_path.is_dir():
                errors.append(f"{name}/{binary}: main package directory is missing")
    # Avoid continuing with malformed manifest paths or keys.
    if errors:
        return errors

    # The family includes reviewed applications that were deliberately excluded
    # from this import. Depending on those modules would still couple tools.
    family_file = root / "docs" / "review" / "inventory.json"
    family_modules = dict(modules)
    try:
        family = json.loads(family_file.read_text())
        if not isinstance(family, list):
            raise ValueError("expected the reviewed repository list")
        for repository in family:
            declaration = repository.get("module")
            if declaration is None:
                continue
            fields = shlex.split(declaration)
            if len(fields) != 2 or fields[0] != "module":
                raise ValueError(f"invalid recorded module for {repository.get('name')}")
            family_modules.setdefault(fields[1], repository["name"])
    except (OSError, ValueError, KeyError, AttributeError) as exc:
        return [f"docs/review/inventory.json: cannot establish the reviewed tool family: {exc}"]

    def owner(path: Path) -> str | None:
        for name, leaf in leaves.items():
            if inside(path, leaf):
                return name
        return None

    tools_dir = root / "tools"
    if tools_dir.is_symlink():
        errors.append("tools: component parent must not be a symlink")
    if tools_dir.is_dir():
        for child in tools_dir.iterdir():
            if child.name not in names:
                errors.append(f"{child.relative_to(root)}: unlisted component or shared source")
    files = inventory(root)
    worker_go, worker_modules = worker_go_sources(root, files, errors)
    for module, worker in worker_modules.items():
        if module in family_modules:
            errors.append(f"{worker}: worker module collides with tool module {module}")
        else:
            family_modules[module] = worker
    go_paths = []
    for path in files:
        relative, component = path.relative_to(root).as_posix(), owner(path)
        if path.name == ".git":
            errors.append(f"{relative}: nested Git marker changes repository instruction scope")
        if path.name == "go.work" or path.name == "go.work.sum":
            errors.append(f"{relative}: Go workspaces are forbidden")
        if path.name == "go.mod" and path not in worker_go and (component is None or path != leaves[component] / "go.mod"):
            errors.append(f"{relative}: module outside a declared component root")
        if path.is_symlink():
            try:
                target = path.resolve(strict=True)
            except (OSError, RuntimeError) as exc:
                errors.append(f"{relative}: unresolved symlink: {exc}")
                continue
            if component is not None and not inside(target, leaves[component].resolve()):
                errors.append(f"{relative}: symlink escapes component {component}")
            elif component is None and (not inside(target, root) or owner(target) is not None):
                errors.append(f"{relative}: root symlink crosses a component/external boundary")
        if path.suffix == ".go":
            if component is None and path not in worker_go and relative != "scripts/go-boundaries/main.go":
                errors.append(f"{relative}: Go source outside a component (shared runtime is forbidden)")
            elif component is not None or path in worker_go:
                go_paths.append(path)

    for component in components:
        name, module = component["name"], component["module"]
        leaf = leaves[name]
        modfile = leaf / "go.mod"
        if module is None:
            if modfile.exists():
                errors.append(f"{name}: undeclared Go module")
            continue
        if not modfile.is_file():
            errors.append(f"{name}: missing standalone go.mod")
            continue
        try:
            metadata = json.loads(run(["go", "mod", "edit", "-json", str(modfile)], cwd=root))
        except (ValueError, OSError) as exc:
            errors.append(f"{name}: cannot parse go.mod: {exc}")
            continue
        if metadata.get("Module", {}).get("Path") != module:
            errors.append(f"{name}: module path differs from components.json")
        for requirement in metadata.get("Require") or []:
            peer = module_owner(requirement["Path"], family_modules)
            if peer is not None and peer != name:
                errors.append(f"{name}: cross-tool module requirement on {peer}")
        for replacement in metadata.get("Replace") or []:
            old, new = replacement["Old"], replacement["New"]
            if not new.get("Version"):
                errors.append(f"{name}: local module replacement {new['Path']} is forbidden")
            for side in (old, new):
                peer = module_owner(side["Path"], family_modules)
                if peer is not None and peer != name:
                    errors.append(f"{name}: cross-tool module replacement involving {peer}")

    # Parse only files whose resolved bytes are still within the declared leaf.
    # Report a bad link without following it to external/private source bytes.
    safe_go_paths = []
    for path in go_paths:
        try:
            source_root = worker_go[path] if path in worker_go else leaves[owner(path)]
            if inside(path.resolve(strict=True), source_root.resolve()):
                safe_go_paths.append(path)
        except (OSError, RuntimeError):
            pass
    try:
        parsed = parse_go(safe_go_paths, root, helper)
    except (ValueError, OSError) as exc:
        errors.append(f"cannot parse Go sources: {exc}")
        parsed = []
    main_dirs = set()
    standard_imports = set()
    if worker_go:
        try:
            standard_imports = set(run(["go", "list", "std"], cwd=root).splitlines())
        except (OSError, ValueError) as exc:
            errors.append(f"cannot establish standard library imports for worker helpers: {exc}")
    for source in parsed:
        path = Path(source["path"])
        name, relative = owner(path), path.relative_to(root).as_posix()
        if source.get("error"):
            errors.append(f"{relative}: invalid package/import syntax: {source['error']}")
            continue
        if source["package"] == "main" and not path.name.endswith("_test.go"):
            main_dirs.add(path.parent)
        for imported in source["imports"]:
            if path in worker_go and imported not in standard_imports:
                errors.append(f"{relative}: worker helpers may import only the standard library: {imported}")
            peer = module_owner(imported, family_modules)
            if peer is not None and peer != name:
                errors.append(f"{relative}: cross-tool Go import from {peer}: {imported}")
            if imported.startswith(("./", "../", "/", "tools/")):
                errors.append(f"{relative}: local/path Go import {imported} is forbidden")
    for component in components:
        if component["module"] is None:
            continue
        name, leaf = component["name"], leaves[component["name"]]
        declared = set()
        for command in component["commands"]:
            directory = leaf / command["package"]
            declared.add(directory)
            if directory not in main_dirs:
                errors.append(f"{name}/{command['name']}: command does not name an actual main package")
        for directory in main_dirs:
            if inside(directory, leaf / "cmd") and directory not in declared:
                errors.append(f"{directory.relative_to(root)}: command main package missing from manifest")

    if baseline:
        errors.extend(baseline_errors(root, components, files))
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=HERE.parent,
                        help="repository to inspect (defaults to this script's repository)")
    parser.add_argument("--baseline", action="store_true",
                        help="also compare all tool files/modes to original source trees")
    args = parser.parse_args()
    try:
        errors = check(args.root, baseline=args.baseline)
    except (OSError, ValueError, KeyError, TypeError) as exc:
        errors = [f"cannot complete boundary verification: {exc}"]
    if errors:
        for error in errors:
            print(f"boundary: {error}", file=sys.stderr)
        return 1
    print("boundaries: ok" + ("; source baseline: exact" if args.baseline else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
