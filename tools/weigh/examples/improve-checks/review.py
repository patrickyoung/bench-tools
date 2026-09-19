#!/usr/bin/env python3
"""Freeze and review one tested text change; apply with the existing Git command.

This caller-owned helper never changes the selected source. The conversation
owns selection and authorization; a proposal digest only identifies bytes.
"""
import argparse
import base64
import binascii
import errno
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import stat
import subprocess
import sys
import tempfile
import unicodedata

LIMIT = 8 * 1024 * 1024
EVALUATOR = Path(__file__).with_name("evaluate.py")


class Broken(ValueError):
    """A bounded, caller-safe explanation of a broken review condition."""


def raw_file(path):
    with path.open("rb") as stream:
        raw = stream.read(LIMIT + 1)
    if len(raw) > LIMIT:
        raise Broken("selected file exceeds 8 MiB")
    return raw


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, sort_keys=True, ensure_ascii=False, allow_nan=False,
                       separators=(",", ":")) + "\n").encode("utf-8")


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise Broken("duplicate JSON key")
        result[key] = value
    return result


def parsed(raw):
    if len(raw) > LIMIT:
        raise Broken("JSON input exceeds 8 MiB")
    value = json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                       parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON")))
    encoded(value)
    return value


def paths(values):
    if (not isinstance(values, list) or not values or len(values) > 128
            or any(not isinstance(value, str) for value in values)):
        raise Broken("select 1 to 128 unique paths")
    aliases = set()
    for value in values:
        path = PurePosixPath(value)
        alias = unicodedata.normalize("NFC", value).casefold()
        if (not path.parts or path.is_absolute() or str(path) != value or "\\" in value
                or any(part == ".." or part.casefold() == ".git" for part in path.parts)
                or any(ord(c) < 32 or ord(c) == 127 for c in value)
                or len(value.encode("utf-8")) > 4096):
            raise Broken("select normalized relative file paths outside .git")
        if alias in aliases:
            raise Broken("selected paths must be distinct on case-insensitive filesystems")
        aliases.add(alias)
    for alias in aliases:
        if any(str(parent) in aliases for parent in PurePosixPath(alias).parents if str(parent) != "."):
            raise Broken("selected file paths cannot contain one another")
    return sorted(values)


def selected_file(root, name):
    """Read regular files without following any selected descendant symlink."""
    descriptors = [os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)]
    try:
        parts = PurePosixPath(name).parts
        for index, part in enumerate(parts):
            last = index == len(parts) - 1
            flags = os.O_RDONLY | os.O_NOFOLLOW | (os.O_NONBLOCK if last else os.O_DIRECTORY)
            try:
                descriptor = os.open(part, flags, dir_fd=descriptors[-1])
            except FileNotFoundError:
                return None, None
            except OSError as error:
                if error.errno in (errno.ELOOP, errno.ENOTDIR):
                    raise Broken("selected paths cannot traverse symbolic links or non-directories") from None
                raise
            descriptors.append(descriptor)
        info = os.fstat(descriptors[-1])
        if not stat.S_ISREG(info.st_mode):
            raise Broken("selected paths must be regular files or absent")
        with os.fdopen(os.dup(descriptors[-1]), "rb") as stream:
            raw = stream.read(LIMIT + 1)
        if len(raw) > LIMIT:
            raise Broken("selected file exceeds 8 MiB")
        raw.decode("utf-8")
        if b"\x00" in raw:
            raise Broken("only UTF-8 text changes are supported")
        return raw, "100755" if info.st_mode & 0o111 else "100644"
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def snapshot(root, names):
    root = root.resolve(strict=True)
    if not root.is_dir():
        raise Broken("source must be a directory")
    rows, total = [], 0
    for name in paths(names):
        raw, mode = selected_file(root, name)
        if raw is None:
            rows.append({"path": name, "content": None, "mode": None})
            continue
        total += len(raw)
        if total > LIMIT:
            raise Broken("selected source exceeds 8 MiB")
        rows.append({"path": name, "content": base64.b64encode(raw).decode("ascii"),
                     "mode": mode})
    return rows


def decoded(value, label):
    if not isinstance(value, str):
        raise Broken(label + " must be canonical base64 text")
    try:
        raw = base64.b64decode(value, validate=True)
    except (ValueError, binascii.Error):
        raise Broken(label + " must be canonical base64 text") from None
    if len(raw) > LIMIT or base64.b64encode(raw).decode("ascii") != value:
        raise Broken(label + " must be bounded canonical base64 text")
    return raw


def validate_rows(rows, selected=None):
    if not isinstance(rows, list) or not 1 <= len(rows) <= 128:
        raise Broken("snapshots require 1 to 128 rows")
    for row in rows:
        if not isinstance(row, dict) or set(row) != {"path", "content", "mode"}:
            raise Broken("snapshot rows require exactly path, content and mode")
    names = paths([row["path"] for row in rows])
    if [row["path"] for row in rows] != names or selected is not None and names != selected:
        raise Broken("snapshot rows must exactly match the sorted selected file list")
    total = 0
    for row in rows:
        if row["content"] is None:
            if row["mode"] is not None:
                raise Broken("absent snapshot files require null content and mode")
            continue
        if row["mode"] not in ("100644", "100755"):
            raise Broken("snapshot files require a regular text file mode")
        raw = decoded(row["content"], "snapshot content")
        raw.decode("utf-8")
        if b"\x00" in raw:
            raise Broken("snapshots support UTF-8 text only")
        total += len(raw)
        if total > LIMIT:
            raise Broken("snapshot content exceeds 8 MiB")
    return names


def fingerprint(rows):
    validate_rows(rows)
    return sha(encoded(rows))


def run(argv, **kwargs):
    # Never allow caller Git environment settings to redirect a read-only check
    # to another repository or inject configuration/attributes into the diff.
    environment = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    environment.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
                       GIT_ATTR_NOSYSTEM="1", LC_ALL="C")
    try:
        result = subprocess.run(argv, capture_output=True, timeout=60, env=environment, **kwargs)
    except FileNotFoundError:
        raise Broken("a required dependency is unavailable") from None
    except subprocess.TimeoutExpired:
        raise Broken("a dependency exceeded the 60 second limit") from None
    if len(result.stdout) > LIMIT or len(result.stderr) > LIMIT:
        raise Broken("dependency output exceeds 8 MiB")
    return result


def patch_for(before, after):
    names = validate_rows(before)
    validate_rows(after, names)
    with tempfile.TemporaryDirectory(prefix="bench-review-") as directory:
        root = Path(directory)
        initialized = run(["git", "init", "--quiet", str(root)])
        if initialized.returncode:
            raise Broken("Git could not initialize temporary diff storage")
        # info/attributes takes precedence over selected .gitattributes files.
        (root / ".git" / "info" / "attributes").write_text("* diff -text -filter -ident -working-tree-encoding\n")
        for prefix, rows in (("old", before), ("new", after)):
            (root / prefix).mkdir()
            for row in rows:
                if row["content"] is None:
                    continue
                path = root / prefix / row["path"]
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(decoded(row["content"], "snapshot content"))
                path.chmod(0o755 if row["mode"] == "100755" else 0o644)
        result = run(["git", "--no-pager", "-c", "core.attributesFile=/dev/null", "-c", "core.fileMode=true",
                      "-c", "core.autocrlf=false", "-c", "core.quotePath=false", "diff", "--no-index",
                      "--no-ext-diff", "--no-textconv", "--binary", "--text", "--full-index",
                      "--no-renames", "--no-color", "--no-relative", "--no-indent-heuristic",
                      "--diff-algorithm=myers", "--unified=3", "--src-prefix=a/", "--dst-prefix=b/",
                      "--", "old", "new"], cwd=root)
        if result.returncode not in (0, 1):
            raise Broken("Git could not construct the selected diff")
        if result.returncode == 0 or not result.stdout:
            raise Broken("proposal must contain a text or executable-mode change")
        return result.stdout


def evaluate(raw):
    result = run([sys.executable, str(EVALUATOR), "--input", "-"], input=raw)
    if result.returncode:
        raise Broken("evaluation input did not validate")
    return parsed(result.stdout)


def text_field(value):
    if (not isinstance(value, str) or not value.strip() or len(value.encode("utf-8")) > 8192
            or any(ord(c) < 32 or ord(c) == 127 for c in value)):
        raise Broken("title, reason and tradeoff need bounded single-line text")


def valid_sha(value):
    if not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value):
        raise Broken("review identities must be lowercase SHA256 hexadecimal digests")


def bound_evaluation(raw, before, after):
    request, report = parsed(raw), evaluate(raw)
    settings = {setting["id"]: setting for setting in request["settings"]}
    if (settings[request["baseline"]]["source_sha256"] != fingerprint(before)
            or settings[request["proposal"]]["source_sha256"] != fingerprint(after)):
        raise Broken("evaluation source pins do not match the selected before and after files")
    return report


def prepare(args):
    source, candidate = args.source.resolve(strict=True), args.candidate.resolve(strict=True)
    output = args.out.resolve()
    if any(output == root or root in output.parents for root in (source, candidate)):
        raise Broken("proposal output must be outside the source and candidate directories")
    names = paths(args.file)
    before, after = snapshot(source, names), snapshot(candidate, names)
    raw = raw_file(args.evaluation)
    report = bound_evaluation(raw, before, after)
    for value in (args.title, args.reason, args.tradeoff):
        text_field(value)
    patch = patch_for(before, after)
    proposal = {"version": 1, "source": str(source), "files": names,
                "before": before, "after": after, "title": args.title,
                "reason": args.reason, "tradeoff": args.tradeoff,
                "evaluation": base64.b64encode(raw).decode("ascii"), "report": report,
                "evaluator_sha256": sha(raw_file(EVALUATOR)),
                "patch": base64.b64encode(patch).decode("ascii")}
    if before != snapshot(source, names) or after != snapshot(candidate, names):
        raise Broken("selected files changed during preparation")
    payload = encoded(proposal)
    if len(payload) > LIMIT:
        raise Broken("proposal exceeds 8 MiB")
    args.out.mkdir(mode=0o700, parents=True, exist_ok=False)
    (args.out / "proposal.json").write_bytes(payload)
    (args.out / "change.patch").write_bytes(patch)
    return proposal, sha(payload)


def load(directory, reviewed=None):
    raw = raw_file(directory / "proposal.json")
    digest = sha(raw)
    if reviewed is not None:
        valid_sha(reviewed)
        if reviewed != digest:
            raise Broken("proposal changed since review; present a fresh suggestion")
    proposal = parsed(raw)
    required = {"version", "source", "files", "before", "after", "title", "reason",
                "tradeoff", "evaluation", "report", "evaluator_sha256", "patch"}
    if (not isinstance(proposal, dict) or set(proposal) != required
            or type(proposal["version"]) is not int or proposal["version"] != 1):
        raise Broken("invalid proposal")
    source = proposal["source"]
    if (not isinstance(source, str) or not Path(source).is_absolute() or str(Path(source)) != source
            or ".." in Path(source).parts or len(source.encode("utf-8")) > 8192
            or any(ord(c) < 32 or ord(c) == 127 for c in source)):
        raise Broken("proposal source must be an absolute normalized directory path")
    names = paths(proposal["files"])
    if names != proposal["files"]:
        raise Broken("selected file list must be sorted")
    validate_rows(proposal["before"], names)
    validate_rows(proposal["after"], names)
    for field in ("title", "reason", "tradeoff"):
        text_field(proposal[field])
    valid_sha(proposal["evaluator_sha256"])
    patch = decoded(proposal["patch"], "patch")
    if patch != raw_file(directory / "change.patch"):
        raise Broken("patch differs from reviewed proposal")
    if sha(raw_file(EVALUATOR)) != proposal["evaluator_sha256"]:
        raise Broken("evaluator changed; repeat evaluation and review")
    report = bound_evaluation(decoded(proposal["evaluation"], "evaluation"), proposal["before"], proposal["after"])
    if report != proposal["report"]:
        raise Broken("evaluation differs from reviewed report")
    # A fresh diff prevents a hand-edited proposal from smuggling an unrelated patch.
    if patch != patch_for(proposal["before"], proposal["after"]):
        raise Broken("patch does not match selected snapshots")
    return proposal, digest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    finger = sub.add_parser("fingerprint", help="hash the explicit selected source files for evaluation settings")
    finger.add_argument("--root", type=Path, required=True)
    finger.add_argument("--file", action="append", required=True)
    prep = sub.add_parser("prepare", help="freeze a tested proposal without modifying source")
    for field in ("source", "candidate", "evaluation", "out"):
        prep.add_argument("--" + field, type=Path, required=True)
    prep.add_argument("--file", action="append", required=True)
    for field in ("title", "reason", "tradeoff"):
        prep.add_argument("--" + field, required=True)
    show = sub.add_parser("show", help="show one numbered suggestion and exact review identity")
    show.add_argument("proposal", type=Path)
    show.add_argument("--number", type=int, default=1)
    verify = sub.add_parser("verify", help="after acceptance, check exact reviewed bytes and current source")
    verify.add_argument("proposal", type=Path)
    verify.add_argument("--reviewed-sha", required=True)
    args = parser.parse_args()
    try:
        if args.command == "show" and not 1 <= args.number <= 10000:
            raise Broken("suggestion number must be between 1 and 10000")
        if args.command == "fingerprint":
            print(fingerprint(snapshot(args.root, args.file)))
            return 0
        if args.command == "prepare":
            proposal, digest = prepare(args)
        else:
            proposal, digest = load(args.proposal, args.reviewed_sha if args.command == "verify" else None)
        if args.command == "verify":
            if proposal["report"]["decision"] != "supported":
                raise Broken("proposal needs more evidence; no supported change to apply")
            if Path(proposal["source"]).resolve(strict=True) != Path(proposal["source"]):
                raise Broken("source directory identity changed since review; prepare a fresh proposal")
            if snapshot(Path(proposal["source"]), proposal["files"]) != proposal["before"]:
                raise Broken("source changed since review; prepare and review a fresh proposal")
            command = ["git", "-C", proposal["source"], "--git-dir=" + str(Path(proposal["source"]) / ".git"),
                       "--work-tree=" + proposal["source"], "-c", "core.autocrlf=false", "-c", "core.fileMode=true",
                       "apply", "--binary", "--whitespace=nowarn", "-p2", "--", str((args.proposal / "change.patch").resolve())]
            position = command.index("apply") + 1
            checked = run(command[:position] + ["--check"] + command[position:])
            if checked.returncode:
                raise Broken("Git cannot apply the exact reviewed patch")
            if (sha(raw_file(args.proposal / "proposal.json")) != digest
                    or raw_file(args.proposal / "change.patch") != decoded(proposal["patch"], "patch")
                    or snapshot(Path(proposal["source"]), proposal["files"]) != proposal["before"]):
                raise Broken("proposal or source changed during verification; review the current bytes")
            print(json.dumps({"reviewed_sha256": digest, "apply_argv": command,
                              "expected_source_sha256": fingerprint(proposal["after"])}))
            return 0
        report = proposal["report"]
        number = args.number if args.command == "show" else 1
        result = {"number": number, "title": proposal["title"], "why": proposal["reason"],
                  "tested": report["decision"], "scope": report["scope"],
                  "evidence": report["evaluation"], "tradeoff": proposal["tradeoff"],
                  "files": proposal["files"], "reviewed_sha256": digest,
                  "next": f"Apply {number}, inspect the change, or leave it." if report["decision"] == "supported"
                  else "Test further or leave it; this is not a supported improvement."}
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0
    except Broken as error:
        print("review: " + str(error), file=sys.stderr)
        return 2
    except (OSError, ValueError, TypeError, KeyError, OverflowError, RecursionError, subprocess.SubprocessError):
        print("review: invalid, stale or unsupported proposal; repeat preparation/review with current evidence", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
