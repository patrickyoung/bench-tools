#!/usr/bin/env python3
"""Offline public fix selection, external action and independent check contracts."""

import argparse
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import threading


ROOT = Path(__file__).resolve().parents[1]
SUPPORT = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
require, invoke = SUPPORT["require"], SUPPORT["invoke"]
SELECT = ROOT / "tools/weigh/examples/improve-checks/select-fix.py"


@contextmanager
def decisions():
    """Return a native non-argmax choice, or one explicit protocol failure."""
    calls, response = [], {"action": "targeted_repair", "mode": "valid"}

    class Fixture(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((self.path, request, self.headers.get("Authorization")))
            if response["mode"] == "unavailable":
                self.send_response(503)
                payload = {"error": {"message": "offline fixture unavailable"}}
            else:
                answers = {}
                for key, question in request["questions"].items():
                    options = question["criteria"]
                    probabilities = {name: 0 for name in options}
                    probabilities.update(targeted_repair=.2, no_change=.8)
                    answer = {"type": "choice", "choice": response["action"],
                              "probabilities": probabilities, "confidence": .73}
                    if response["mode"] == "missing_distribution":
                        del answer["probabilities"]
                    answers[key] = answer
                payload = {"model": "typesafe/fixture-resolved", "answers": answers,
                           "usage": {"input_tokens": 12, "output_tokens": 3, "cost": .000002}}
                self.send_response(200)
            wire = json.dumps(payload).encode()
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(wire)))
            self.end_headers()
            self.wfile.write(wire)

    server = HTTPServer(("127.0.0.1", 0), Fixture)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:%s/api/alpha/decisions" % server.server_port, calls, response
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    for name in ("weigh", "ask", "record"):
        require((bins / name).is_file(), "missing binary: " + name)
    require(SELECT.is_file(), "missing optional fix selector")
    with tempfile.TemporaryDirectory(prefix="fix-selection-contract-") as tmp:
        root = Path(tmp).resolve()
        env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "BENCH_REPLAY_RECORD_DIR", "BENCH_REPLAY_BIN_DIR")
               if key in os.environ}
        env.update(HOME=str(root), PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
                   PYTHONDONTWRITEBYTECODE="1")
        snapshot = {"version": 1, "id": "one-explicit-check", "task": "Return the sum as JSON with only total.",
                    "candidate": '{"total":99}', "evidence": {"values": [2, 3]},
                    "check": {"verdict": "reject", "findings": ["The total does not match the source values."],
                              "source": "caller-selected-check"},
                    "actions": {"mechanical_repair": "Remove a JSON fence without changing values.",
                                "targeted_repair": "Recompute the total from the supplied source values.",
                                "gather_evidence": "Read the explicitly available supplementary evidence.",
                                "stronger_repair": "Request a repair from the separately selected guide model.",
                                "investigate_check": "Review whether this check applies to the requirement.",
                                "no_change": "Keep the candidate unchanged for the independent final check."}}
        raw = (json.dumps(snapshot, indent=2) + "\n").encode()
        source = root / "snapshot.json"
        source.write_bytes(raw)
        digest = hashlib.sha256(raw).hexdigest()
        rules = root / "rules.json"

        def write_rules(action, input_sha256=digest):
            rules.write_text(json.dumps({"version": 1, "input_sha256": input_sha256,
                                         "action": action, "reason": "Explicit controller rule for this snapshot."}) + "\n")

        common = [sys.executable, SELECT, "--records", root / "selections",
                  "--ask", bins / "ask", "--record", bins / "record", "--weigh", bins / "weigh"]
        missing = [sys.executable, SELECT, "--records", root / "rules-only", "--rules", rules,
                   "--ask", root / "missing-ask", "--weigh", root / "missing-weigh",
                   "--record", root / "missing-record"]
        write_rules("mechanical_repair")
        ruled = json.loads(invoke(missing, cwd=root, env=env, data=raw).stdout)
        require(ruled["selector"] == "rule" and ruled["action"] == "mechanical_repair"
                and ruled["input_sha256"] == digest and ruled["requires_final_check"] is True,
                "explicit rule did not select a snapshot-bound action")
        require(ruled["model"] is None and ruled["inference"] is None and ruled["record"] is None,
                "rule-only selection invented inference evidence")
        require(ruled["rules_sha256"] == hashlib.sha256(rules.read_bytes()).hexdigest(),
                "rule evidence hash did not bind the selected rule bytes")
        for action, binding in (("unavailable_action", digest), ("mechanical_repair", "0" * 64)):
            write_rules(action, binding)
            failed = invoke(missing, cwd=root, env=env, data=raw, code=2)
            require(not failed.stdout, "invalid rule emitted a usable action")
        write_rules(None)
        failed = invoke(missing, cwd=root, env=env, data=raw, code=2)
        require(not failed.stdout, "undecided rules bypassed explicit semantic-call opt-in")

        action_program = root / "action.py"
        action_program.write_text(
            "import json, pathlib, sys\n"
            "action, mode, destination, log = sys.argv[1:]\n"
            "with open(log, 'a') as out: out.write(action + '\\n')\n"
            "if action == 'targeted_repair':\n"
            "    pathlib.Path(destination).write_text(json.dumps({'total': 5 if mode == 'good' else 99}))\n"
            "elif action != 'no_change':\n"
            "    raise SystemExit(2)\n")
        checker_program = root / "final-check.py"
        checker_program.write_text(
            "import json, pathlib, sys\n"
            "artifact, evidence, log = sys.argv[1:]\n"
            "value = json.loads(pathlib.Path(artifact).read_text())\n"
            "numbers = json.loads(pathlib.Path(evidence).read_text())['values']\n"
            "accepted = (type(value) is dict and set(value) == {'total'}\n"
            "            and type(value['total']) is int and value['total'] == sum(numbers))\n"
            "with open(log, 'a') as out: out.write(('accept' if accepted else 'reject') + '\\n')\n"
            "print(json.dumps({'verdict': 'accept' if accepted else 'reject'}))\n"
            "raise SystemExit(0 if accepted else 1)\n")
        evidence = root / "final-evidence.json"
        evidence.write_text(json.dumps(snapshot["evidence"]))
        action_log, check_log = root / "actions.log", root / "checks.log"
        replays = []

        def finish(selection, label, repair_mode, accepted):
            # This is the caller's literal executable map, outside the selector.
            # Selection success never determines the independent check verdict.
            require(selection["status"] == "selected" and selection["requires_final_check"] is True,
                    "selector blurred action selection and artifact acceptance")
            artifact = root / (label + ".json")
            artifact.write_text(snapshot["candidate"])
            commands = {name: [sys.executable, action_program, name, repair_mode, artifact, action_log]
                        for name in ("targeted_repair", "no_change")}
            selected_command = commands[selection["action"]]
            action_record, check_record = root / (label + ".action.jsonl"), root / (label + ".check.jsonl")
            invoke([bins / "record", "run", "-ask", bins / "ask", "-f", action_record,
                    "--", *selected_command], cwd=root, env=env)
            checked = invoke([bins / "record", "run", "-ask", bins / "ask", "-f", check_record,
                              "--", sys.executable, checker_program, artifact, evidence, check_log],
                             cwd=root, env=env, code=0 if accepted else 1)
            require(json.loads(checked.stdout)["verdict"] == ("accept" if accepted else "reject"),
                    "selection bypassed final artifact verification")
            replays.append((check_record, checked.stdout, checked.returncode))

        write_rules("no_change")
        rule_unchanged = json.loads(invoke(missing, cwd=root, env=env, data=raw).stdout)
        finish(rule_unchanged, "rule-unchanged", "good", False)
        inference_replays = []
        with decisions() as (endpoint, calls, response):
            live_env = dict(env, OPENROUTER_API_KEY="offline-key")
            weigh = [*common, "--backend", "weigh", "--model", "openrouter/typesafe/fixture",
                     "--endpoint", endpoint]
            before = len(calls)
            failed = invoke(weigh, cwd=root, env=live_env, data=raw, code=2)
            require(not failed.stdout and len(calls) == before, "semantic call ran without --live")
            failed = invoke([*weigh, "--live", "--record", root / "missing-record"],
                            cwd=root, env=live_env, data=raw, code=2)
            require(not failed.stdout and len(calls) == before, "broken recorder triggered inference or fallback")
            write_rules("mechanical_repair")
            invoke([*missing, "--backend", "weigh", "--model", "openrouter/typesafe/fixture",
                    "--endpoint", endpoint, "--live"], cwd=root, env=live_env, data=raw)
            require(len(calls) == before, "explicit rule needlessly called a provider")
            write_rules(None)
            selected_raw = invoke([*weigh, "--live", "--rules", rules, "--input", source],
                                  cwd=root, env=live_env).stdout
            selected = json.loads(selected_raw)
            require(len(calls) == before + 1, "semantic selection retried or made multiple requests")
            require(calls[-1][0] == "/api/alpha/decisions" and calls[-1][2] == "Bearer offline-key",
                    "Weigh selection did not use the explicit native fixture")
            require(calls[-1][1]["state"] == snapshot and len(calls[-1][1]["questions"]) == 1,
                    "fix selector changed supplied state or asked unrelated questions")
            question = next(iter(calls[-1][1]["questions"].values()))
            require(question["criteria"] == snapshot["actions"], "selector changed the caller's available actions")
            require(selected["selector"] == "weigh" and selected["action"] == "targeted_repair"
                    and selected["input_sha256"] == digest,
                    "selector replaced the native non-argmax choice or lost snapshot binding")
            answer = next(iter(selected["inference"]["answers"].values()))
            require(answer["probabilities"] == {name: (.2 if name == "targeted_repair" else .8 if name == "no_change" else 0)
                                                 for name in snapshot["actions"]},
                    "selector discarded, fabricated or changed native probabilities")
            require(selected["model"] == selected["inference"]["model"]
                    and selected["model"]["reported"] == "typesafe/fixture-resolved",
                    "selector lost model identity")
            metadata = selected["inference"]["metadata"]
            require(list(metadata["confidence"].values()) == [.73]
                    and metadata["usage"] == {"input_tokens": 12, "output_tokens": 3, "cost": .000002},
                    "selector changed provider confidence or inference usage")
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", selected["record"]], cwd=root, env=env)
            inference_replays.append((selected["record"], selected["inference"]))
            finish(selected, "good-repair", "good", True)
            finish(selected, "bad-repair", "bad", False)

            response["action"] = "no_change"
            unchanged = json.loads(invoke([*weigh, "--live"], cwd=root, env=live_env, data=raw).stdout)
            finish(unchanged, "unchanged", "good", False)
            inference_replays.append((unchanged["record"], unchanged["inference"]))
            for mode, action in (("valid", "unavailable_action"), ("missing_distribution", "targeted_repair"),
                                 ("unavailable", "targeted_repair")):
                response.update(mode=mode, action=action)
                before = len(calls)
                failed = invoke([*weigh, "--live"], cwd=root, env=live_env, data=raw, code=2)
                require(not failed.stdout and len(calls) == before + 1,
                        "failed inference emitted a usable action, retried or fell back")

        # Ask has the same action boundary and needs no Weigh binary or key.
        with SUPPORT["model_fixture"](env, lambda _: '{"action":"targeted_repair"}') as (ask_env, ask_calls):
            selected = json.loads(invoke([*common, "--backend", "ask", "--model", "openai/fixture", "--live",
                                          "--weigh", root / "missing-weigh"], cwd=root, env=ask_env, data=raw).stdout)
            require(len(ask_calls) == 1 and selected["selector"] == "ask"
                    and selected["action"] == "targeted_repair"
                    and selected["inference"] == {"action": "targeted_repair"},
                    "Ask-only selection failed or invented a distribution")
            inference_replays.append((selected["record"], selected["inference"]))
            finish(selected, "ask-repair", "good", True)
        with SUPPORT["model_fixture"](env, lambda _: '{"action":"unavailable_action"}') as (ask_env, ask_calls):
            failed = invoke([*common, "--backend", "ask", "--model", "openai/fixture", "--live",
                             "--weigh", root / "missing-weigh"], cwd=root, env=ask_env, data=raw, code=2)
            require(not failed.stdout and len(ask_calls) == 1, "invalid Ask action retried, fell back or escaped validation")

        require(check_log.read_text().splitlines() == ["reject", "accept", "reject", "reject", "accept"],
                "a selected action bypassed the independent final checker")
        require(action_log.read_text().splitlines() == ["no_change", "targeted_repair", "targeted_repair", "no_change", "targeted_repair"],
                "literal caller action dispatch did not follow valid selections")
        # Both provider fixtures are stopped, and replay has no model credentials.
        recorded_input = None
        for receipt, expected in inference_replays:
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", receipt], cwd=root, env=env)
            replay = invoke([bins / "record", "replay", "-ask", bins / "ask", "-f", receipt], cwd=root, env=env)
            require(json.loads(replay.stdout) == expected, "offline selection replay lost inference evidence")
            request = invoke([bins / "record", "replay", "-ask", bins / "ask", "-f", receipt,
                              "-stream", "stdin"], cwd=root, env=env).stdout
            require(json.loads(request)["state"] == snapshot, "recorded inference omitted the explicit snapshot")
            require(recorded_input is None or recorded_input == request,
                    "Weigh and Ask selection did not receive identical candidate/evidence/action input")
            recorded_input = request
        for receipt, stdout, code in replays:
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", receipt], cwd=root, env=env)
            replay = invoke([bins / "record", "replay", "-ask", bins / "ask", "-f", receipt],
                            cwd=root, env=env, code=code)
            require(replay.stdout == stdout, "offline final-check replay changed output or rejection status")
    print("ok fix selection: zero-call rules, native Weigh choice, optional Ask, explicit failures, external actions, independent checks and offline replay")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print("fix selection integration: " + str(error), file=sys.stderr)
        raise SystemExit(1)
