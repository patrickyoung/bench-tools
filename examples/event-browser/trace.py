#!/usr/bin/env python3
"""Bench Trace: a single-file, read-only Ask and Record event browser.

python3 trace.py --ask /path/to/ask --record /path/to/record NAME=/agent/evidence
python3 trace.py /first/evidence /second/evidence --export replay.html

Only Python's standard library and the public Ask/Record executables are used.
The HTML application is embedded at the end of this file. Nothing in an archive
is executed. All command arguments come from this program or the operator.
"""

import argparse
import base64
import copy
import hashlib
import heapq
import json
import os
import re
from pathlib import Path
import secrets
import shutil
import signal
import stat as stat_module
import subprocess
import sys
import tempfile
import threading
import time
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import unquote, urlsplit
import webbrowser

SCHEMA = "bench.trace/v1"
PREVIEW = 16384
DETAIL = 24000
EXCLUDED = {".git", "node_modules", ".venv", "__pycache__", "state", ".build", ".checks"}


def identity(value):
    return hashlib.sha256(str(value).encode("utf-8", "surrogateescape")).hexdigest()[:24]


def bdecode(value):
    return base64.b64decode(value or "", validate=True)


def btext(value):
    try:
        return bdecode(value).decode("utf-8", "replace")
    except (ValueError, TypeError):
        return "[invalid base64]"


def millis(value):
    try:
        value = re.sub(r"\.(\d+)(?=Z|[+-]\d\d:\d\d$)", lambda m: "." + (m[1] + "000000")[:6], str(value))
        return int(datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp() * 1000)
    except (ValueError, TypeError, OverflowError):
        return 0


def text_blocks(data):
    blocks = data.get("blocks") or []
    return "\n".join(str(b.get("text", "")) for b in blocks if isinstance(b, dict)) if blocks else str(data.get("text", ""))


def compact(value, limit=DETAIL):
    text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False, indent=2)
    return text if len(text) <= limit else text[:limit] + "\n… Preview only; open Raw for the complete event."


def outcome(code):
    return {0: "accepted", 2: "unfinished", 75: "unfinished", 125: "unknown", 130: "interrupted"}.get(code, "failed")


def execute(argv, output=None, timeout=30):
    """Fixed read-only public tool invocations, with bounded process lifetime."""
    with subprocess.Popen(list(map(str, argv)), stdin=subprocess.DEVNULL,
                          stdout=output or subprocess.PIPE, stderr=subprocess.PIPE,
                          start_new_session=True) as child:
        try:
            stdout, stderr = child.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(child.pid, signal.SIGKILL)
            child.communicate()
            raise ValueError("Replay command timed out; increase --command-timeout for large evidence.")
    return child.returncode, stdout or b"", stderr.decode("utf-8", "replace")[:12000]


class Archive:
    def __init__(self, targets, ask="ask", record="record", max_bytes=256 * 1024 * 1024,
                 max_sessions=2000, timeout=30, max_embedded_bytes=256 * 1024 * 1024):
        self.ask, self.record = ask, record
        self.max_bytes, self.max_sessions, self.timeout = max_bytes, max_sessions, timeout
        self.max_embedded_bytes = max_embedded_bytes
        self.temp = tempfile.TemporaryDirectory(prefix="bench-trace-")
        self.directory = Path(self.temp.name)
        self.lock = threading.RLock()
        self.cache, self.sessions, self.raw, self.aliases = {}, {}, {}, {}
        self.sources = []
        for target in targets:
            label, sep, selected = str(target).partition("=")
            path = Path(selected if sep else target).expanduser().absolute()
            if path.is_dir() and (path / ".agent").is_dir():
                path = path / ".agent"
            self.sources.append({"id": identity(path), "label": label if sep else path.name or str(path), "path": str(path)})
        self.revision, self.fingerprint, self.source_fingerprint = 0, None, None
        self.snapshot = self.empty()

    def empty(self):
        return {"schema": SCHEMA, "revision": self.revision, "live": True, "clockSync": "unknown", "collectedAt": int(time.time() * 1000),
                "sources": self.sources, "lanes": [], "sessions": [], "events": [], "edges": [], "issues": []}

    def close(self):
        self.temp.cleanup()

    def command(self, argv, output=None):
        return execute(argv, output, self.timeout)

    def candidates(self):
        found, issues = {}, []
        for source in self.sources:
            root = Path(source["path"])
            if root.is_symlink():
                issues.append({"path": str(root), "message": "Select a regular evidence directory or file, not a symlink.", "severity": "warning"})
                continue
            if root.is_file():
                paths = [root]
            elif root.is_dir():
                paths = []
                for directory, dirs, files in os.walk(root, followlinks=False):
                    dirs[:] = sorted(d for d in dirs if d not in EXCLUDED and not Path(directory, d).is_symlink())
                    paths.extend(Path(directory, f) for f in sorted(files) if f.endswith(".jsonl"))
                    if len(paths) > self.max_sessions:
                        break
            else:
                issues.append({"path": str(root), "message": "Waiting for this selected evidence path to appear.", "severity": "info"})
                continue
            for path in paths:
                if path.is_symlink() or not path.is_file():
                    continue
                key = str(path.absolute())
                if key in found:
                    continue
                if len(found) >= self.max_sessions:
                    issues.append({"path": str(root), "message": "Session limit reached; narrow the selected roots or increase --max-sessions.", "severity": "warning"})
                    break
                found[key] = source
        return found, issues

    def load(self, key, source, blob=None, original=None, embedded=False):
        path = Path(key)
        sid = identity(key)
        if blob is None:
            descriptor = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | os.O_NONBLOCK)
            with os.fdopen(descriptor, "rb") as stream:
                before = os.fstat(stream.fileno())
                if not stat_module.S_ISREG(before.st_mode):
                    raise ValueError("Evidence must be a regular file.")
                if before.st_size > self.max_bytes:
                    raise ValueError("Evidence exceeds --max-file-mib; it was not silently truncated.")
                blob = stream.read(self.max_bytes + 1)
                after = os.fstat(stream.fileno())
            if (before.st_ino, before.st_mtime_ns, before.st_size) != (after.st_ino, after.st_mtime_ns, after.st_size):
                raise ValueError("Evidence changed while reading; waiting for the next snapshot.")
        if len(blob) > self.max_bytes:
            raise ValueError("Evidence exceeds --max-file-mib; it was not silently truncated.")
        if not blob:
            return None
        # Identify Ask files only. Ask, through its public command, owns parsing
        # and verification. No fold, canonicalization or seal implementation here.
        try:
            first = json.loads(blob.split(b"\n", 1)[0])
        except (ValueError, UnicodeError):
            return None
        if not isinstance(first, dict) or first.get("type") != "session":
            return None
        clipped = False
        if not blob.endswith(b"\n"):
            try:
                json.loads(blob.rsplit(b"\n", 1)[-1])
            except (ValueError, UnicodeError):
                clipped = True
        observed = blob if not clipped else blob[:blob.rfind(b"\n") + 1]
        sha = hashlib.sha256(blob).hexdigest()
        saved = self.directory / (sid + "-" + sha + ".jsonl")
        saved.write_bytes(observed)
        saved.chmod(0o600)
        original_file = saved
        if clipped:
            original_file = saved.with_suffix(".original")
            original_file.write_bytes(blob)
            original_file.chmod(0o600)
        public = saved.with_suffix(".events")
        with public.open("wb") as output:
            code, _, diagnostic = self.command([self.ask, "replay", "-check", "-json", saved], output)
        verification = "verified" if code == 0 else "invalid"
        if code:
            if "not immediately sealed" in diagnostic:
                verification = "incomplete"
            with public.open("wb") as output:
                self.command([self.ask, "replay", "-json", saved], output)
        if clipped:
            verification = "incomplete" if code == 0 else verification
            diagnostic = (diagnostic + "\nAn unfinished final line is omitted from the displayed prefix; the source remains unchanged.").strip()
        events = []
        with public.open(encoding="utf-8") as output:
            for line in output:
                try:
                    event = json.loads(line)
                    if isinstance(event, dict) and isinstance(event.get("type"), str):
                        events.append(event)
                except ValueError:
                    break
        if not events:
            raise ValueError(diagnostic or "No readable Ask events.")
        header = events[0].get("data") or {}
        kinds = {e["data"].get("kind", "") for e in events if e.get("type") == "note" and isinstance(e.get("data"), dict)}
        kind = "agent" if "ply.recording/v1" in kinds else "process" if "record.intent/v1" in kinds else "conversation"
        session = {"id": sid, "source": source["id"], "laneId": sid, "kind": kind,
                   "title": header.get("model") or path.stem, "path": original or key,
                   "state": "open", "verification": verification, "complete": False,
                   "start": millis(events[0].get("time")), "end": None,
                   "model": header.get("model", ""), "eventCount": len(events), "bytes": len(blob),
                   "sha256": sha, "embedded": embedded, "streams": [], "problem": diagnostic,
                   "role": "", "argv": [], "cwd": "", "exit": None, "signal": 0,
                   "sessionKey": header.get("id", ""), "parentKey": header.get("parent", ""),
                   "summaryKey": header.get("summary", "")}
        entry = {"session": session, "events": events, "file": saved, "originalFile": original_file, "public": public,
                 "source": source, "original": original or key, "index": [], "intent": {}, "terminal": {}, "key": key}
        if kind == "process":
            code, out, error = self.command([self.record, "replay", "-ask", self.ask, "-f", saved, "-json"])
            if code == 0 and not clipped:
                receipt = json.loads(out)
                entry.update(intent=receipt["intent"], terminal=receipt["terminal"])
                session["verification"] = "verified"
                session["complete"] = True
            else:
                if session["verification"] == "verified":
                    session["verification"] = "incomplete"
                session["problem"] = (session["problem"] + "\n" + error).strip()
        return entry

    def project(self, entry):
        s = entry["session"]
        stream_info, projected = {}, []
        requests = []
        process_start = None
        for raw in entry["events"]:
            seq, kind = raw.get("seq", 0), raw.get("type", "note")
            data = raw.get("data") if isinstance(raw.get("data"), dict) else {"value": raw.get("data")}
            at = millis(raw.get("time"))
            ev = {"id": s["id"] + ":" + str(seq), "sessionId": s["id"], "laneId": s["laneId"],
                  "seq": seq, "time": at, "type": "note", "title": kind, "text": "", "status": "", "duration": None,
                  "details": {}, "stream": None, "bytes": 0, "offset": 0}
            if kind == "seal":
                continue
            if kind == "session":
                ev.update(type="context", title="Session context", text=compact(data.get("system", "")),
                          details={"model": data.get("model", ""), "session": data.get("id", ""), "parent": data.get("parent", "")})
            elif kind == "request":
                ev.update(type="model", title="Model request · " + str(data.get("model", s["model"])),
                          text="Open Raw to reconstruct the exact normalized request through Ask.", details={"model": data.get("model"), "effort": data.get("effort")})
                requests.append(ev)
                s.update(state="open", complete=False, end=None)
            elif kind == "assistant":
                ev.update(type="response", title="Partial response" if data.get("partial") else "Model response",
                          text=compact(text_blocks(data)), details={"usage": data.get("usage"), "model": data.get("model"), "partial": data.get("partial", False)},
                          status="incomplete" if data.get("partial") else "complete")
                if requests:
                    requests[-1]["duration"] = max(0, at - requests[-1]["time"])
                    requests[-1]["status"] = ev["status"]
            elif kind == "user":
                ev.update(type="input", title="Input" + (" · " + str(data["source"]) if data.get("source") else ""), text=compact(text_blocks(data)))
            elif kind == "done":
                state = "complete" if data.get("reason") == "end" else "unfinished" if data.get("reason") == "overflow" else "failed"
                s.update(state=state, complete=True, end=at)
                ev.update(type="result", title="Conversation " + state, status=state, text=compact(data))
            elif kind == "abort":
                s.update(state="interrupted", complete=False, end=at)
                ev.update(type="error", title="Interrupted conversation", status="interrupted")
            elif kind == "retry":
                ev.update(type="retry", title="Provider retry", text=compact(data), status="unfinished")
            elif kind == "note":
                note_kind, body = data.get("kind", ""), data.get("body", {})
                if not isinstance(body, dict):
                    body = {"value": body}
                if note_kind == "record.intent/v1":
                    entry["intent"] = body
                    argv = [btext(a) for a in body.get("argv", [])]
                    labels = body.get("labels") or []
                    role = next((label[5:] for label in labels if isinstance(label, str) and label.startswith("role=")), "process")
                    s.update(argv=argv, cwd=btext(body.get("cwd")), role=role,
                             title=role + " · " + " ".join(argv)[:140])
                    ev.update(type="check" if role == "verifier" else "action", title=" ".join(argv)[:160] or "Process invocation", text="\n".join(argv), details={"argv": argv, "cwd": s["cwd"], "labels": labels})
                    if argv == ["/usr/bin/true"] and body.get("artifacts"):
                        phases = {a.get("phase", "") for a in body["artifacts"]}
                        title = "Retain inputs" if phases == {"input"} else "Retain outputs and conversations"
                        s.update(title=title, role="snapshot")
                        ev.update(type="artifact", title=title)
                    process_start = ev
                elif note_kind == "record.chunk/v1":
                    name = body.get("stream", "")
                    try:
                        chunk = bdecode(body.get("data"))
                    except (ValueError, TypeError):
                        chunk = b""
                    info = stream_info.setdefault(name, {"size": 0, "prefix": bytearray()})
                    info["size"] += len(chunk)
                    info["prefix"].extend(chunk[:max(0, PREVIEW - len(info["prefix"]))])
                    ev.update(type=name if name in ("stdout", "stderr") else "input" if name == "stdin" else "artifact",
                              title=name + " · " + str(len(chunk)) + " bytes", stream=name, bytes=len(chunk), offset=body.get("offset", 0),
                              text=chunk[:2000].decode("utf-8", "replace"))
                elif note_kind == "record.terminal/v1":
                    entry["terminal"] = body
                    complete = bool(body.get("complete")) and s["verification"] == "verified"
                    state = ("complete" if body.get("exit") == 0 else outcome(body.get("exit"))) if complete else "incomplete"
                    s.update(state=state, complete=complete, exit=body.get("exit"), signal=body.get("signal", 0), end=at)
                    ev.update(type="result", title="Process " + state, status=state, text=compact(body), details=body)
                    if process_start:
                        process_start.update(duration=max(0, at - process_start["time"]), status=state)
                elif note_kind == "ply.recording/v1":
                    entry["index"].append(body)
                    phase = body.get("phase", "")
                    if phase == "start":
                        s.update(cwd=body.get("directory", ""), title=Path(body.get("directory", "agent")).name or "Agent")
                        ev.update(type="start", title="Agent invocation", text=body.get("directory", ""), details=body)
                    elif phase == "terminal":
                        complete = body.get("complete") is True and s["verification"] == "verified"
                        state = outcome(body.get("exit")) if complete else "incomplete"
                        s.update(state=state, complete=complete, exit=body.get("exit"), end=at)
                        ev.update(type="end", title="Agent " + state, status=state, details=body, text=compact(body))
                    else:
                        ev.update(title="Recorded link · " + phase, details=body, text=compact(body))
                elif note_kind.startswith("ply.verifier/"):
                    status = "accepted" if body.get("outcome") == "accepted" else "failed" if body.get("outcome") == "rejected" else "unknown"
                    observed = btext(body.get("output")) if note_kind == "ply.verifier/v2" else str(body.get("output", ""))
                    ev.update(type="check", title="Check " + str(body.get("outcome", "unknown")), text=compact(observed),
                              details={key: value for key, value in body.items() if key != "output"}, status=status)
                else:
                    ev.update(title=note_kind or "Note · " + str(data.get("source", "")), text=compact(body if note_kind else data.get("text", "")), details={"source": data.get("source", ""), "kind": note_kind})
            else:
                ev.update(text=compact(data))
            projected.append(ev)
        if s["kind"] == "process" and not s["complete"]:
            s["state"] = "incomplete"
        if s["verification"] == "invalid":
            s.update(state="invalid", complete=False)
        elif s["verification"] == "incomplete":
            s.update(state="incomplete", complete=False)
        artifacts = entry["intent"].get("artifacts") or []
        names = set(stream_info)
        if s["kind"] == "process":
            names.update(["stdin", "stdout", "stderr"])
            names.update("artifact:" + str(i) for i in range(len(artifacts)))
        for name in sorted(names):
            info = stream_info.get(name, {"size": 0, "prefix": b""})
            payload = bytes(info["prefix"])
            binary = b"\0" in payload
            try:
                preview = payload.decode("utf-8")
            except UnicodeError:
                preview, binary = payload.decode("utf-8", "replace"), True
            row = {"name": name, "bytes": info["size"], "preview": preview, "truncated": info["size"] > len(payload),
                   "encoding": "binary" if binary else "utf-8", "verified": s["verification"] == "verified", "complete": s["complete"]}
            row["sha256"] = entry["terminal"].get("streams", {}).get(name, {}).get("sha256", "")
            if name.startswith("artifact:"):
                index = int(name.partition(":")[2])
                if index < len(artifacts):
                    artifact = artifacts[index]
                    row.update(filename=artifact.get("name", name), originalPath=btext(artifact.get("path")), phase=artifact.get("phase", ""))
            s["streams"].append(row)
        entry["projected"] = projected

    def observed_stream(self, entry, name):
        out = bytearray()
        for event in entry["events"]:
            note = event.get("data") or {}
            if not isinstance(note, dict):
                continue
            body = note.get("body", {})
            if note.get("kind") != "record.chunk/v1" or not isinstance(body, dict) or body.get("stream") != name:
                continue
            if body.get("offset") != len(out):
                raise ValueError("Observed stream has an offset gap; it cannot be presented as contiguous bytes.")
            out.extend(bdecode(body.get("data")))
        return bytes(out)

    def refresh(self):
        with self.lock:
            candidates, issues = self.candidates()
            stamps = {}
            for key in candidates:
                try:
                    stat = Path(key).stat()
                    stamps[key] = (stat.st_ino, stat.st_mtime_ns, stat.st_size)
                except OSError:
                    stamps[key] = None
            current = (stamps, json.dumps(issues, sort_keys=True))
            if current == self.source_fingerprint:
                return copy.deepcopy(self.snapshot)
            self.source_fingerprint = current
            fingerprint = []
            loaded = {}
            for key, source in candidates.items():
                try:
                    stat = Path(key).stat()
                    stamp = (stat.st_ino, stat.st_mtime_ns, stat.st_size)
                    fingerprint.append((key, stamp))
                    cached = self.cache.get(key)
                    if cached and cached[0] == stamp:
                        entry = copy.deepcopy(cached[1])
                    else:
                        entry = self.load(key, source)
                        self.cache[key] = (stamp, copy.deepcopy(entry))
                    if entry:
                        self.project(entry)
                        loaded[entry["session"]["id"]] = entry
                    elif key == source["path"] or (cached and cached[1]):
                        issues.append({"path": key, "message": "No readable Ask session header; this file cannot be interpreted as event evidence.", "severity": "warning"})
                except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
                    issues.append({"path": key, "message": str(error), "severity": "warning"})
            self.cache = {k: v for k, v in self.cache.items() if k in candidates}
            # Recover retained child conversations from complete Record receipts.
            # Never open an artifact's recorded original path. Bytes come from the
            # selected archive, then Ask verifies their private snapshot itself.
            originals = {entry["original"] for entry in loaded.values()}
            retained_aliases = {}
            embedded_bytes = 0
            for entry in list(loaded.values()):
                s = entry["session"]
                if s["kind"] != "process" or not s["complete"]:
                    continue
                for i, artifact in enumerate(entry["intent"].get("artifacts") or []):
                    if artifact.get("phase") != "session":
                        continue
                    original = btext(artifact.get("path"))
                    if original in originals:
                        continue
                    try:
                        data = self.observed_stream(entry, "artifact:" + str(i))
                        digest = hashlib.sha256(data).hexdigest()
                        identical = next((oid for oid, other in loaded.items() if other["session"]["sha256"] == digest), None)
                        if identical:
                            retained_aliases[original] = identical
                            originals.add(original)
                            continue
                        embedded_bytes += len(data)
                        if embedded_bytes > self.max_embedded_bytes:
                            raise ValueError("Embedded session limit reached; increase --max-embedded-mib to expand more retained conversations.")
                        virtual = "embedded:" + s["id"] + ":" + str(i)
                        child = self.load(virtual, entry["source"], data, original, True)
                        if child:
                            child["container"] = s["id"]
                            self.project(child)
                            loaded[child["session"]["id"]] = child
                            originals.add(original)
                    except (OSError, ValueError, KeyError, TypeError) as error:
                        issues.append({"path": s["path"], "message": "Retained session: " + str(error), "severity": "warning"})
            self.sessions = loaded
            self.aliases = {entry["original"]: sid for sid, entry in loaded.items()}
            self.aliases.update(retained_aliases)
            # A relocated evidence directory is resolved from the index's own
            # original recordings path, only to files already selected above.
            for sid, entry in loaded.items():
                if entry["session"]["kind"] != "agent":
                    continue
                for body in entry["index"]:
                    if body.get("phase") != "start" or not body.get("inputs"):
                        continue
                    old = str(Path(body["inputs"]).parent)
                    new = str(Path(entry["key"]).parent)
                    for oid, other in loaded.items():
                        try:
                            rel = Path(other["key"]).relative_to(new)
                            self.aliases[str(Path(old) / rel)] = oid
                        except ValueError:
                            pass
                    if "/recordings/" in old and "/recordings/" in new:
                        oldroot, newroot = old.split("/recordings/", 1)[0], new.split("/recordings/", 1)[0]
                        for oid, other in loaded.items():
                            try:
                                rel = Path(other["key"]).relative_to(newroot)
                                self.aliases[str(Path(oldroot) / rel)] = oid
                            except ValueError:
                                pass
            edges = []
            for sid, entry in loaded.items():
                s = entry["session"]
                if s["kind"] != "agent":
                    continue
                for body in entry["index"]:
                    phase = body.get("phase")
                    refs = [body.get("inputs"), body.get("outputs"), body.get("path"), body.get("session")]
                    for ref in filter(None, refs):
                        target = self.aliases.get(ref)
                        if target:
                            loaded[target]["session"]["laneId"] = sid
                        elif phase in ("process", "session"):
                            issues.append({"path": s["path"], "message": "Recorded link is not present in the selected evidence: " + str(ref), "severity": "info"})
                    parent = self.aliases.get(body.get("parent", ""))
                    if phase == "start" and parent:
                        edges.append({"fromSession": parent, "to": sid, "kind": "child", "label": "Recorded child", "verified": s["verification"] == "verified", "fromTime": loaded[parent]["session"]["start"], "toTime": s["start"], "eventIds": [e["id"] for e in entry["projected"] if e["type"] == "start"]})
                    for path_field, digest_field in [("inputs", "inputs_sha256"), ("outputs", "outputs_sha256"), ("path", "sha256")]:
                        expected = body.get(digest_field)
                        ref = body.get(path_field)
                        if not expected or not ref:
                            continue
                        target = self.aliases.get(ref)
                        if target is None:
                            s.update(state="incomplete", complete=False)
                            issues.append({"path": s["path"], "message": "Committed receipt is missing from selected evidence: " + str(ref), "severity": "warning"})
                        elif "sha256:" + loaded[target]["session"]["sha256"] != expected:
                            s.update(state="invalid", verification="invalid", complete=False)
                            loaded[target]["session"].update(state="invalid", verification="invalid", complete=False)
                            issues.append({"path": s["path"], "message": "Index commitment does not match the selected receipt: " + str(ref), "severity": "error"})
            for sid, entry in loaded.items():
                if entry.get("container"):
                    entry["session"]["laneId"] = loaded[entry["container"]]["session"]["laneId"]
            lane_sessions = {}
            events = []
            for sid, entry in loaded.items():
                s = entry["session"]
                lane_sessions.setdefault(s["laneId"], []).append(sid)
                for event in entry["projected"]:
                    event["laneId"] = s["laneId"]
                    events.append(event)
                if s["problem"]:
                    issues.append({"path": s["path"], "message": s["problem"], "severity": "warning" if s["verification"] == "invalid" else "info"})
            keys = {entry["session"]["sessionKey"]: sid for sid, entry in loaded.items() if entry["session"]["sessionKey"]}
            for sid, entry in loaded.items():
                s = entry["session"]
                for field, relation in [("parentKey", "compaction"), ("summaryKey", "summary")]:
                    other = keys.get(s[field])
                    if other and other != sid:
                        edges.append({"fromSession": other, "to": s["laneId"], "kind": relation, "label": "Recorded " + relation, "verified": s["verification"] == "verified", "fromTime": loaded[other]["session"]["end"], "toTime": s["start"], "eventIds": [e["id"] for e in entry["projected"] if e["type"] == "context"]})
            producers = {}
            for sid, entry in loaded.items():
                s = entry["session"]
                if not s["complete"] or s["verification"] != "verified":
                    continue
                for stream in s["streams"]:
                    if stream.get("phase") == "output" and stream["bytes"] > 0 and stream.get("sha256"):
                        producers.setdefault(stream["sha256"], []).append((sid, stream))
            matched = set()
            for entry in loaded.values():
                s = entry["session"]
                if not s["complete"] or s["verification"] != "verified":
                    continue
                for stream in s["streams"]:
                    if stream.get("phase") != "input":
                        continue
                    for producer, output in producers.get(stream.get("sha256"), []):
                        origin = loaded[producer]["session"]
                        key = (origin["laneId"], s["laneId"], stream["sha256"])
                        if origin["laneId"] == s["laneId"] or key in matched:
                            continue
                        matched.add(key)
                        if len(matched) > 200:
                            continue
                        edges.append({"fromSession": producer, "to": s["laneId"], "kind": "artifact", "label": "Matching retained bytes · " + str(output.get("filename", "artifact")), "verified": True, "fromTime": origin["end"], "toTime": s["start"], "sha256": stream["sha256"], "eventIds": [e["id"] for selected in (loaded[producer], entry) for e in selected["projected"] if e["type"] == "result"]})
            if len(matched) > 200:
                issues.append({"path": "", "message": "More than 200 matching-artifact relationships; narrow the selected sources to inspect additional links.", "severity": "warning"})
            for edge in edges:
                edge["from"] = loaded[edge.pop("fromSession")]["session"]["laneId"]
                edge["verified"] = edge["verified"] and all(loaded[lid]["session"]["verification"] == "verified" for lid in (edge["from"], edge["to"]))
            lanes = []
            for lid, ids in lane_sessions.items():
                s = loaded[lid]["session"]
                starts = [loaded[i]["session"]["start"] for i in ids]
                ends = [loaded[i]["session"]["end"] for i in ids]
                lanes.append({"id": lid, "title": s["title"], "source": s["source"], "kind": s["kind"],
                              "state": s["state"], "verification": s["verification"], "sessionIds": ids,
                              "start": min(starts), "end": s["end"] or max(filter(None, ends), default=None),
                              "parentId": next((e["from"] for e in edges if e["kind"] == "child" and e["to"] == lid), None)})
            # Repeated invocations of the same expert must remain distinguishable.
            titles = [lane["title"] for lane in lanes]
            for lane in lanes:
                if titles.count(lane["title"]) > 1:
                    path = loaded[lane["id"]]["session"]["path"]
                    root = Path(path.split("/recordings/", 1)[0]).name if "/recordings/" in path else "run"
                    lane["title"] += " · " + root + " · " + lane["id"][-6:]
            # Wall clocks are displayed as recorded. Sequence is never inferred
            # from stream interleaving or a timestamp in a different session.
            # Merge session-local sequences instead of globally sorting every
            # event by wall time, which would reverse a session after clock skew.
            by_session = {}
            for event in events:
                by_session.setdefault(event["sessionId"], []).append(event)
            heap, events = [], []
            for sid, rows in by_session.items():
                rows.sort(key=lambda e: e["seq"])
                heapq.heappush(heap, (rows[0]["time"], sid, 0))
            while heap:
                _, sid, index = heapq.heappop(heap)
                event = by_session[sid][index]
                event["order"] = len(events)
                events.append(event)
                if index + 1 < len(by_session[sid]):
                    heapq.heappush(heap, (by_session[sid][index + 1]["time"], sid, index + 1))
            for entry in loaded.values():
                times = [millis(e.get("time")) for e in entry["events"]]
                if any(b < a for a, b in zip(times, times[1:])):
                    issues.append({"path": entry["session"]["path"], "message": "Recorded clock moved backward. Inspect sequence numbers for within-session order.", "severity": "warning"})
            fingerprint = (fingerprint, json.dumps(issues, sort_keys=True))
            if fingerprint != self.fingerprint:
                self.revision += 1
                self.fingerprint = fingerprint
            snapshot = self.empty()
            snapshot.update(lanes=sorted(lanes, key=lambda l: (l["start"], l["id"])), sessions=[e["session"] for e in loaded.values()], events=events,
                            edges=edges, issues=issues, revision=self.revision)
            self.snapshot = snapshot
            retained = {entry[name] for entry in loaded.values() for name in ("file", "public", "originalFile")}
            for file in self.directory.iterdir():
                if file not in retained:
                    file.unlink()
            return copy.deepcopy(snapshot)

    def stream(self, sid, name):
        with self.lock:
            entry = self.sessions.get(sid)
            if not entry or name not in {s["name"] for s in entry["session"]["streams"]}:
                raise ValueError("Unknown selected stream.")
            s = entry["session"]
            verified = s["verification"] == "verified" and s["complete"]
            if verified:
                code, payload, error = self.command([self.record, "replay", "-ask", self.ask, "-f", entry["file"], "-stream", name])
                if code:
                    raise ValueError(error)
            else:
                payload = self.observed_stream(entry, name)
            return {"base64": base64.b64encode(payload).decode(), "bytes": len(payload), "verified": verified, "complete": s["complete"]}

    def detail(self, eid):
        with self.lock:
            sid, _, number = eid.rpartition(":")
            entry = self.sessions.get(sid)
            if not entry:
                raise ValueError("The selected session is no longer present.")
            event = next((e for e in entry["events"] if str(e.get("seq")) == number), None)
            if not event:
                raise ValueError("Unknown event.")
            result = copy.deepcopy(event)
            result["verification"] = entry["session"]["verification"]
            if event.get("type") == "request":
                code, output, diagnostic = self.command([self.ask, "replay", "-step", number, entry["file"]])
                if code == 0:
                    result["normalizedRequest"] = json.loads(output)
                else:
                    result["requestProblem"] = diagnostic
            return result

    def export(self):
        with self.lock:
            snapshot = copy.deepcopy(self.snapshot)
            snapshot["live"] = False
            streams, details, originals = {}, {}, {}
            for sid, entry in self.sessions.items():
                originals[sid] = base64.b64encode(entry["originalFile"].read_bytes()).decode()
                for row in entry["session"]["streams"]:
                    try:
                        streams[sid + "/" + row["name"]] = self.stream(sid, row["name"])
                    except ValueError as error:
                        streams[sid + "/" + row["name"]] = {"error": str(error)}
                for event in entry["projected"]:
                    details[event["id"]] = self.detail(event["id"])
            return page(snapshot, {"streams": streams, "details": details, "originals": originals})


def javascript_json(value):
    return json.dumps(value, ensure_ascii=True, separators=(",", ":")).replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")


def page(snapshot, offline=None, prefix=""):
    driver = r"""
<script>
(()=>{'use strict';
const initial=__DATA__, offline=__OFFLINE__, prefix=__PREFIX__;
const publish=data=>{window.__BENCH_TRACE_DATA__=data;document.dispatchEvent(new CustomEvent('bench:snapshot',{detail:data}));};
const get=async path=>{const r=await fetch(prefix+path,{cache:'no-store'});const d=await r.json();if(!r.ok)throw Error(d.error||'Unable to read evidence');return d;};
const file=(name,b64,type)=>{const bytes=Uint8Array.from(atob(b64),c=>c.charCodeAt(0));return URL.createObjectURL(new Blob([bytes],{type}));};
window.BenchTrace={
readStream:async(id,name)=>{if(!offline)return get('api/stream/'+encodeURIComponent(id)+'/'+encodeURIComponent(name));const d=offline.streams[id+'/'+name];if(!d||d.error)throw Error(d?.error||'No retained stream');return d;},
readEvent:async id=>{if(!offline)return get('api/event/'+encodeURIComponent(id));if(!offline.details[id])throw Error('No retained event');return offline.details[id];},
downloadSession:id=>offline?(offline.originals[id]?file('session.jsonl',offline.originals[id],'application/x-ndjson'):null):prefix+'api/session/'+encodeURIComponent(id),
exportSnapshot:()=>{const a=document.createElement('a');a.href=offline?URL.createObjectURL(new Blob(['<!DOCTYPE html>\n'+document.documentElement.outerHTML],{type:'text/html'})):prefix+'api/export';a.download='bench-trace.html';a.click();}
};
publish(initial);
document.dispatchEvent(new CustomEvent('bench:connection',{detail:{connected:!offline,message:offline?'Offline snapshot':'Following recorded evidence'}}));
if(!offline){const feed=new EventSource(prefix+'api/live');feed.addEventListener('snapshot',e=>{try{publish(JSON.parse(e.data));}catch(error){document.dispatchEvent(new CustomEvent('bench:connection',{detail:{connected:false,message:String(error)}}));}});feed.onopen=()=>document.dispatchEvent(new CustomEvent('bench:connection',{detail:{connected:true,message:'Connected to local evidence'}}));feed.onerror=()=>document.dispatchEvent(new CustomEvent('bench:connection',{detail:{connected:false,message:'Disconnected · retrying local evidence feed'}}));}
})();
</script>
"""
    substitutions = {"DATA": snapshot, "OFFLINE": offline, "PREFIX": prefix}
    driver = re.sub(r"__(DATA|OFFLINE|PREFIX)__", lambda match: javascript_json(substitutions[match[1]]), driver)
    before, closing, after = HTML.rpartition("</body>")
    if not closing:
        raise ValueError("Embedded HTML has no body closing tag.")
    return before + driver + "\n" + closing + after


class TraceServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address, archive, interval):
        super().__init__(address, Handler)
        self.archive, self.interval = archive, interval
        self.token = secrets.token_urlsafe(24)
        self.prefix = "/" + self.token + "/"
        self.stop = threading.Event()
        self.condition = threading.Condition()

    def collect(self):
        while not self.stop.wait(self.interval):
            try:
                previous = self.archive.revision
                self.archive.refresh()
                if self.archive.revision != previous:
                    with self.condition:
                        self.condition.notify_all()
            except Exception as error:
                print("trace: snapshot: " + str(error), file=sys.stderr)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def send(self, payload, content_type="application/json", code=200, filename=None):
        if not isinstance(payload, bytes):
            payload = payload.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Referrer-Policy", "no-referrer")
        self.send_header("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
        if filename:
            self.send_header("Content-Disposition", 'attachment; filename="' + filename + '"')
        self.end_headers()
        self.wfile.write(payload)

    def json(self, payload, code=200):
        self.send(json.dumps(payload, ensure_ascii=True), code=code)

    def do_GET(self):
        self.connection.settimeout(15)
        server = self.server
        port = server.server_address[1]
        hosts = {"127.0.0.1:" + str(port), "localhost:" + str(port)}
        origin = self.headers.get("Origin")
        if self.headers.get("Host") not in hosts or (origin and origin not in {"http://" + h for h in hosts}):
            return self.json({"error": "Only the local browser origin can read this evidence."}, 403)
        path = urlsplit(self.path).path
        if not path.startswith(server.prefix):
            return self.json({"error": "Open the complete local URL printed by trace.py."}, 404)
        route = path[len(server.prefix):]
        archive = server.archive
        try:
            if not route:
                with archive.lock:
                    return self.send(page(archive.snapshot, prefix=server.prefix), "text/html; charset=utf-8")
            if route == "api/snapshot":
                with archive.lock:
                    return self.json(archive.snapshot)
            if route == "api/live":
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Cache-Control", "no-store")
                self.send_header("X-Content-Type-Options", "nosniff")
                self.end_headers()
                revision = -1
                while not server.stop.is_set():
                    with archive.lock:
                        snapshot = archive.snapshot
                        if snapshot["revision"] != revision:
                            payload = json.dumps(snapshot, ensure_ascii=True, separators=(",", ":"))
                            message = ("event: snapshot\ndata: " + payload + "\n\n").encode()
                            revision = snapshot["revision"]
                        else:
                            message = b": connected\n\n"
                    self.wfile.write(message)
                    self.wfile.flush()
                    with server.condition:
                        server.condition.wait(timeout=10)
                return
            if route.startswith("api/stream/"):
                parts = route[len("api/stream/"):].split("/", 1)
                if len(parts) != 2:
                    raise ValueError("Select a stream.")
                return self.json(archive.stream(*map(unquote, parts)))
            if route.startswith("api/event/"):
                return self.json(archive.detail(unquote(route[len("api/event/"):])))
            if route.startswith("api/session/"):
                with archive.lock:
                    entry = archive.sessions.get(route[len("api/session/"):])
                    if not entry:
                        raise ValueError("Unknown selected session.")
                    return self.send(entry["originalFile"].read_bytes(), "application/x-ndjson", filename="session.jsonl")
            if route == "api/export":
                return self.send(archive.export(), "text/html; charset=utf-8", filename="bench-trace.html")
            self.json({"error": "Unknown read-only route."}, 404)
        except (ValueError, OSError, KeyError) as error:
            if isinstance(error, (BrokenPipeError, ConnectionResetError)):
                return
            self.json({"error": str(error)}, 400)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("targets", nargs="+", help="Ask/Record file or evidence directory; optional NAME=PATH labels; repeat for multiple agents")
    parser.add_argument("--ask", default=os.environ.get("TRACE_ASK", "ask"), help="public Ask executable (no model connection needed)")
    parser.add_argument("--record", default=os.environ.get("TRACE_RECORD", "record"), help="public Record executable")
    parser.add_argument("--port", type=int, default=0, help="loopback port; 0 selects a free port")
    parser.add_argument("--interval", type=float, default=1, help="filesystem snapshot interval in seconds")
    parser.add_argument("--no-open", action="store_true", help="print the URL without opening a browser")
    parser.add_argument("--export", type=Path, help="write one standalone offline HTML snapshot instead of serving")
    parser.add_argument("--json", action="store_true", help="emit a snapshot to stdout and exit (Unix filter)")
    parser.add_argument("--max-file-mib", type=int, default=256, help="explicit per-session read bound; oversized files are reported, never truncated")
    parser.add_argument("--max-embedded-mib", type=int, default=256, help="total expanded child-session bytes")
    parser.add_argument("--max-sessions", type=int, default=2000)
    parser.add_argument("--command-timeout", type=float, default=30)
    args = parser.parse_args(argv)
    if min(args.interval, args.max_file_mib, args.max_embedded_mib, args.max_sessions, args.command_timeout) <= 0:
        parser.error("limits and interval must be positive")
    ask, record = shutil.which(args.ask), shutil.which(args.record)
    if not ask or not record:
        parser.error("Ask and Record must be installed; select them with --ask and --record or TRACE_ASK/TRACE_RECORD.")
    archive = Archive(args.targets, ask, record, args.max_file_mib * 1024 * 1024,
                      args.max_sessions, args.command_timeout, args.max_embedded_mib * 1024 * 1024)
    server = None
    try:
        snapshot = archive.refresh()
        if args.json:
            print(json.dumps(snapshot, ensure_ascii=True))
        elif args.export:
            with args.export.open("x", encoding="utf-8") as output:
                output.write(archive.export())
            print(str(args.export.absolute()))
        else:
            server = TraceServer(("127.0.0.1", args.port), archive, args.interval)
            threading.Thread(target=server.collect, daemon=True).start()
            url = "http://127.0.0.1:" + str(server.server_address[1]) + server.prefix
            print(url, flush=True)
            print("Read-only local evidence. Ctrl-C stops the browser server.", file=sys.stderr)
            if not args.no_open:
                webbrowser.open(url)
            server.serve_forever(poll_interval=0.25)
    except KeyboardInterrupt:
        pass
    except (OSError, ValueError) as error:
        print("trace: " + str(error), file=sys.stderr)
        return 1
    finally:
        if server:
            server.stop.set()
            with server.condition:
                server.condition.notify_all()
            server.server_close()
        archive.close()
    return 0


# The checked Frontend worker supplies this one embedded document. No runtime
# companion file is read by the application.
HTML = r'''<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<title>Bench Trace — Event browser</title>
<style>
:root{--paper:#f3efe7;--panel:#fbf8f2;--ink:#172426;--muted:#566463;--line:#d5cfc4;--teal:#087b76;--blue:#28649b;--orange:#bd5918;--red:#ae2933;--lane-h:56px;--lane-label:130px;--shadow:0 10px 28px #17232512}
*{box-sizing:border-box}html,body{margin:0;min-height:100%;background:var(--paper);color:var(--ink);font:14px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif}button,input,select{font:inherit;color:inherit}button,select,input[type=search]{border:1px solid var(--line);background:#fffdf8;border-radius:7px;min-height:36px}button{padding:.45rem .7rem;cursor:pointer}button:hover:not(:disabled){border-color:#869493}button:disabled{opacity:.42;cursor:not-allowed}:focus-visible{outline:3px solid #159b91;outline-offset:2px}.sr-only{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}
.topbar{min-height:62px;display:flex;align-items:center;gap:16px;padding:10px 18px;background:var(--ink);color:white;position:sticky;top:0;z-index:30}.brand{display:flex;align-items:baseline;gap:9px;white-space:nowrap}.brand strong{font-size:20px}.brand span,.connection{color:#c2ccca;font-size:12px}.connection{display:flex;align-items:center;gap:7px}.dot{width:8px;height:8px;border-radius:50%;background:#899593}.connected .dot{background:#53c6a4}.mode{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:#d7dfdd;border:1px solid #566563;border-radius:99px;padding:3px 8px}.controls{display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin-left:auto}.controls button,.controls select{background:#263638;color:#fff;border-color:#4a5b5d}.controls button[aria-pressed=true]{background:#08746e}.layout{display:grid;grid-template-columns:225px minmax(420px,1fr) 340px;min-height:calc(100vh - 62px)}.sidebar,.inspector{background:var(--panel);min-width:0}.sidebar{border-right:1px solid var(--line);padding:18px 14px}.inspector{border-left:1px solid var(--line);overflow:auto}.main{padding:18px;min-width:0;overflow:hidden}
.eyebrow,.section-title{font-size:11px;letter-spacing:.1em;text-transform:uppercase;font-weight:750;color:var(--muted)}.section-title{margin:17px 0 7px}.search,.agent-select{width:100%;padding:7px 9px}.filter-toggle{display:none;width:100%;justify-content:space-between;margin:10px 0}.filter-list,.source-list{display:grid;gap:5px}.filter-list label{display:flex;gap:8px;align-items:center}.filter-actions{display:flex;gap:6px;margin-top:10px}.filter-actions button{font-size:12px;min-height:30px}.source{padding:9px;border:1px solid var(--line);border-radius:7px;background:#fffdf8}.source small{display:block;color:var(--muted);overflow-wrap:anywhere}.count{background:#e3ded5;border-radius:10px;padding:1px 6px;font-size:11px}.notice{border:1px solid #dec394;background:#fff7e4;color:#654b20;border-radius:7px;padding:9px 11px;margin:0 0 12px}.notice.error{border-color:#e2a9ae;background:#fff0f1;color:#7d2028}.notice.success{border-color:#a8cfb6;background:#eff8f1;color:#28583a}
.empty{text-align:center;padding:50px 18px;border:1px dashed #bcb4a7;border-radius:10px;background:#faf6ef}.empty h1{font-size:19px;margin:0 0 7px}.empty p{max-width:490px;margin:0 auto 18px;color:var(--muted)}.view-head{display:flex;justify-content:space-between;gap:10px;align-items:flex-start}.view-head h1{font-size:19px;margin:0}.view-head p{font-size:12px;color:var(--muted);margin:3px 0}.results{white-space:nowrap;color:var(--muted)}.legend{display:flex;gap:12px;flex-wrap:wrap;color:var(--muted);font-size:12px;margin:10px 0}.key{display:flex;gap:5px;align-items:center}.swatch{width:9px;height:9px;border-radius:2px;background:var(--blue)}.swatch.action{background:var(--orange)}.swatch.check{background:var(--teal)}.swatch.error{background:var(--red)}
.timeline-shell{border:1px solid var(--line);border-radius:10px;background:#fffdf9;box-shadow:var(--shadow);overflow:auto}.timeline-inner{min-width:520px}.ruler{height:36px;position:relative;margin-left:var(--lane-label);border-bottom:1px solid var(--line);background:repeating-linear-gradient(90deg,transparent 0,transparent calc(20% - 1px),#ddd7cd calc(20% - 1px),#ddd7cd 20%)}.ruler span{white-space:nowrap;position:absolute;top:8px;transform:translateX(-50%);font-size:10px;color:var(--muted)}.lanes{position:relative}.lane{height:var(--lane-h);display:grid;grid-template-columns:var(--lane-label) 1fr;border-bottom:1px solid #e6e1d8}.lane-label{padding:9px 10px;border-right:1px solid var(--line);overflow:hidden}.lane-label strong,.lane-label small{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.lane-label small{font-size:10px;color:var(--muted)}.lane-track{position:relative;background:linear-gradient(90deg,#d8d3ca66 1px,transparent 1px);background-size:20% 100%}.event-mark{position:absolute;top:13px;height:28px;min-width:9px;border:0;border-radius:4px;background:var(--blue);padding:0;box-shadow:0 0 0 1px white,0 1px 3px #0003;z-index:3}.event-mark[data-type=action],.event-mark[data-type=artifact]{background:var(--orange)}.event-mark[data-type=check],.event-mark[data-type=result]{background:var(--teal)}.event-mark[data-type=error],.event-mark[data-status=failed],.event-mark[data-status=invalid]{background:var(--red)}.event-mark.optional{height:14px;min-width:7px;border-radius:50%}.event-mark.future{opacity:.38}.event-mark.selected{outline:3px solid var(--ink);outline-offset:2px;z-index:7}.edge-layer{position:absolute;left:var(--lane-label);top:0;width:calc(100% - var(--lane-label));height:100%;pointer-events:none;z-index:2;overflow:visible}.edge{fill:none;stroke:#087b76;stroke-width:2}.edge.unverified{stroke:#9b6b3c;stroke-dasharray:5 4}.edge.artifact{stroke:#7d4f9d;stroke-dasharray:2 4}.edge-label{font-size:9px;fill:#334746;paint-order:stroke;stroke:#fffdf9;stroke-width:3px}.playhead{position:absolute;top:0;bottom:0;width:2px;background:var(--ink);pointer-events:none;z-index:8}.playhead:before{content:"";position:absolute;left:-4px;border-left:5px solid transparent;border-right:5px solid transparent;border-top:7px solid var(--ink)}.scrub-row{display:grid;grid-template-columns:70px 1fr 70px;gap:8px;align-items:center;font-size:11px;color:var(--muted)}.scrub{width:100%;accent-color:var(--teal)}
.zoom-row{display:flex;justify-content:space-between;align-items:center;gap:8px;margin:8px 0;font-size:11px;color:var(--muted)}.zoom-buttons{display:flex;gap:4px}.zoom-buttons button{min-height:30px;padding:3px 8px}.zoom-buttons button[aria-pressed=true]{background:#d8ece9;border-color:var(--teal)}.feed{margin-top:18px}.feed-list{border-top:1px solid var(--line)}.event-row{width:100%;display:grid;grid-template-columns:75px 72px minmax(120px,1fr) auto;gap:10px;align-items:center;text-align:left;border:0;border-bottom:1px solid var(--line);border-radius:0;background:transparent;padding:9px 5px}.event-row:hover,.event-row.selected{background:#fffdf8}.event-row.selected{box-shadow:inset 3px 0 var(--teal)}.event-row.future{background:#eae6dd}.event-row.future .event-title strong{font-weight:450}.event-row time,.status{font-size:11px;color:var(--muted)}.pill{font-size:10px;text-transform:uppercase;letter-spacing:.05em;border:1px solid var(--line);border-radius:99px;padding:2px 6px;width:max-content}.event-title{min-width:0}.event-title strong,.event-title small{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.event-title small{color:var(--muted)}.feed-pager{display:flex;justify-content:center;align-items:center;gap:10px;margin:12px}.feed-pager span{font-size:12px;color:var(--muted)}
.inspector-head{padding:16px;border-bottom:1px solid var(--line);position:sticky;top:0;background:var(--panel);z-index:3}.inspector-head h2{font-size:17px;margin:3px 38px 0 0}.close-details{display:none;position:absolute;right:12px;top:12px}.tabs{display:flex;padding:10px 12px 0;gap:2px;border-bottom:1px solid var(--line)}.tabs button{border:0;border-radius:5px 5px 0 0;background:transparent}.tabs button[aria-selected=true]{background:#e7e2d9;color:#075f5a}.tab-panel{padding:16px}.meta{display:grid;grid-template-columns:95px 1fr;margin:0}.meta dt,.meta dd{padding:7px 0;border-bottom:1px solid #e6e1d8;margin:0;min-width:0}.meta dt{color:var(--muted)}.meta dd{overflow-wrap:anywhere}.copy-text,.raw{white-space:pre-wrap;overflow-wrap:anywhere;background:#ebe7df;border:1px solid #d8d1c7;border-radius:7px;padding:10px;max-height:330px;overflow:auto;font:12px/1.55 ui-monospace,monospace}.stream-card{border:1px solid var(--line);border-radius:8px;padding:10px;margin-bottom:9px;background:#fffdf8}.stream-card header{display:flex;gap:8px;align-items:center}.stream-card header small{margin-left:auto;color:var(--muted)}.stream-actions{display:flex;gap:6px;flex-wrap:wrap;margin:8px 0}.mobile-details{display:none}
@media(max-width:980px){.layout{grid-template-columns:205px minmax(0,1fr)}.inspector{position:fixed;right:0;top:62px;bottom:0;width:min(390px,94vw);z-index:25;box-shadow:-12px 0 30px #0002;transform:translateX(105%);transition:transform .18s}.inspector:not(.open){display:none}.inspector.open{transform:none}.mobile-details,.close-details{display:block}}
@media(max-width:650px){:root{--lane-label:88px}.topbar{position:relative;padding:10px 12px;gap:8px;flex-wrap:wrap}.brand span{display:none}.connection,.mode{font-size:10px;padding:2px 5px}.connection{display:flex}.mode{display:inline-block}.controls{margin-left:0;width:100%}.layout{display:block;min-height:0}.sidebar{border-right:0;border-bottom:1px solid var(--line);padding:10px 12px}.filter-toggle{display:flex}.filter-body[hidden]{display:none}.main{padding:12px}.inspector{top:0}.event-row{grid-template-columns:61px 57px minmax(80px,1fr)}.event-row .status{display:none}.scrub-row{grid-template-columns:55px 1fr 55px}.controls #export-button{margin-left:auto}.timeline-inner{min-width:100%}.timeline-shell.needs-scroll .timeline-inner{min-width:430px}.zoom-row{align-items:flex-start}.zoom-buttons{flex-wrap:wrap;justify-content:flex-end}.lane-label{padding-left:6px;padding-right:6px}}

.narration-toggle{white-space:nowrap}.narration-panel{scroll-margin-top:80px;margin:14px 0 18px;border:1px solid var(--line);border-left:4px solid var(--teal);border-radius:10px;background:#fffdf9;box-shadow:var(--shadow);padding:14px}.narration-panel[hidden]{display:none}.narration-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}.narration-head h2{font-size:18px;margin:1px 0 3px}.narration-head p,.narration-help,.narration-meta,.narration-speech-state{color:var(--muted);font-size:12px;margin:3px 0}.narration-controls{display:flex;flex-wrap:wrap;align-items:end;gap:8px;margin:12px 0}.narration-controls label{display:grid;gap:3px;font-size:11px;font-weight:700;color:var(--muted)}.narration-controls select{padding:6px 8px}.narration-status{margin:10px 0}.narration-title{font-size:16px;margin:12px 0 2px}.narration-summary{font-size:14px;margin:4px 0 12px;max-width:76ch}.narration-notes{margin:8px 0;padding-left:20px;color:var(--muted)}.narration-list{max-height:380px;overflow-y:auto;overscroll-behavior:contain;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}.narration-chapter{padding:12px 4px;border-bottom:1px solid #e5dfd5}.narration-chapter:last-child{border-bottom:0}.narration-chapter h3{font-size:14px;margin:0 0 4px}.narration-chapter p{margin:0;max-width:78ch;white-space:pre-wrap}.narration-evidence{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}.narration-evidence button{min-height:30px;padding:3px 8px;font-size:12px}.narration-more{display:block;margin:10px auto 0}.narration-empty{padding:18px 0;color:var(--muted)}@media(max-width:480px){.narration-head{display:block}.narration-toggle{margin-top:8px}.narration-controls{align-items:stretch}.narration-controls label,.narration-controls select{width:100%}.narration-controls button{flex:1}.narration-list{max-height:55vh}}
@media(prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition:none!important;animation:none!important}}
</style>
</head>
<body>
<header class="topbar">
 <div class="brand"><strong>Bench Trace</strong><span>evidence event browser</span></div>
 <div class="connection" id="connection" role="status"><span class="dot"></span><span id="connection-label">Awaiting host</span></div>
 <span class="mode" id="mode-label">No recording</span>
 <div class="controls" aria-label="Replay controls">
  <button id="follow-button" type="button" aria-pressed="false">Follow live</button>
  <button id="prev-button" type="button" aria-label="Previous event">←</button>
  <button id="play-button" type="button" aria-label="Play replay" aria-pressed="false">▶</button>
  <button id="next-button" type="button" aria-label="Next event">→</button>
  <label class="sr-only" for="speed">Replay speed</label><select id="speed"><option value=".5">0.5×</option><option value="1" selected>1×</option><option value="2">2×</option><option value="4">4×</option></select>
  <button id="export-button" type="button">Export</button>
  <button id="details-button" class="mobile-details" type="button" aria-expanded="false" aria-controls="details-panel">Details</button>
 </div>
</header>
<div class="layout">
<aside class="sidebar" aria-label="Trace filters">
 <label class="eyebrow" for="search">Search evidence</label><input class="search" id="search" type="search" placeholder="Command, message, status…">
 <button class="filter-toggle" id="filter-toggle" type="button" aria-expanded="false" aria-controls="filter-body"><span>Filters</span><span aria-hidden="true">▾</span></button>
 <div class="filter-body" id="filter-body">
  <div class="section-title"><label for="agent-filter">Agent lane</label></div><select id="agent-filter" class="agent-select"><option value="">All agents</option></select>
  <div class="section-title"><label for="status-filter">Status</label></div>
  <select id="status-filter" class="agent-select"><option value="all">All statuses</option><option value="success">Successful</option><option value="failed">Failed / unknown</option><option value="open">Incomplete / open</option></select>
  <div class="section-title">Event kinds</div><div class="filter-list" id="kind-filters"></div>
  <div class="filter-actions"><button id="show-all" type="button">Show all</button><button id="reset-filters" type="button">Clear filters</button></div>
 </div>
 <div class="source-section"><div class="section-title">Sources</div><div class="source-list" id="sources"></div></div>
 <section class="issues" aria-labelledby="issues-title"><div class="section-title" id="issues-title">Issues <span class="count" id="issue-count">0</span></div><div id="issues-list"></div></section>
</aside>
<main class="main">
 <div id="state-banner" aria-live="polite"></div>
 <div id="empty" class="empty"><h1>No retained events loaded</h1><p id="empty-copy">Waiting for a selected Bench recording. Nothing here is evidence of a live run.</p><button id="demo-button" type="button">Load illustrative demo</button></div>
 <div id="workspace" hidden>
  <div class="view-head"><div><h1>Recorded timeline</h1><p>Recorded wall times · parentage and artifact links</p></div><div><span class="results" id="result-count">0 events</span> <button class="narration-toggle" id="narration-toggle" type="button" aria-expanded="false" aria-controls="narration-panel">Narrate</button></div></div>
  <div class="legend" aria-label="Event legend"><span class="key"><i class="swatch"></i>model / response</span><span class="key"><i class="swatch action"></i>action / artifact</span><span class="key"><i class="swatch check"></i>check / result</span><span class="key"><i class="swatch error"></i>failure</span></div>
  <div class="zoom-row" aria-label="Timeline zoom"><span id="visible-range">Visible range</span><span class="zoom-buttons"><button type="button" data-zoom="1">1×</button><button type="button" data-zoom="2">2×</button><button type="button" data-zoom="4">4×</button><button type="button" id="fit-all">Fit all</button></span></div>
  <section class="timeline-shell" aria-label="Synchronized event timeline"><div class="timeline-inner"><div class="ruler" id="ruler"></div><div class="lanes" id="lanes"></div></div></section>
  <div class="scrub-row"><span id="scrub-start">—</span><input class="scrub" id="scrubber" type="range" min="0" max="0" value="0" step="1" aria-label="Recorded event cursor"><span id="scrub-end">—</span></div>
  <section class="narration-panel" id="narration-panel" aria-labelledby="narration-heading" hidden>
   <div class="narration-head"><div><div class="eyebrow">Evidence narration</div><h2 id="narration-heading">Timeline narration</h2><p>Locally written from the retained recording.</p></div></div>
   <div class="narration-controls">
    <label for="narration-scope">Narration range<select id="narration-scope"><option value="cursor" selected>Through cursor</option><option value="all">Whole recording — includes later events</option></select></label>
    <button id="narration-download" type="button" disabled>Download narration</button>
    <button id="narration-speak" type="button" disabled>Read aloud</button>
    <button id="narration-stop" type="button" disabled>Stop reading</button>
   </div>
   <p class="narration-help">The agent lane filter limits narration. Search, event-kind, and status filters do not. “Through cursor” excludes later outcomes.</p>
   <div class="narration-status" id="narration-status" role="status" aria-live="polite"></div>
   <p class="narration-speech-state" id="narration-speech-state"></p>
   <div id="narration-content"></div>
  </section>
  <section class="feed" aria-labelledby="feed-title"><div class="view-head"><div><div class="eyebrow">Cursor-synchronized feed</div><h1 id="feed-title">Events through cursor; later events are dimmed</h1></div></div><div class="feed-list" id="feed-list"></div><div class="feed-pager"><button id="feed-earlier" type="button">Earlier</button><span id="feed-count" aria-live="polite"></span><button id="feed-later" type="button">Later</button></div></section>
 </div>
</main>
<aside class="inspector" id="details-panel" aria-label="Event details">
 <div class="inspector-head"><div class="eyebrow">Selected evidence</div><h2 id="detail-title">No event selected</h2><button class="close-details" id="close-details" type="button" aria-label="Close event details">×</button></div>
 <div class="tabs" role="tablist" aria-label="Event information">
  <button id="tab-details" role="tab" aria-selected="true" tabindex="0" aria-controls="panel-details">Details</button>
  <button id="tab-streams" role="tab" aria-selected="false" tabindex="-1" aria-controls="panel-streams">Streams &amp; artifacts</button>
  <button id="tab-raw" role="tab" aria-selected="false" tabindex="-1" aria-controls="panel-raw">Raw event</button>
 </div>
 <div class="tab-panel" id="panel-details" role="tabpanel" aria-labelledby="tab-details"></div>
 <div class="tab-panel" id="panel-streams" role="tabpanel" aria-labelledby="tab-streams" hidden></div>
 <div class="tab-panel" id="panel-raw" role="tabpanel" aria-labelledby="tab-raw" hidden></div>
</aside>
</div>
<div id="visual-fallback" data-visual-fallback hidden>Timeline graphics unavailable; retained events remain available in the list.</div>
<script id="page-tests" type="application/json">[{"name":"Open narration","steps":[{"action":"click","selector":"#demo-button"},{"action":"click","selector":"#narration-toggle"},{"action":"expectVisible","selector":"#narration-panel"},{"action":"expectText","selector":"#narration-status","value":"Narration ready"}]},{"name":"Open details","steps":[{"action":"click","selector":"#demo-button"},{"action":"click","selector":"#details-button"},{"action":"expectVisible","selector":"#details-panel"},{"action":"expectText","selector":"#detail-title","value":"Check accepted"}]},{"name":"Reveal filters","steps":[{"action":"click","selector":"#filter-toggle"},{"action":"expectVisible","selector":"#filter-body"},{"action":"click","selector":"#show-all"},{"action":"expectText","selector":"#result-count","value":"8 events"}]}]</script>
<script id="bench-narrator">
/* Evidence narration is a local projection, never a model or an action runner. */
globalThis.BenchNarrator = (() => {
  'use strict';
  const list = x => Array.isArray(x) ? x : [];
  const text = x => x == null ? '' : String(x);
  const excerpt = (x, n = 180) => {const s=text(x).replace(/\s+/g,' ').trim();return s.length>n?s.slice(0,n-1)+'…':s;};
  const quote = x => '“'+excerpt(x)+'”';
  const md = x => text(x).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/[\\`*_\[\]#]/g,'\\$&');
  const elapsed = (a,b) => {
    if(a.time==null||b.time==null||!Number.isFinite(Number(a.time))||!Number.isFinite(Number(b.time)))return '';
    const ms=Number(b.time)-Number(a.time);if(ms<0)return ' (the recorded clock moved backward)';
    return ' after '+(ms<1000?ms+' ms':ms<60000?(ms/1000).toFixed(1)+' seconds':(ms/60000).toFixed(1)+' minutes');
  };
  function build(snapshot={},options={}) {
    const all=list(snapshot.events).map((e,i)=>({e,i})).sort((a,b)=>{
      const ao=a.e.order,bo=b.e.order;return ao!=null&&bo!=null&&Number.isFinite(Number(ao))&&Number.isFinite(Number(bo))?Number(ao)-Number(bo)||a.i-b.i:a.i-b.i;
    }).map(x=>x.e);
    const index=new Map(all.map((e,i)=>[text(e.id),i]));
    const scope=options.scope==='all'?'all':'cursor', cutoff=scope==='all'?all.length-1:(index.get(text(options.cutoffId))??-1);
    const prefix=all.slice(0,cutoff+1), visibleIds=new Set(prefix.map(e=>text(e.id)));
    const laneId=text(options.laneId), rows=prefix.filter(e=>!laneId||text(e.laneId)===laneId);
    const lanes=new Map(list(snapshot.lanes).map(l=>[text(l.id),l]));
    const sessions=new Map(list(snapshot.sessions).map(s=>[text(s.id),s]));
    const name=id=>text(lanes.get(text(id))?.title)||'Unnamed lane';
    const chapters=[], pending=new Map(), pendingModels=new Map(), rejected=new Map(), rejectedCommands=new Map(), inputLanes=new Set();
    const counts={model:0,action:0,rejected:0,acceptedChecks:0,recovered:0,commandRecoveries:0,acceptedAgents:0,failedProcesses:0};
    function add(e,kind,title,body,refs=[e.id],extraLanes=[]) {
      const ids=[...new Set(refs.map(text))].filter(id=>visibleIds.has(id));if(!ids.length)return null;
      const bad=ids.some(id=>{const e=all[index.get(id)],s=sessions.get(text(e.sessionId));return ['invalid','unavailable'].includes(s?.verification);});
      const chapter={id:kind+':'+text(e.id),title,text:body+(bad?' The referenced archive is not verified; these observations are not established.':''),eventIds:ids,laneIds:[...new Set([text(e.laneId),...extraLanes.map(text)])],time:e.time,kind};
      chapters.push(chapter);return chapter;
    }
    for(const e of rows) {
      const who=name(e.laneId),sid=text(e.sessionId),d=e.details||{},session=sessions.get(sid)||{};
      if(e.type==='start')add(e,'start','Work began',who+' began a recorded invocation.');
      else if(e.type==='input'&&!e.stream&&!inputLanes.has(text(e.laneId))&&text(e.text).trim()) {
        inputLanes.add(text(e.laneId));add(e,'input','The recorded request',who+' received this input excerpt: '+quote(e.text)+'.');
      } else if(e.type==='model') {
        counts.model++;const chapter=add(e,'model','Waiting for a model response',who+' sent a request'+(d.model?' to '+quote(d.model):' to the model')+'. No response has been recorded for this request at this point.');
        pendingModels.set(sid,{event:e,chapter});
      } else if(e.type==='response') {
        const p=pendingModels.get(sid),partial=d.partial||e.status==='incomplete';
        const body=who+' received '+(partial?'a partial model response':'a model response')+(p?elapsed(p.event,e):'')+'.'+(text(e.text).trim()?' Recorded response excerpt: '+quote(e.text)+'.':'');
        if(p){p.chapter.title=partial?'Partial model response':'Model response received';p.chapter.text=body;p.chapter.eventIds.push(text(e.id));p.chapter.time=e.time;pendingModels.delete(sid);}
        else add(e,'response',partial?'Partial model response':'Model response received',body);
      } else if((e.type==='action'||e.type==='check')&&Array.isArray(d.argv)) {
        counts.action++;const isCheck=session.role==='verifier'||e.type==='check';
        const command=excerpt(d.argv.join(' ').replace(/\/(?:[^\/\s"'`]+\/)+([^\/\s"'`]+)/g,'…/$1'),160)||'unnamed command';
        const chapter=add(e,isCheck?'check-process':'action',isCheck?'A check started':'An action started',who+' started '+(isCheck?'a check command':'a command')+': '+quote(command)+'. No process result is recorded at this point.');
        const checkKey=isCheck&&d.cwd?text(e.laneId)+'\0'+text(d.cwd)+'\0'+JSON.stringify(d.argv):null;pending.set(sid,{event:e,chapter,isCheck,command,checkKey});
      } else if(e.type==='check') {
        const key=d.verifier_sha256&&d.directory?text(e.laneId)+'\0'+d.directory+'\0'+d.verifier_sha256:null;
        if(e.status==='failed'&&d.outcome==='rejected') {
          counts.rejected++;if(key)rejected.set(key,e);
          add(e,'rejection','A check rejected the candidate',who+' recorded a rejected check.'+(text(e.text).trim()?' Check output excerpt: '+quote(e.text)+'.':''));
        } else if(e.status==='accepted'&&d.outcome==='accepted') {
          counts.acceptedChecks++;const earlier=key&&rejected.get(key);
          if(earlier){counts.recovered++;add(e,'recovery','The same check later accepted',who+' later recorded acceptance from the same verifier in the same working directory. This follows its earlier rejection; the records do not by themselves identify which change caused the acceptance.',[earlier.id,e.id]);rejected.delete(key);}
          else add(e,'acceptance','A check accepted the candidate',who+' recorded an accepted check.'+(text(e.text).trim()?' Check output excerpt: '+quote(e.text)+'.':''));
        } else add(e,'uncertain','The check outcome is unresolved',who+' recorded a check with an unknown or broken outcome.'+(text(e.text).trim()?' Check output excerpt: '+quote(e.text)+'.':''));
      } else if(e.type==='result') {
        const p=pending.get(sid);
        if(p) {
          const code=d.exit,exit=Number.isInteger(code)?' Exit status: '+code+'.':'';
          const state=e.status==='complete'?'finished':e.status==='failed'?'failed':e.status==='interrupted'?'was interrupted':e.status==='unfinished'?'stopped unfinished':'has incomplete or uncertain recorded results';
          p.chapter.title=p.isCheck?'Check process '+state:'Action '+state;
          p.chapter.text=who+'’s '+(p.isCheck?'check command ':'command ')+state+elapsed(p.event,e)+'. Command excerpt: '+quote(p.command)+'.'+exit+(e.status==='incomplete'?' A complete process receipt is not available.':'');
          p.chapter.eventIds.push(text(e.id));p.chapter.time=e.time;pending.delete(sid);if(e.status==='failed')counts.failedProcesses++;
          if(p.checkKey&&e.status==='failed')rejectedCommands.set(p.checkKey,{start:p.event,result:e});
          if(p.checkKey&&e.status==='complete'&&code===0&&rejectedCommands.has(p.checkKey)){const earlier=rejectedCommands.get(p.checkKey);counts.commandRecoveries++;add(e,'recovery','The check command later succeeded',who+' later ran the same check command in the same directory and recorded exit status 0, following its earlier failure. The records establish this change in outcome, not which edit caused it.',[earlier.start.id,earlier.result.id,p.event.id,e.id]);rejectedCommands.delete(p.checkKey);}
        } else if(session.kind==='conversation')add(e,'conversation','Conversation '+text(e.status),who+' recorded its model conversation as '+text(e.status||'unknown')+'.');
      } else if(e.type==='end') {
        if(e.status==='accepted')counts.acceptedAgents++;
        const ending={accepted:'reached the runner’s accepted outcome',failed:'ended with a failed outcome',unfinished:'stopped before the runner accepted the work',interrupted:'was interrupted',incomplete:'has an incomplete terminal record',invalid:'has an invalid terminal record'}[e.status]||'ended with an unknown outcome';
        add(e,'outcome','Invocation '+text(e.status||'unknown'),who+' '+ending+'.'+(e.status==='accepted'?' This is the recorded runner/check result, not independent proof of task correctness.':''));
      } else if(e.type==='artifact'&&!e.stream)add(e,'artifact','Evidence capture began',who+' began retaining '+(e.title==='Retain inputs'?'the selected inputs':'selected output or conversation evidence')+'. Completion must be established by its receipt.');
      else if(e.type==='retry')add(e,'retry','A retry was recorded',who+' recorded a retry.'+(text(e.text).trim()?' Recorded detail: '+quote(e.text)+'.':''));
      else if(e.type==='error') {
        add(e,'interruption','Work was interrupted',who+' recorded '+quote(e.title||'an error')+'.');
        const p=pendingModels.get(sid);if(p){p.chapter.title='Model request interrupted';p.chapter.text=who+' requested a model response, then the conversation was interrupted'+elapsed(p.event,e)+'.';p.chapter.eventIds.push(text(e.id));pendingModels.delete(sid);}
      }
    }
    let unanchored=0;
    for(const edge of list(snapshot.edges)) {
      if(laneId&&text(edge.from)!==laneId&&text(edge.to)!==laneId)continue;
      const refs=list(edge.eventIds).map(text);if(!refs.length){unanchored++;continue;}if(!refs.every(id=>visibleIds.has(id)))continue;
      const last=refs.reduce((a,b)=>index.get(a)>index.get(b)?a:b),event=all[index.get(last)];
      const relation=edge.kind==='artifact'?name(edge.from)+' and '+name(edge.to)+' retained matching output/input bytes. This is evidence of identical content, not proof that one agent consumed the other’s output or caused its next action.':edge.kind==='child'?'A recorded parent/child relationship connects '+name(edge.from)+' to '+name(edge.to)+'.':'A recorded '+text(edge.kind)+' relationship connects '+name(edge.from)+' and '+name(edge.to)+'.';
      const c=add(event,'relationship',edge.kind==='artifact'?'Matching evidence across agents':'A recorded agent relationship',relation+(edge.verified===true?'':' The relationship is not verified.'),refs,[edge.from,edge.to]);
      if(c)c.id+=':'+text(edge.kind)+':'+text(edge.from)+':'+text(edge.to)+':'+text(edge.sha256);
    }
    // Pairing can update earlier paragraphs; sort by their last observed event.
    chapters.sort((a,b)=>Math.max(...a.eventIds.map(id=>index.get(id)))-Math.max(...b.eventIds.map(id=>index.get(id))));
    // Never keep a now-verified tone when a paired terminal belongs to damaged evidence.
    for(const c of chapters)if(c.eventIds.some(id=>['invalid','unavailable'].includes(sessions.get(text(all[index.get(id)].sessionId))?.verification))&&!c.text.includes('not established'))c.text+=' The referenced archive is not verified; these observations are not established.';
    const selectedLanes=new Set(rows.map(e=>text(e.laneId)));
    let summary=rows.length?'Across '+selectedLanes.size+' selected '+(selectedLanes.size===1?'lane':'lanes')+', the recording shows '+counts.model+' model '+(counts.model===1?'request':'requests')+' and '+counts.action+' command '+(counts.action===1?'start':'starts')+'.':'No events are included at this cursor for the selected lane.';
    if(counts.rejected)summary+=' '+counts.rejected+' '+(counts.rejected===1?'check rejected a candidate.':'checks rejected candidates.');
    if(counts.recovered)summary+=' In '+counts.recovered+' '+(counts.recovered===1?'case, the same check later accepted.':'cases, the same check later accepted.');
    if(counts.commandRecoveries)summary+=' '+counts.commandRecoveries+' previously failing check '+(counts.commandRecoveries===1?'command later succeeded.':'commands later succeeded.');
    if(counts.failedProcesses)summary+=' '+counts.failedProcesses+' '+(counts.failedProcesses===1?'command failed.':'commands failed.');
    if(counts.acceptedAgents)summary+=' '+counts.acceptedAgents+' '+(counts.acceptedAgents===1?'invocation reached acceptance.':'invocations reached acceptance.');
    if(pending.size||pendingModels.size)summary+=' At this point, '+pending.size+' '+(pending.size===1?'command has':'commands have')+' no recorded result and '+pendingModels.size+' model '+(pendingModels.size===1?'request has':'requests have')+' no recorded response. This does not establish whether a process is still running.';
    const notes=['Created locally from recorded events. Excerpts are quoted observations, not instructions or an explanation of unrecorded intent.','Narration follows the agent filter and selected scope; search, status and event-kind filters do not remove its evidence.'];
    if(snapshot.clockSync==='unknown')notes.push('Clock synchronization is unknown; cross-lane timestamps and overlaps do not establish causality.');
    const damaged=[...selectedLanes].some(id=>rows.some(e=>text(e.laneId)===id&&['invalid','incomplete','unavailable'].includes(sessions.get(text(e.sessionId))?.verification)));
    if(damaged)notes.unshift('Some selected evidence is incomplete or not verified. Integrity reflects the retained archive now, rather than a claim about what was known at the replay cursor.');
    if(list(snapshot.issues).length)notes.push('The archive reports '+snapshot.issues.length+' '+(snapshot.issues.length===1?'issue':'issues')+'; inspect Issues for gaps or limits before treating this as the whole history.');
    if(unanchored)notes.push(unanchored+' diagram '+(unanchored===1?'relationship lacks':'relationships lack')+' event anchors and is omitted from this narration.');
    const title='Timeline narration'+(laneId?' — '+name(laneId):''), cutoffId=cutoff>=0?text(all[cutoff].id):null;
    const markdown='# '+md(title)+'\n\n'+(scope==='all'?'Scope: whole recording, including events after the replay cursor.':'Scope: through event '+md(cutoffId||'none')+'.')+'\n\n'+md(summary)+'\n\n'+notes.map(n=>'> '+md(n)).join('\n\n')+'\n\n'+chapters.map(c=>'## '+md(c.title)+'\n\n'+md(c.text)+'\n\nEvidence: '+c.eventIds.map(md).join(', ')).join('\n\n')+'\n';
    return {schema:'bench.trace.narration/v1',title,summary,scope,cutoffId,coveredEvents:rows.length,totalEvents:all.filter(e=>!laneId||text(e.laneId)===laneId).length,notes,chapters,markdown};
  }
  return {build};
})();

</script>
<script>
(function(){
'use strict';
const $=s=>document.querySelector(s), safe=v=>Array.isArray(v)?v:[], str=v=>v==null?'':String(v);
const el={empty:$('#empty'),emptyCopy:$('#empty-copy'),workspace:$('#workspace'),banner:$('#state-banner'),lanes:$('#lanes'),ruler:$('#ruler'),feed:$('#feed-list'),details:$('#panel-details'),streams:$('#panel-streams'),raw:$('#panel-raw'),title:$('#detail-title'),scrub:$('#scrubber'),result:$('#result-count'),agent:$('#agent-filter'),kinds:$('#kind-filters'),sources:$('#sources'),issues:$('#issues-list'),issueCount:$('#issue-count'),mode:$('#mode-label'),follow:$('#follow-button'),play:$('#play-button')};
const primary=new Set(['start','model','response','action','check','result','retry','error','end','artifact']);
let snapshot=null, ordered=[], orderIndex=new Map(), eventMap=new Map(), laneMap=new Map(), sessionMap=new Map(), laneEvents=new Map(), selectedId=null, enabledKinds=new Set(), knownKinds=new Set(), filtersInitialized=false, following=false, playing=false, timer=null, feedStart=0, feedPage=200, generation=0, demoActive=false, zoom=1;const rawCache=new Map(),streamCache=new Map();
let narration=null,narrationKey='',narrationShown=50,narrationSnapshotVersion=0,narrationVoice=null,narrationSpeaking=false,narrationFrozenKey='',narrationQueue=[],narrationUtterance=null,narrationReadingVoice=null,narrationSpeechToken=0;

function make(tag,cls,text){const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=text;return n}
function timeOf(e){const n=Number(e&&e.time);return Number.isFinite(n)?n:0}
function fmtTime(v){if(!Number.isFinite(v))return 'Not supplied';const d=new Date(v);return d.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3})}
function duration(v){return v==null||v===''||!Number.isFinite(Number(v))?'Not supplied':Number(v).toLocaleString()+' ms'}
function buildIndexes(){
 const raw=safe(snapshot.events);eventMap=new Map();laneMap=new Map(safe(snapshot.lanes).map(x=>[str(x.id),x]));sessionMap=new Map(safe(snapshot.sessions).map(x=>[str(x.id),x]));laneEvents=new Map();
 ordered=raw.map((e,i)=>({e,i})).sort((a,b)=>{const ao=Number(a.e.order),bo=Number(b.e.order),ah=Number.isFinite(ao),bh=Number.isFinite(bo);if(ah&&bh)return ao-bo||a.i-b.i;if(ah!==bh)return ah?-1:1;return a.i-b.i}).map(x=>x.e);
 orderIndex=new Map();ordered.forEach((e,i)=>{orderIndex.set(str(e.id),i);eventMap.set(str(e.id),e);const id=str(e.laneId);if(!laneEvents.has(id))laneEvents.set(id,[]);laneEvents.get(id).push(e)});
}
function currentIndex(){return orderIndex.has(str(selectedId))?orderIndex.get(str(selectedId)):-1}
function renderSources(){el.sources.replaceChildren();safe(snapshot.sources).forEach(s=>{const n=make('div','source');n.append(make('strong','',s.label||s.id||'Unnamed source'),make('small','',s.path||'Path not supplied'));el.sources.append(n)});if(!snapshot.sources||!snapshot.sources.length)el.sources.append(make('p','', 'No source metadata supplied.'))}
function renderIssues(){const a=safe(snapshot.issues);el.issueCount.textContent=str(a.length);el.issues.replaceChildren();a.slice(0,50).forEach(i=>{const n=make('div','notice '+(i.severity==='error'?'error':''),i.message||'Unspecified issue');if(i.path)n.title=i.path;el.issues.append(n)});if(a.length>50)el.issues.append(make('p','',`${a.length-50} additional issues retained in the snapshot.`))}
function renderFilters(){
 const priorAgent=el.agent.value, kinds=[...new Set(safe(snapshot.events).map(e=>e.type||'event'))].sort();
 if(!filtersInitialized){enabledKinds=new Set(kinds.filter(k=>primary.has(k)));filtersInitialized=true}else kinds.forEach(k=>{if(!knownKinds.has(k)&&primary.has(k))enabledKinds.add(k)});
 knownKinds=new Set(kinds);el.kinds.replaceChildren();
 kinds.forEach(k=>{const lab=make('label'),box=document.createElement('input');box.type='checkbox';box.value=k;box.checked=enabledKinds.has(k);box.addEventListener('change',()=>{box.checked?enabledKinds.add(k):enabledKinds.delete(k);feedStart=0;render()});lab.append(box,document.createTextNode(k));el.kinds.append(lab)});
 el.agent.replaceChildren();const all=make('option','','All agents');all.value='';el.agent.append(all);safe(snapshot.lanes).forEach(l=>{const o=make('option','',l.title||l.id||'Unnamed lane');o.value=str(l.id);el.agent.append(o)});if([...el.agent.options].some(o=>o.value===priorAgent))el.agent.value=priorAgent;
}
function statusGroup(e){const v=str(e.status||sessionMap.get(str(e.sessionId))?.state||'unknown').toLowerCase();if(['accepted','complete','success','passed'].includes(v))return 'success';if(['open','incomplete','unfinished','interrupted'].includes(v))return 'open';return 'failed'}
function matchesBase(e){const q=$('#search').value.trim().toLowerCase(),status=$('#status-filter').value;return enabledKinds.has(e.type||'event')&&(status==='all'||statusGroup(e)===status)&&(!q||JSON.stringify(e).toLowerCase().includes(q))}function matches(e){const agent=el.agent.value;return (!agent||str(e.laneId)===agent)&&matchesBase(e)}
function filtered(){return ordered.filter(matches)}
function contextLanes(){const agent=el.agent.value;if(!agent)return new Set(safe(snapshot.lanes).map(l=>str(l.id)));const ids=new Set([agent]);safe(snapshot.edges).forEach(x=>{if(str(x.from)===agent)ids.add(str(x.to));if(str(x.to)===agent)ids.add(str(x.from))});return ids}
function fullBounds(){let source=ordered;if(el.agent.value){source=laneEvents.get(el.agent.value)||[]}const times=source.map(timeOf).filter(Number.isFinite);let a=times.length?Math.min(...times):0,b=times.length?Math.max(...times):a;if(!el.agent.value){if(Number.isFinite(Number(snapshot.start)))a=Number(snapshot.start);if(Number.isFinite(Number(snapshot.end)))b=Number(snapshot.end)}if(b<=a)b=a+1;return [a,b]}
function bounds(){const [fa,fb]=fullBounds();if(zoom===1)return[fa,fb];const cursor=timeOf(eventMap.get(str(selectedId))),center=Math.max(fa,Math.min(fb,cursor||((fa+fb)/2))),span=(fb-fa)/zoom;let a=center-span/2,b=center+span/2;if(a<fa){b+=fa-a;a=fa}if(b>fb){a-=b-fb;b=fb}return[Math.max(fa,a),Math.min(fb,b)]}
function pct(e,a,b){return b===a?50:Math.max(0,Math.min(100,(timeOf(e)-a)/(b-a)*100))}
function renderTimeline(){
 const [a,b]=bounds(),span=Math.max(1,b-a),context=contextLanes(),visible=ordered.filter(e=>context.has(str(e.laneId))&&matchesBase(e)&&timeOf(e)<=b&&timeOf(e)+Math.max(0,Number(e.duration)||0)>=a),visibleIds=new Set(visible.map(e=>str(e.id))),ci=currentIndex();el.ruler.replaceChildren();
 for(let i=0;i<=4;i++){if(window.innerWidth<650&&i%2)continue;const value=a+(b-a)*i/4,t=make('span','',new Date(value).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit',hour12:false}));t.style.left=(i*25)+'%';t.style.transform=i===0?'none':i===4?'translateX(-100%)':'translateX(-50%)';t.title=new Date(value).toISOString();el.ruler.append(t)}
 el.lanes.replaceChildren();const ids=contextLanes(),rendered=safe(snapshot.lanes).filter(l=>ids.has(str(l.id)));
 rendered.forEach(l=>{const row=make('div','lane'),label=make('div','lane-label'),track=make('div','lane-track'),positions=[];label.title=l.title||l.id||'Unnamed lane';label.append(make('strong','',l.title||l.id||'Unnamed lane'),make('small','',(l.state||'state unknown')+' · '+(l.verification||'not verified')));(laneEvents.get(str(l.id))||[]).filter(e=>visibleIds.has(str(e.id))).forEach(e=>{const btn=make('button','event-mark'+(primary.has(e.type)?'':' optional'));btn.type='button';btn.dataset.type=e.type||'event';btn.dataset.status=e.status||'unknown';btn.classList.toggle('future',orderIndex.get(str(e.id))>ci&&ci>=0);btn.classList.toggle('selected',str(e.id)===str(selectedId));btn.setAttribute('aria-label',(e.type||'event')+': '+(e.title||e.id));btn.title=(e.title||e.id||'Event')+' · '+new Date(timeOf(e)).toISOString();const left=pct(e,a,b),w=e.duration==null?1.1:Math.max(.8,Math.min(100-left,(Math.min(b,timeOf(e)+Math.max(0,Number(e.duration)||0))-Math.max(a,timeOf(e)))/span*100));const collision=positions.some(x=>Math.abs(x-left)<1.4);positions.push(left);btn.style.left=left+'%';if(left>=99.9)btn.style.transform='translateX(-100%)';btn.style.width=w+'%';btn.style.top=collision?(primary.has(e.type)?'28px':'31px'):(primary.has(e.type)?'8px':'12px');btn.addEventListener('click',()=>selectEvent(e.id,true));track.append(btn)});row.append(label,track);el.lanes.append(row)});
 drawEdges(rendered,a,b);const head=make('div','playhead'),e=eventMap.get(str(selectedId));head.style.left=`calc(var(--lane-label) + (100% - var(--lane-label)) * ${e?pct(e,a,b)/100:0})`;el.lanes.append(head);
 el.scrub.max=Math.max(0,ordered.length-1);el.scrub.value=Math.max(0,ci);$('#scrub-start').textContent=new Date(a).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'});$('#scrub-end').textContent=new Date(b).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'});$('#scrub-start').title=new Date(a).toISOString();$('#scrub-end').title=new Date(b).toISOString();$('#visible-range').textContent=`Visible ${duration(b-a)} · ${zoom}×`;document.querySelectorAll('[data-zoom]').forEach(x=>x.setAttribute('aria-pressed',str(Number(x.dataset.zoom)===zoom)));$('.timeline-shell').classList.toggle('needs-scroll',window.innerWidth<500&&rendered.length>4);
}
function laneAnchor(id,time,a,b,rendered,fallback){const lane=rendered.findIndex(l=>str(l.id)===str(id));return{x:time!=null&&Number.isFinite(Number(time))?pct({time:Number(time)},a,b):fallback,y:lane<0?0:(lane+.5)*56}}
function drawEdges(rendered,a,b){
 const ids=new Set(rendered.map(l=>str(l.id))),height=Math.max(1,rendered.length*56),svg=document.createElementNS('http://www.w3.org/2000/svg','svg'),width=Math.max(240,el.lanes.clientWidth-(parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--lane-label'))||130));svg.classList.add('edge-layer');svg.setAttribute('viewBox',`0 0 ${width} ${height}`);svg.setAttribute('role','img');svg.setAttribute('aria-label','Explicit recorded relationship and artifact links');const defs=document.createElementNS(svg.namespaceURI,'defs');defs.innerHTML='<marker id="edge-arrow" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto"><path d="M0,0 L7,3.5 L0,7 z" fill="context-stroke"/></marker>';svg.append(defs);
 safe(snapshot.edges).filter(x=>ids.has(str(x.from))&&ids.has(str(x.to))).forEach((x,i)=>{const pp=laneAnchor(x.from,x.fromTime,a,b,rendered,20+(i%3)*5),qq=laneAnchor(x.to,x.toTime,a,b,rendered,75+(i%3)*5),p={x:pp.x/100*width,y:pp.y},q={x:qq.x/100*width,y:qq.y},path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d',`M ${p.x} ${p.y} Q ${(p.x+q.x)/2} ${(p.y+q.y)/2-12} ${q.x} ${q.y}`);path.setAttribute('marker-end','url(#edge-arrow)');path.setAttribute('class','edge'+(x.verified===true?'':' unverified')+(x.kind==='artifact'?' artifact':''));const full=x.kind==='artifact'?'matching retained bytes / artifact link':(x.label||x.kind||'relationship');const title=document.createElementNS(svg.namespaceURI,'title');title.textContent=`${full}: ${x.from} to ${x.to} · ${x.verified===true?'verified':'unverified'}${x.fromTime!=null||x.toTime!=null?' · observed anchors supplied':' · relationship-only positioning'}`;path.append(title);svg.append(path);const text=document.createElementNS(svg.namespaceURI,'text');text.setAttribute('x',String((p.x+q.x)/2));text.setAttribute('y',String((p.y+q.y)/2-4));text.setAttribute('class','edge-label');const compact=x.kind==='artifact'?'artifact link':(x.label||x.kind||'link');text.textContent=compact.length>22?compact.slice(0,20)+'…':compact;text.append(title.cloneNode(true));svg.append(text)});
 el.lanes.append(svg);
}
function renderFeed(followCursor=true){
 const list=filtered(),ci=currentIndex(),selected=list.findIndex(e=>str(e.id)===str(selectedId));if(followCursor&&selected>=0&&(selected<feedStart||selected>=feedStart+feedPage))feedStart=Math.max(0,Math.min(list.length-feedPage,selected-Math.floor(feedPage/2)));feedStart=Math.max(0,Math.min(feedStart,Math.max(0,list.length-feedPage)));const shown=list.slice(feedStart,feedStart+feedPage);el.feed.replaceChildren();
 shown.forEach(e=>{const oi=orderIndex.get(str(e.id)),b=make('button','event-row'+(str(e.id)===str(selectedId)?' selected':'')+(oi>ci&&ci>=0?' future':''));b.type='button';b.addEventListener('click',()=>selectEvent(e.id,true));const tm=make('time','',new Date(timeOf(e)).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'}));tm.title=new Date(timeOf(e)).toISOString();b.append(tm,make('span','pill '+str(e.type),e.type||'event'));const title=make('span','event-title'),lane=laneMap.get(str(e.laneId))||{};title.append(make('strong','',e.title||'Untitled event'),make('small','',(lane.title||e.laneId||'Unassigned')+' · seq '+(e.seq??'—')+(oi>ci&&ci>=0?' · later than cursor':'')));b.append(title,make('span','status',e.status||'unknown'));el.feed.append(b)});
 $('#feed-earlier').disabled=feedStart===0;$('#feed-later').disabled=feedStart+feedPage>=list.length;$('#feed-count').textContent=list.length?`${feedStart+1}–${Math.min(list.length,feedStart+feedPage)} of ${list.length}`:'0 events';
}
function addMeta(dl,k,v){dl.append(make('dt','',k),make('dd','',v))}
function renderDetails(){
 el.details.replaceChildren();el.streams.replaceChildren();el.raw.replaceChildren();const e=eventMap.get(str(selectedId));if(!e){el.title.textContent='No event selected';el.details.append(make('p','','Select a retained event.'));return}
 const s=sessionMap.get(str(e.sessionId))||{},lane=laneMap.get(str(e.laneId))||{};el.title.textContent=e.title||e.type||'Event';const dl=make('dl','meta');addMeta(dl,'Type',e.type||'Not supplied');addMeta(dl,'Status',e.status||'Unknown');addMeta(dl,'Time',fmtTime(timeOf(e)));addMeta(dl,'Duration',duration(e.duration));addMeta(dl,'Lane',lane.title||e.laneId||'Not supplied');addMeta(dl,'Session',s.title||e.sessionId||'Not supplied');addMeta(dl,'Sequence',e.seq??'Not supplied');addMeta(dl,'Order',e.order??'Array order fallback');addMeta(dl,'Verification',s.verification||lane.verification||'Not established');addMeta(dl,'Completeness',s.complete===true?'Observed terminal completeness':s.complete===false?'Incomplete / no complete terminal':'Not established');el.details.append(dl);
 if(e.text!==undefined)el.details.append(make('div','section-title','Message / observation'),make('div','copy-text',e.text||'Empty text'));if(e.details&&Object.keys(e.details).length)el.details.append(make('div','section-title','Arguments / fields'),make('pre','raw',JSON.stringify(e.details,null,2)));
 const links=safe(snapshot.edges).filter(x=>str(x.from)===str(e.laneId)||str(x.to)===str(e.laneId));
 if(links.length){
  el.details.append(make('div','section-title','Recorded relationships and artifact matches'));
  links.forEach(x=>{
   const linkText=(x.label||x.kind||'link')+' · '+(laneMap.get(str(x.from))?.title||x.from)+' → '+(laneMap.get(str(x.to))?.title||x.to)+' · '+(x.verified===true?'verified':'unverified')+(x.kind==='artifact'?' · Matching bytes do not prove consumption or parentage.':'');
   el.details.append(make('div','notice',linkText));
  });
 }
 const download=make('button','','Download session JSONL');download.type='button';download.addEventListener('click',()=>{try{const url=window.BenchTrace?.downloadSession?.(s.id);if(!url)throw Error('Original session download is unavailable.');const a=document.createElement('a');a.href=url;a.download='session.jsonl';a.click();if(url.startsWith('blob:'))setTimeout(()=>URL.revokeObjectURL(url),1000)}catch(error){el.details.append(make('p','notice error',error.message))}});el.details.append(download);
 renderStreams(s,e);renderRaw(e);
}
function decode(base64){const bin=atob(base64||''),a=new Uint8Array(bin.length);for(let i=0;i<bin.length;i++)a[i]=bin.charCodeAt(i);return a}
function bytesText(a){try{return new TextDecoder('utf-8',{fatal:true}).decode(a)}catch(_){return null}}
function hex(a){return Array.from(a).map((v,i)=>(i%16===0?(i?'\n':'')+i.toString(16).padStart(8,'0')+'  ':'')+v.toString(16).padStart(2,'0')+' ').join('')}
function preview(card,text,label,kind){let box=card.querySelector('[data-output]');if(!box){box=make('pre','raw');box.dataset.output='';box.setAttribute('aria-live','polite');card.append(box)}box.textContent=label+'\n'+text;box.classList.toggle('error',kind==='error')}
function showStreamResult(card,r,mode){const bytes=decode(r.base64),txt=bytesText(bytes),asHex=mode==='hex'||txt===null;preview(card,asHex?hex(bytes):txt,(asHex?'Hex':'Text')+' · '+(r.bytes??bytes.length)+' bytes · '+(r.verified?'verified':'not verified')+' · '+(r.complete?'complete':'incomplete'))}
async function readStream(card,s,st,mode,requestEvent,requestGen){const api=window.BenchTrace,key=str(s.id)+'\u0000'+str(st.name)+'\u0000'+mode;if(!api||typeof api.readStream!=='function'){preview(card,str(st.preview),'Preview only — full-stream host API unavailable'+(st.truncated?' · truncated':''),'error');return}preview(card,'','Loading full retained bytes…');try{const r=await api.readStream(s.id,st.name);if(generation!==requestGen||selectedId!==requestEvent)return;streamCache.set(key,r);if(!card.isConnected){renderDetails();return}showStreamResult(card,r,mode)}catch(err){if(card.isConnected)preview(card,str(err&&err.message||err),'Unable to load full stream','error')}}
function renderStreams(s,e){const streams=safe(s.streams).filter(x=>!e.stream||x.name===e.stream);if(!streams.length){el.streams.append(make('p','','No streams or artifacts are associated with this event session.'));return}streams.forEach(st=>{const c=make('article','stream-card'),h=make('header');h.append(make('strong','',st.name||'unnamed stream'),make('small','',(st.bytes??'unknown')+' bytes'));c.append(h);const acts=make('div','stream-actions'),g=generation,id=selectedId;[['Full text','text'],['Hex','hex']].forEach(([label,mode])=>{const b=make('button','',label);b.type='button';b.addEventListener('click',()=>readStream(c,s,st,mode,id,g));acts.append(b)});const down=make('button','','Download bytes');down.type='button';down.addEventListener('click',async()=>{const api=window.BenchTrace;if(!api||typeof api.readStream!=='function'){preview(c,str(st.preview),'Download unavailable — preview only','error');return}try{const r=await api.readStream(s.id,st.name);if(generation!==g||selectedId!==id||!c.isConnected)return;const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([decode(r.base64)]));a.download=st.filename||st.name||'stream.bin';a.click();setTimeout(()=>URL.revokeObjectURL(a.href),1000)}catch(err){preview(c,str(err&&err.message||err),'Download failed','error')}});acts.append(down);c.append(acts);const textKey=str(s.id)+'\u0000'+str(st.name)+'\u0000text',hexKey=str(s.id)+'\u0000'+str(st.name)+'\u0000hex',cached=streamCache.get(textKey)||streamCache.get(hexKey);if(cached)showStreamResult(c,cached,streamCache.has(textKey)?'text':'hex');else preview(c,str(st.preview||''),'Preview · '+(st.truncated?'truncated':'not marked truncated')+' · '+(st.encoding||'encoding unknown'));el.streams.append(c)})}
function renderRaw(e){const cached=rawCache.get(str(e.id));if(cached){if(cached.normalizedRequest!==undefined)el.raw.append(make('div','section-title','Exact normalized model request'),make('pre','raw',typeof cached.normalizedRequest==='string'?cached.normalizedRequest:JSON.stringify(cached.normalizedRequest,null,2)));el.raw.append(make('pre','raw',JSON.stringify(cached,null,2)),make('div','section-title','Snapshot event'),make('pre','raw',JSON.stringify(e,null,2)));return}const msg=make('p','','Select “Load complete event” to request retained raw evidence from the host.'),btn=make('button','','Load complete event'),id=selectedId,g=generation;btn.type='button';btn.addEventListener('click',async()=>{btn.disabled=true;msg.textContent='Loading complete event…';try{const api=window.BenchTrace;if(!api||typeof api.readEvent!=='function')throw new Error('Host readEvent API unavailable');const raw=await api.readEvent(e.id);if(generation!==g||selectedId!==id)return;rawCache.set(str(e.id),raw);if(!btn.isConnected){renderDetails();return}msg.remove();btn.remove();if(raw&&raw.normalizedRequest!==undefined)el.raw.append(make('div','section-title','Exact normalized model request'),make('pre','raw',typeof raw.normalizedRequest==='string'?raw.normalizedRequest:JSON.stringify(raw.normalizedRequest,null,2)));el.raw.append(make('pre','raw',JSON.stringify(raw,null,2)))}catch(err){if(btn.isConnected){msg.textContent='Unable to load complete event: '+str(err&&err.message||err);btn.disabled=false}}});el.raw.append(msg,btn,make('div','section-title','Snapshot event'),make('pre','raw',JSON.stringify(e,null,2)))}

function narrationElements(){return{panel:$('#narration-panel'),toggle:$('#narration-toggle'),scope:$('#narration-scope'),status:$('#narration-status'),content:$('#narration-content'),download:$('#narration-download'),speak:$('#narration-speak'),stop:$('#narration-stop'),speech:$('#narration-speech-state')}}
function narrationCurrentKey(){const n=narrationElements();return [narrationSnapshotVersion,str(selectedId),str(el.agent.value),str(n.scope.value)].join('\u0000')}
function narrationRangeKey(){const n=narrationElements();return [str(selectedId),str(el.agent.value),str(n.scope.value)].join('\u0000')}
function stopNarration(reason){const n=narrationElements(),wasSpeaking=narrationSpeaking;narrationSpeaking=false;narrationSpeechToken++;narrationQueue=[];narrationUtterance=null;narrationReadingVoice=null;narrationFrozenKey='';if(window.speechSynthesis&&wasSpeaking)window.speechSynthesis.cancel();n.stop.disabled=true;n.speak.disabled=!narration||!narrationVoice;if(reason)n.speech.textContent=reason;else if(n.speech.textContent.startsWith('Reading'))n.speech.textContent='Reading stopped.'}
function updateNarrationVoice(){
 const n=narrationElements(),api=window.speechSynthesis;
 if(!api||typeof window.SpeechSynthesisUtterance!=='function'){narrationVoice=null;n.speak.disabled=true;n.speech.textContent='Read aloud unavailable: this browser does not provide Speech Synthesis.';return}
 const voices=safe(api.getVoices()).filter(v=>v.localService===true);
 narrationVoice=voices.find(v=>/^en(?:-|$)/i.test(v.lang||''))||voices[0]||null;
 n.speak.disabled=!narration||!narrationVoice||narrationSpeaking;
 if(!narrationVoice)n.speech.textContent='Read aloud unavailable: no on-device voice is installed.';
 else if(!narrationSpeaking)n.speech.textContent='On-device voice available: '+str(narrationVoice.name||narrationVoice.lang);
}
function narrationEvidence(chapter){
 const wrap=make('div','narration-evidence'),ids=safe(chapter.eventIds).map(str).filter(id=>eventMap.has(id));
 ids.forEach((id,i)=>{const last=i===ids.length-1,b=make('button','',last?'Show event':'Reference '+(i+1));b.type='button';b.addEventListener('click',()=>{selectEvent(id,true);const panel=$('#details-panel');panel.classList.add('open');$('#details-button').setAttribute('aria-expanded','true');setInspectorAccess(true);activateTab($('#tab-details'),false)});wrap.append(b)});
 return wrap
}
function renderNarrationResult(){
 const n=narrationElements(),oldScroll=n.content.querySelector('.narration-list')?.scrollTop||0,focused=document.activeElement,chapterId=focused?.closest('[data-chapter-id]')?.dataset.chapterId,buttonIndex=chapterId?[...focused.closest('[data-chapter-id]').querySelectorAll('button')].indexOf(focused):-1,moreFocused=focused?.classList.contains('narration-more'),notesOpen=n.content.querySelector('details')?.open||false;
 n.content.replaceChildren();n.download.disabled=!narration;
 if(!narration)return;
 if(narration.title)n.content.append(make('h3','narration-title',str(narration.title)));
 if(narration.summary)n.content.append(make('p','narration-summary',str(narration.summary)));
 const meta=make('p','narration-meta',(narration.scope==='all'?'Whole recording':'Through cursor')+' · '+Number(narration.coveredEvents||0)+' of '+Number(narration.totalEvents||0)+' events covered');
 n.content.append(meta);
 const notes=safe(narration.notes);if(notes.length){const details=make('details','narration-notes');details.open=notesOpen;details.append(make('summary','','Evidence notes'));const ul=make('ul');notes.forEach(x=>ul.append(make('li','',str(x))));details.append(ul);n.content.append(details)}
 const chapters=safe(narration.chapters),list=make('div','narration-list');list.setAttribute('aria-label','Narration paragraphs');
 chapters.slice(0,narrationShown).forEach((c,i)=>{const article=make('article','narration-chapter');article.dataset.chapterId=str(c.id);article.append(make('h3','',str(c.title||'Section '+(i+1))),make('p','',str(c.text||'')));const refs=narrationEvidence(c);if(refs.childElementCount)article.append(refs);list.append(article)});
 if(!chapters.length)list.append(make('p','narration-empty','No narrative chapters were returned for this range.'));
 n.content.append(list);list.scrollTop=oldScroll;
 if(chapters.length>narrationShown){const more=make('button','narration-more','Load more ('+(chapters.length-narrationShown)+' remaining)');more.type='button';more.addEventListener('click',()=>{narrationShown+=50;renderNarrationResult()});n.content.append(more)}
 if(chapterId){const chapter=[...n.content.querySelectorAll('[data-chapter-id]')].find(c=>c.dataset.chapterId===chapterId);chapter?.querySelectorAll('button')[buttonIndex]?.focus({preventScroll:true})}else if(moreFocused)n.content.querySelector('.narration-more')?.focus({preventScroll:true});
 updateNarrationVoice()
}
function buildNarration(force=false){
 const n=narrationElements();if(n.panel.hidden||!snapshot)return;
 const key=narrationCurrentKey();if(!force&&key===narrationKey)return;
 if(narrationSpeaking&&narrationRangeKey()!==narrationFrozenKey)stopNarration('Reading stopped because the narration range changed.');
 narrationKey=key;if(force)narrationShown=50;narration=null;n.download.disabled=true;n.speak.disabled=true;
 const engine=window.BenchNarrator;
 if(!engine||typeof engine.build!=='function'){n.content.replaceChildren();n.status.textContent='Narration unavailable: the local narration engine was not supplied.';updateNarrationVoice();return}
 n.status.textContent='Building narration locally…';
 try{
  const result=engine.build(snapshot,{cutoffId:selectedId,scope:n.scope.value,laneId:el.agent.value});
  if(!result||!Array.isArray(result.chapters)||typeof result.markdown!=='string')throw new Error('Narration engine returned an invalid result');
  narration=result;n.status.textContent='Narration ready.';renderNarrationResult()
 }catch(err){n.content.replaceChildren();console.error('Bench Trace narration failed',err);n.status.textContent='Narration unavailable: '+str(err&&err.message||err);updateNarrationVoice()}
}
function downloadNarration(){
 if(!narration)return;const blob=new Blob([narration.markdown],{type:'text/markdown;charset=utf-8'}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download='bench-trace-narration.md';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)
}
function speakNext(){
 const n=narrationElements();if(!narrationSpeaking)return;
 const text=narrationQueue.shift();if(text===undefined){narrationSpeaking=false;narrationUtterance=null;n.stop.disabled=true;n.speak.disabled=!narrationVoice;n.speech.textContent='Reading complete. This reading used the frozen narration snapshot.';return}
 if(narrationReadingVoice?.localService!==true){stopNarration('Reading stopped: the selected local voice is unavailable.');return}const token=narrationSpeechToken;try{const u=new SpeechSynthesisUtterance(text);narrationUtterance=u;u.voice=narrationReadingVoice;u.onend=()=>{if(narrationSpeaking&&token===narrationSpeechToken)speakNext()};u.onerror=e=>{if(token!==narrationSpeechToken)return;stopNarration('Reading stopped because the on-device speech service reported an error.')};window.speechSynthesis.speak(u)}catch(error){stopNarration('Reading stopped: '+str(error.message))}
}
function speakNarration(){
 if(!narration||!narrationVoice||!window.speechSynthesis)return;
 const pieces=[narration.title,narration.scope==='all'?'Whole recording, including events after the replay cursor.':'Through the replay cursor.',narration.summary,...safe(narration.notes),...safe(narration.chapters).flatMap(c=>[c.title,c.text])].map(str).filter(Boolean);if(!pieces.length)return;
 stopNarration();narrationReadingVoice=narrationVoice;narrationFrozenKey=narrationRangeKey();narrationQueue=pieces;narrationSpeaking=true;const n=narrationElements();n.speak.disabled=true;n.stop.disabled=false;n.speech.textContent='Reading a frozen narration snapshot. New live events will not be queued.';speakNext()
}
function renderNarration(){if(!$('#narration-panel').hidden)buildNarration()}
function controls(){const n=ordered.length,i=currentIndex();$('#prev-button').disabled=n<2||i<=0;$('#next-button').disabled=n<2||i<0||i>=n-1;el.play.disabled=n<2;el.scrub.disabled=n<2;$('#export-button').disabled=!snapshot||demoActive;el.follow.disabled=!snapshot||!snapshot.live}
function render(){
 el.banner.replaceChildren();if(!snapshot){el.empty.hidden=false;el.workspace.hidden=true;controls();return}renderSources();renderIssues();const count=safe(snapshot.events).length;if(!count){el.empty.hidden=false;el.workspace.hidden=true;el.emptyCopy.textContent='This host snapshot contains zero events. Source metadata and issues are shown; the illustrative demo remains separate and opt-in.'}else{el.empty.hidden=true;el.workspace.hidden=false;const f=filtered();el.result.textContent=f.length+' event'+(f.length===1?'':'s');renderTimeline();renderFeed();renderDetails()}
 if(snapshot.clockSync==='unknown'||safe(snapshot.sources).some(s=>s.clockSync==='unknown'))el.banner.append(make('div','notice','Clock sync unknown · cross-source wall times do not establish causality.'));
 if(demoActive)el.banner.append(make('div','notice','Illustrative demo only · not observed or verified evidence.'));renderNarration();el.mode.textContent=demoActive?'Illustrative demo':playing?'Historical replay':following&&snapshot.live?'Following live feed':snapshot.live?'Live feed · paused':'Recorded evidence';el.follow.setAttribute('aria-pressed',str(following));controls();
}
function stop(){playing=false;clearTimeout(timer);timer=null;el.play.textContent='▶';el.play.setAttribute('aria-pressed','false');el.play.setAttribute('aria-label','Play replay')}
function schedule(){if(!playing)return;const i=currentIndex();if(i<0||i>=ordered.length-1){stop();render();return}const gap=Math.max(0,timeOf(ordered[i+1])-timeOf(ordered[i]))/(Number($('#speed').value)||1);timer=setTimeout(()=>{timer=null;if(!playing)return;const next=currentIndex()+1;if(next>=ordered.length){stop();render();return}selectedId=ordered[next].id;generation++;render();schedule()},Math.min(gap,2147483647))}
function togglePlay(){if(playing){stop();render();return}if(ordered.length<2)return;if(currentIndex()>=ordered.length-1)selectedId=ordered[0].id;playing=true;following=false;el.play.textContent='❚❚';el.play.setAttribute('aria-pressed','true');el.play.setAttribute('aria-label','Pause replay');render();schedule()}
function selectEvent(id,manual){selectedId=id;generation++;if(manual){following=false;stop()}render()}
function step(d){const i=currentIndex();if(!ordered.length)return;selectEvent(ordered[Math.max(0,Math.min(ordered.length-1,(i<0?0:i)+d))].id,true)}
function invalidateCaches(next){
 const sessions=new Map(safe(next.sessions).map(s=>[str(s.id),s]));
 const changed=id=>{const a=sessionMap.get(str(id)),b=sessions.get(str(id));return !a||!b||a.sha256!==b.sha256||a.bytes!==b.bytes||a.verification!==b.verification};
 for(const key of streamCache.keys())if(changed(key.split('\u0000')[0]))streamCache.delete(key);
 for(const key of rawCache.keys())if(changed(eventMap.get(key)?.sessionId))rawCache.delete(key);
 if(selectedId&&changed(eventMap.get(str(selectedId))?.sessionId))generation++;
}
function setData(next){narrationSnapshotVersion++;if(!next||next.schema!=='bench.trace/v1'){snapshot={schema:'bench.trace/v1',revision:1,sources:[],lanes:[],sessions:[],events:[],edges:[],issues:[{severity:'error',message:'Invalid snapshot: expected schema bench.trace/v1.'}]};selectedId=null;buildIndexes();renderFilters();render();return}const previous=selectedId,wasFollowing=following,wasPlaying=playing,hadSnapshot=!!snapshot;invalidateCaches(next);snapshot=next;demoActive=false;buildIndexes();renderFilters();if(wasFollowing&&next.live&&ordered.length)selectedId=ordered.at(-1).id;else if(previous&&eventMap.has(str(previous)))selectedId=previous;else{const meaningful=[...ordered].reverse().find(e=>primary.has(e.type));selectedId=meaningful?.id||ordered.at(-1)?.id||null;following=!hadSnapshot&&!!next.live}if(str(previous)!==str(selectedId))generation++;if(wasPlaying&&playing&&str(previous)!==str(selectedId)){clearTimeout(timer);schedule()}render()}
function demo(){const t=Date.UTC(2026,1,3,10);setData({schema:'bench.trace/v1',revision:1,live:false,start:t,end:t+9000,clockSync:'unknown',sources:[{id:'demo',label:'Illustrative demo',path:'Inline example · not observed evidence',clockSync:'unknown'}],lanes:[{id:'lead',title:'Lead · illustrative',state:'complete',verification:'unsealed'},{id:'child',title:'Frontend · illustrative',state:'accepted',verification:'verified',parentId:'lead'},{id:'sibling',title:'Incomplete · illustrative',state:'open',verification:'incomplete'}],sessions:[{id:'ask1',laneId:'lead',title:'Plan',complete:true,verification:'unsealed',streams:[]},{id:'proc1',laneId:'child',title:'Build UI',complete:true,verification:'verified',streams:[{name:'stderr',bytes:25,preview:'check failed: missing UI\n',truncated:false,encoding:'utf-8',verified:true,complete:true}]},{id:'proc2',laneId:'child',title:'Repair UI',complete:true,verification:'verified',streams:[{name:'stdout',bytes:13,preview:'check passed\n',truncated:false,encoding:'utf-8',verified:true,complete:true}]},{id:'ask2',laneId:'sibling',complete:false,verification:'incomplete',streams:[]}],events:[{id:'e1',sessionId:'ask1',laneId:'lead',seq:1,order:1,time:t,type:'model',title:'Model request',text:'Plan the evidence browser.',status:'complete',duration:900},{id:'e2',sessionId:'ask1',laneId:'lead',seq:2,order:2,time:t+900,type:'response',title:'Plan recorded',status:'complete'},{id:'e3',sessionId:'proc1',laneId:'child',seq:1,order:3,time:t+1700,type:'action',title:'Build interface',status:'complete',duration:1800},{id:'e4',sessionId:'ask2',laneId:'sibling',seq:1,order:4,time:t+2400,type:'model',title:'Sibling request',status:'incomplete'},{id:'e5',sessionId:'proc1',laneId:'child',seq:2,order:5,time:t+3600,type:'check',title:'Check failed',text:'failed: missing accessible label',status:'failed',stream:'stderr'},{id:'e6',sessionId:'proc2',laneId:'child',seq:1,order:6,time:t+5000,type:'retry',title:'Repair labels',status:'complete'},{id:'e7',sessionId:'proc2',laneId:'child',seq:2,order:7,time:t+6800,type:'artifact',title:'Artifact retained',status:'complete'},{id:'e8',sessionId:'proc2',laneId:'child',seq:3,order:8,time:t+7800,type:'result',title:'Check accepted',status:'accepted'}],edges:[{from:'lead',to:'child',kind:'child',label:'recorded child',verified:true},{from:'lead',to:'sibling',kind:'compaction',label:'compacted context',verified:false}],issues:[{severity:'warning',message:'Illustrative sibling has no observed terminal event.'}]});demoActive=true;render()}
function setInspectorAccess(open){const p=$('#details-panel'),overlay=matchMedia('(max-width:980px)').matches;if(overlay&&!open){p.inert=true;p.setAttribute('aria-hidden','true')}else{p.inert=false;p.removeAttribute('aria-hidden')}}function closeInspector(){const p=$('#details-panel');p.classList.remove('open');setInspectorAccess(false);$('#details-button').setAttribute('aria-expanded','false');$('#details-button').focus()}
function activateTab(tab,focus){document.querySelectorAll('[role=tab]').forEach(t=>{const on=t===tab;t.setAttribute('aria-selected',str(on));t.tabIndex=on?0:-1});document.querySelectorAll('[role=tabpanel]').forEach(p=>p.hidden=p.id!==tab.getAttribute('aria-controls'));if(focus)tab.focus()}
window.BenchTraceUI={setData};document.addEventListener('bench:snapshot',e=>setData(e.detail));document.addEventListener('bench:connection',e=>{const d=e.detail||{},n=$('#connection');n.classList.toggle('connected',!!d.connected);$('#connection-label').textContent=d.message||(d.connected?'Connected':'Disconnected')});
$('#narration-toggle').addEventListener('click',e=>{const n=narrationElements(),open=n.panel.hidden;n.panel.hidden=!open;e.currentTarget.setAttribute('aria-expanded',str(open));if(open){buildNarration(true);n.panel.scrollIntoView({block:'start'})}else stopNarration('Reading stopped because the narration panel was closed.')});
$('#narration-scope').addEventListener('change',()=>buildNarration(true));
$('#narration-download').addEventListener('click',downloadNarration);
$('#narration-speak').addEventListener('click',speakNarration);
$('#narration-stop').addEventListener('click',()=>stopNarration());
if(window.speechSynthesis){window.speechSynthesis.addEventListener?.('voiceschanged',updateNarrationVoice)}
window.addEventListener('pagehide',()=>stopNarration());
$('#demo-button').addEventListener('click',demo);$('#search').addEventListener('input',()=>{feedStart=0;render()});el.agent.addEventListener('change',()=>{feedStart=0;render()});$('#prev-button').addEventListener('click',()=>step(-1));$('#next-button').addEventListener('click',()=>step(1));el.play.addEventListener('click',togglePlay);$('#speed').addEventListener('change',()=>{if(playing){clearTimeout(timer);schedule()}});
el.follow.addEventListener('click',()=>{following=!following;if(following){stop();if(ordered.length)selectedId=ordered.at(-1).id}render()});el.scrub.addEventListener('input',()=>{const e=ordered[Number(el.scrub.value)];if(e)selectEvent(e.id,true)});$('#feed-earlier').addEventListener('click',()=>{feedStart=Math.max(0,feedStart-feedPage);renderFeed(false)});$('#feed-later').addEventListener('click',()=>{feedStart+=feedPage;renderFeed(false)});document.querySelectorAll('[data-zoom]').forEach(b=>b.addEventListener('click',()=>{zoom=Number(b.dataset.zoom);renderTimeline()}));$('#fit-all').addEventListener('click',()=>{zoom=1;el.agent.value='';render()});
$('#show-all').addEventListener('click',()=>{knownKinds.forEach(k=>enabledKinds.add(k));renderFilters();render()});$('#reset-filters').addEventListener('click',()=>{enabledKinds=new Set([...knownKinds].filter(k=>primary.has(k)));el.agent.value='';$('#search').value='';$('#status-filter').value='all';zoom=1;feedStart=0;renderFilters();render()});$('#filter-toggle').addEventListener('click',e=>{const open=e.currentTarget.getAttribute('aria-expanded')!=='true';e.currentTarget.setAttribute('aria-expanded',str(open));$('#filter-body').hidden=!open});
$('#details-button').addEventListener('click',()=>{const p=$('#details-panel'),open=!p.classList.contains('open');p.classList.toggle('open',open);$('#details-button').setAttribute('aria-expanded',str(open));setInspectorAccess(open);if(open)$('#close-details').focus()});$('#close-details').addEventListener('click',closeInspector);
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&$('#details-panel').classList.contains('open'))closeInspector()});document.querySelectorAll('[role=tab]').forEach(tab=>{tab.addEventListener('click',()=>activateTab(tab,false));tab.addEventListener('keydown',e=>{const tabs=[...document.querySelectorAll('[role=tab]')],i=tabs.indexOf(tab);let n=null;if(e.key==='ArrowRight')n=tabs[(i+1)%tabs.length];if(e.key==='ArrowLeft')n=tabs[(i-1+tabs.length)%tabs.length];if(e.key==='Home')n=tabs[0];if(e.key==='End')n=tabs.at(-1);if(n){e.preventDefault();activateTab(n,true)}})});
$('#export-button').addEventListener('click',async()=>{const banner=el.banner,show=(text,error)=>{const n=make('div','notice '+(error?'error':'success'),text);banner.prepend(n)};if(demoActive){show('Illustrative demo export is disabled to avoid mixing it with retained evidence.',true);return}const api=window.BenchTrace;try{if(api&&typeof api.exportSnapshot==='function'){await api.exportSnapshot();show('Snapshot export requested from host.');return}if(api&&typeof api.downloadSession==='function'&&selectedId){const e=eventMap.get(str(selectedId)),s=sessionMap.get(str(e&&e.sessionId)),u=s&&await api.downloadSession(s.id);if(u){const a=document.createElement('a');a.href=u;a.download='bench-trace.jsonl';a.click();show('Retained session download requested.');return}}throw new Error('Offline export is unavailable from this host.')}catch(err){console.error('Bench Trace export failed',err);show('Export failed: '+str(err&&err.message||err),true)}});
window.addEventListener('error',e=>console.error('Bench Trace initialization/runtime error',e.error||e.message));const inspectorMedia=matchMedia('(max-width:980px)'),filterMedia=matchMedia('(max-width:650px)');inspectorMedia.addEventListener?.('change',()=>setInspectorAccess($('#details-panel').classList.contains('open')));if(filterMedia.matches){$('#filter-body').hidden=true;$('#filter-toggle').setAttribute('aria-expanded','false')}setInspectorAccess(false);render();
})();
</script>
</body>
</html>
'''

if __name__ == '__main__':
    raise SystemExit(main())
