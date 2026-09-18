"""Fetch source-matched independent packages for the source setup helper."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import platform
import re
import runpy
import tarfile
import tempfile
import urllib.request

from install_support import digest, verify_package

REPOSITORY = "https://github.com/patrickyoung/bench-tools"
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
REVISION = re.compile(r"[0-9a-f]{40}\Z")
ASSET_URL = re.compile(re.escape(REPOSITORY) + r"/releases/download/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+\Z")
MAX_EXPANDED = 512 * 1024 * 1024


def host_platform():
    machine = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    system = platform.system().lower()
    return system + "-" + machine if system in ("darwin", "linux") and machine else None


def current_sources(root, tools):
    check = runpy.run_path(str(root / "scripts/check"))
    components = {item["name"]: item for item in json.loads((root / "components.json").read_text())["components"]}
    sources = {}
    with tempfile.TemporaryDirectory(prefix="bench-source-match-") as scratch:
        for name in tools:
            records = check["export_source"](root, components[name], Path(scratch) / name)
            sources[name] = hashlib.sha256(json.dumps(records, sort_keys=True).encode()).hexdigest()
    return sources


def select_artifact(root, tools):
    path = root / "releases/builder.json"
    if not path.exists():
        print("No published package pin in this checkout; building from source.", flush=True)
        return None
    record = json.loads(path.read_text())
    if not isinstance(record, dict) or record.get("schema") != 1 or not isinstance(record.get("artifacts"), dict):
        raise ValueError("invalid releases/builder.json")
    target = host_platform()
    artifact = record["artifacts"].get(target)
    if artifact is None:
        print("No published packages for this host; building from source.", flush=True)
        return None
    if not isinstance(artifact, dict):
        raise ValueError("invalid published package entry")
    identity = {"schema": 1, "repository": REPOSITORY, "platform": target, "tools": list(tools)}
    if any(artifact.get(key) != value for key, value in identity.items()):
        raise ValueError("invalid published package identity")
    for name, pattern in (("sha256", SHA256), ("revision", REVISION), ("url", ASSET_URL)):
        if not isinstance(artifact.get(name), str) or not pattern.fullmatch(artifact[name]):
            raise ValueError("invalid published package " + name)
    if type(artifact.get("size")) is not int or not 0 < artifact["size"] <= MAX_EXPANDED:
        raise ValueError("invalid published package size")
    for key in ("sources", "packages"):
        values = artifact.get(key)
        if not isinstance(values, dict) or set(values) != set(tools) or any(
                not isinstance(value, str) or not SHA256.fullmatch(value) for value in values.values()):
            raise ValueError("invalid published package " + key)
    if current_sources(root, tools) != artifact["sources"]:
        print("Published packages do not match current component sources; building from source.", flush=True)
        return None
    return dict(artifact, platform=target)


def download(artifact, destination):
    # urllib uses the caller's normal proxy and CA settings. This does not
    # change the host's egress policy or turn off certificate verification.
    print("Downloading verified packages for " + artifact["platform"], flush=True)
    request = urllib.request.Request(artifact["url"], headers={"User-Agent": "bench-tools-setup"})
    size = 0
    with urllib.request.urlopen(request, timeout=60) as response, destination.open("xb") as output:
        if not response.geturl().startswith("https://"):
            raise ValueError("package download redirected away from HTTPS")
        while True:
            chunk = response.read(1024 * 1024)
            if not chunk:
                break
            size += len(chunk)
            if size > artifact["size"]:
                raise ValueError("published package size mismatch")
            output.write(chunk)
    if size != artifact["size"] or digest(destination) != artifact["sha256"]:
        raise ValueError("published package checksum or size mismatch")


def extract(archive, destination, tools):
    """Accept only bounded regular files, without tar's link/path semantics."""
    destination.mkdir()
    seen, total = set(), 0
    with tarfile.open(archive, "r:gz") as source:
        for member in source:
            path = PurePosixPath(member.name)
            if (member.name in seen or not member.isfile() or path.is_absolute()
                    or path.as_posix() != member.name or ".." in path.parts
                    or "\\" in member.name or "\x00" in member.name
                    or member.size < 0
                    or member.mode & ~0o777 or not member.mode & 0o400
                    or not (member.name == "runtime.json" or
                            (len(path.parts) >= 3 and path.parts[0] == "tools" and path.parts[1] in tools))):
                raise ValueError("unsafe or unexpected archive member: " + member.name)
            total += member.size
            seen.add(member.name)
            if total > MAX_EXPANDED or len(seen) > 20000:
                raise ValueError("published package archive exceeds extraction limits")
            target = destination.joinpath(*path.parts)
            target.parent.mkdir(parents=True, exist_ok=True)
            with source.extractfile(member) as payload, target.open("xb") as output:
                while True:
                    chunk = payload.read(1024 * 1024)
                    if not chunk:
                        break
                    output.write(chunk)
            target.chmod(member.mode)


def verify_extracted(directory, artifact, tools):
    metadata = json.loads((directory / "runtime.json").read_text())
    expected = {"schema": 1, "repository": REPOSITORY, "revision": artifact["revision"],
                "platform": artifact["platform"], "tools": list(tools),
                "sources": artifact["sources"], "packages": artifact["packages"]}
    if metadata != expected:
        raise ValueError("published package metadata does not match the checked-in pin")
    for name in tools:
        package = directory / "tools" / name
        if digest(package / "package.json") != artifact["packages"][name]:
            raise ValueError("published package receipt mismatch: " + name)
        receipt = verify_package(package, name)
        source = receipt.get("source", {})
        if (not isinstance(source, dict) or
                source.get("repository_revision") != artifact["revision"] or
                source.get("files_sha256") != artifact["sources"][name] or
                source.get("path") != "tools/" + name):
            raise ValueError("published package source mismatch: " + name)


def prepare_prebuilt(root, tools, work):
    artifact = select_artifact(root, tools)
    if artifact is None:
        return None
    archive, packages = work / "packages.tar.gz", work / "packages"
    try:
        download(artifact, archive)
        extract(archive, packages, tools)
        verify_extracted(packages, artifact, tools)
    except (OSError, ValueError, EOFError, tarfile.TarError) as error:
        raise ValueError("cannot install the pinned packages: " + str(error) +
                         "; inspect network/integrity errors, or select --from-source to compile") from error
    return packages
