"""Build pinned modules through caller-selected transport, without ambient authority."""
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from urllib.parse import urlsplit
from unittest.mock import patch
import zipfile


ROOT = Path(__file__).resolve().parents[2]
BUILD = runpy.run_path(str(ROOT / "scripts/build"))
TRANSPORT = ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
             "http_proxy", "https_proxy", "all_proxy", "no_proxy",
             "SSL_CERT_FILE", "SSL_CERT_DIR", "GIT_SSL_CAINFO", "GIT_SSL_CAPATH")
PRIVATE = ("OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ASK", "BRIEF_PATH",
           "GIT_SSL_NO_VERIFY", "GOINSECURE", "PYTHONHTTPSVERIFY")


def module_hash(files):
    summary = "".join(hashlib.sha256(data).hexdigest() + "  " + name + "\n"
                      for name, data in sorted(files.items()))
    return "h1:" + base64.b64encode(hashlib.sha256(summary.encode()).digest()).decode()


class BuildTransportEnvironmentTests(unittest.TestCase):
    def test_build_transport_does_not_change_the_isolated_check_environment(self):
        ambient = dict.fromkeys(TRANSPORT, "")
        ambient.update(dict.fromkeys(PRIVATE, "must-not-enter-build"))
        with patch.dict(os.environ, ambient, clear=True):
            isolated = BUILD["CHECK"]["isolated_environment"](
                Path("/fixture/home"), Path("/fixture/tmp"), Path("/fixture/bin"),
                {"GOCACHE": "/fixture/cache", "GOMODCACHE": "/fixture/modules"})
            compiler = BUILD["compiler_environment"](isolated)
        for name in TRANSPORT:
            self.assertNotIn(name, isolated)
            self.assertEqual(compiler[name], "")
        for name in PRIVATE:
            self.assertNotIn(name, compiler)
        self.assertEqual(compiler["GOWORK"], "off")
        self.assertEqual(compiler["GOFLAGS"], "")


@unittest.skipUnless(shutil.which("go") and shutil.which("git"), "Go and Git build prerequisites")
class BuildProxyWorkflowTests(unittest.TestCase):
    def test_real_module_download_uses_proxy_and_keeps_checks_isolated(self):
        with tempfile.TemporaryDirectory(prefix="bench-build-proxy-") as scratch:
            base = Path(scratch).resolve()
            root = base / "checkout"
            (root / "scripts").mkdir(parents=True)
            for name in ("build", "check"):
                shutil.copy2(ROOT / "scripts" / name, root / "scripts" / name)
            check_report, compiler_report = base / "check.json", base / "compiler.json"
            (root / "scripts/check-boundaries.py").write_text(
                "import json, os\nfrom pathlib import Path\n" +
                "Path(" + repr(str(check_report)) + ").write_text(json.dumps([name for name in " +
                repr(TRANSPORT + PRIVATE) + " if name in os.environ]))\n")
            leaf = root / "tools/probe"
            leaf.mkdir(parents=True)
            dependency, version = "example.invalid/dependency", "v0.0.1"
            module = b"module example.invalid/dependency\n\ngo 1.26\n"
            files = {dependency + "@" + version + "/go.mod": module,
                     dependency + "@" + version + "/value.go":
                         b'package dependency\nconst Message = "downloaded-through-proxy"\n'}
            archive = io.BytesIO()
            with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as target:
                for name, content in files.items():
                    target.writestr(name, content)
            (leaf / "go.mod").write_text("module example.invalid/probe\n\ngo 1.26\n\nrequire " +
                                         dependency + " " + version + "\n")
            (leaf / "go.sum").write_text(dependency + " " + version + " " + module_hash(files) + "\n" +
                                         dependency + " " + version + "/go.mod " +
                                         module_hash({"go.mod": module}) + "\n")
            (leaf / "main.go").write_text('package main\nimport ("fmt"; "example.invalid/dependency")\n' +
                                         'func main() { fmt.Print(dependency.Message) }\n')
            (root / "components.json").write_text(json.dumps({"components": [{
                "name": "probe", "path": "tools/probe", "module": "example.invalid/probe",
                "source": {"commit": "proxy-fixture"}, "commands": [{"name": "probe", "package": "."}]}]}))
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(["git", "add", "."], cwd=root, check=True)
            subprocess.run(["git", "-c", "user.name=Proxy Fixture", "-c", "user.email=fixture@example.invalid",
                            "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"], cwd=root, check=True)

            compiler = str(Path(shutil.which("go")).resolve())
            wrappers = base / "compiler-probe"
            wrappers.mkdir()
            wrapper = wrappers / "go"
            wrapper.write_text("#!" + sys.executable + "\nimport json, os, sys\nfrom pathlib import Path\n" +
                               "if sys.argv[1:2] == ['build']:\n" +
                               "    Path(" + repr(str(compiler_report)) + ").write_text(json.dumps({name: " +
                               "os.environ[name] for name in " + repr(TRANSPORT + PRIVATE) +
                               " if name in os.environ}))\n" +
                               "os.execv(" + repr(compiler) + ", [" + repr(compiler) + ", *sys.argv[1:]])\n")
            wrapper.chmod(0o755)
            payloads = {"/" + dependency + "/@v/" + version + ".mod": module,
                        "/" + dependency + "/@v/" + version + ".zip": archive.getvalue(),
                        "/" + dependency + "/@v/" + version + ".info":
                            json.dumps({"Version": version, "Time": "2026-01-01T00:00:00Z"}).encode()}
            requests = []

            class ModuleProxy(BaseHTTPRequestHandler):
                def do_GET(self):
                    requests.append(self.path)
                    url = urlsplit(self.path)
                    data = payloads.get(url.path) if url.hostname == "bench-modules.invalid" else None
                    self.send_response(200 if data is not None else 404)
                    self.end_headers()
                    self.wfile.write(data if data is not None else b"unknown module fixture")

                def log_message(self, *args):
                    pass

            with ThreadingHTTPServer(("127.0.0.1", 0), ModuleProxy) as server:
                worker = threading.Thread(target=server.serve_forever, daemon=True)
                worker.start()
                try:
                    for casing in (str.upper, str.lower):
                        with self.subTest(casing=casing.__name__):
                            proxy = "http://127.0.0.1:" + str(server.server_port)
                            selected = {casing(name): proxy for name in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY")}
                            selected[casing("NO_PROXY")] = "bypass.invalid"
                            selected.update({"SSL_CERT_FILE": str(base / "root-ca.pem"),
                                             "SSL_CERT_DIR": str(base / "certs"),
                                             "GIT_SSL_CAINFO": str(base / "git-ca.pem"),
                                             "GIT_SSL_CAPATH": str(base / "git-certs")})
                            env = {name: value for name, value in os.environ.items()
                                   if name not in TRANSPORT + PRIVATE}
                            env.update(selected)
                            env.update(dict.fromkeys(PRIVATE, "must-not-enter-build"))
                            env.update(PATH=str(wrappers) + os.pathsep + os.environ["PATH"],
                                       GOMODCACHE=str(base / ("module-cache-" + casing.__name__)),
                                       GOPROXY="http://bench-modules.invalid", GOSUMDB="off")
                            previous_requests = len(requests)
                            result = subprocess.run([sys.executable, str(root / "scripts/build"), "probe",
                                                     "--output", str(base / "output")], cwd=base, env=env,
                                                    capture_output=True, text=True, timeout=90)
                            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                            self.assertTrue(any(path.endswith(".zip") for path in requests[previous_requests:]),
                                            "the empty module cache must require a real proxy download")
                            self.assertEqual(json.loads(compiler_report.read_text()), selected)
                            self.assertEqual(json.loads(check_report.read_text()), [])
                            self.assertEqual(subprocess.check_output([str(base / "output/bin/probe")], text=True),
                                             "downloaded-through-proxy")
                finally:
                    server.shutdown()
                    worker.join(timeout=5)


if __name__ == "__main__":
    unittest.main()
