#!/usr/bin/env python3
"""Offline executable integration for the single-file Trait application.

Uses real Bundle, Trait, Hire, Agent, Ask, Brief, Ply, Cage, Record and Hone.
Only provider responses come from a loopback fixture. Native Cage is required;
missing dependencies fail rather than skip. These checks establish process and
packaging behavior, not model judgment, useful training, or generalization.
"""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import stat
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ("hire", "agent", "ask", "brief", "ply", "cage", "record", "hone")
RECOVERY_LESSON = ("TRAIT_RECOVERY_LESSON. For this packet fixture, generate answer.txt from input.txt under the selected mode "
                   "instead of substituting a diagnostic string; preserve the trailing newline.")
sys.dont_write_bytecode = True


def load_helper(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


contracts = load_helper("trait_executable_contracts", ROOT / "scripts/check-integration.py")
installer = load_helper("trait_package_verification", ROOT / "scripts/install_support.py")
invoke, require = contracts.invoke, contracts.require


def put(path, data, executable=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data.encode() if isinstance(data, str) else data)
    path.chmod(0o755 if executable else 0o644)


def tree(directory):
    return {p.relative_to(directory).as_posix(): (stat.S_IMODE(p.stat().st_mode),
            hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else "directory")
            for p in directory.rglob("*")}


def inventory_digest(entries):
    value = hashlib.sha256()
    for entry in sorted(entries, key=lambda item: item["path"]):
        value.update(f'{entry["path"]}\0{int(entry["mode"], 8):o}\0{entry["sha256"]}\0'.encode())
    return value.hexdigest()


def text_values(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, list):
        for child in value:
            yield from text_values(child)
    elif isinstance(value, dict):
        for child in value.values():
            yield from text_values(child)


def review_packet(request, required=None):
    """Read the JSON evidence actually sent through the public Ask request."""
    required = required or {"proposal_sha256", "exact_proposal", "qualifying_evidence"}
    decoder = json.JSONDecoder()
    for text in text_values(request):
        if not all('"' + key + '"' in text for key in required):
            continue
        for match in re.finditer(r"\{", text):
            try:
                value, _ = decoder.raw_decode(text[match.start():])
            except json.JSONDecodeError:
                continue
            if isinstance(value, dict) and required <= value.keys():
                return value
    raise RuntimeError("exact-proposal review did not receive its selected evidence packet")


def stage_runtime(bins, prefix, names=RUNTIME):
    (prefix / "bin").mkdir(parents=True)
    for name in names:
        binary = (bins / name).resolve()
        candidates = (binary.parent.parent, bins.parent / "tools" / name,
                      bins.parent / "lib/bench-tools" / name)
        package = next((p for p in candidates if (p / "package.json").is_file()), None)
        require(package is not None, f"missing independent package receipt for {binary}")
        receipt = installer.verify_package(package, name)
        require((package / "bin" / name).read_bytes() == binary.read_bytes(), f"selected {name} differs from package")
        shutil.copytree(package, prefix / "lib/bench-tools" / name)
        for command in receipt["commands"]:
            (prefix / "bin" / command).symlink_to(f"../lib/bench-tools/{name}/bin/{command}")


EXPERT_FILES = {
    "AGENTS.md": "# Packet worker\nTRAIT_FIXTURE_WORKER. Read input.txt and mode.txt. Preserve exact bytes unless the requested mode is upper.\n",
    "README.md": "# Packet worker\nRun Agent with input.txt and mode.txt; answer.txt is the result.\n"
                 "The executable check compares exact bytes or ASCII uppercase according to mode.\n"
                 "Positive: exact mode preserves pear. Negative: silently replacing pear with apple is rejected.\n"
                 "Requires a POSIX shell, cmp and tr. Fixture-only process evidence; no semantic-quality claim.\n",
    "skills/packet-work/SKILL.md": "---\nname: packet-work\ndescription: Preserve selected packet bytes. Use when asked to copy or uppercase a supplied packet.\n---\n"
                                  "TRAIT_FIXTURE_SKILL. Work only on the current selected input.\n",
    "bin/check": "#!/bin/sh\nset -eu\ntest -f answer.txt\n"
                 "if [ \"$(cat mode.txt)\" = upper ]; then\n"
                 "  LC_ALL=C tr '[:lower:]' '[:upper:]' < input.txt | cmp -s - answer.txt\n"
                 "else cmp -s input.txt answer.txt; fi\n",
}


def files_action(files):
    lines = []
    for index, (path, content) in enumerate(files.items()):
        delimiter = f"TRAIT_FIXTURE_FILE_{index}"
        lines.extend(("mkdir -p " + shlex.quote(str(Path(path).parent)),
                      f"cat > {shlex.quote(path)} <<'{delimiter}'", content.rstrip("\n"), delimiter))
    return "\n".join(lines)


def create_action():
    files = {"expert/" + path: content for path, content in EXPERT_FILES.items()}
    files["CAPABILITIES.md"] = "# Fixture capabilities\nByte copying and ASCII case conversion use Agent, Brief, POSIX tools and exact executable checks.\n"
    return files_action(files) + "\nchmod 755 expert/bin/check"


TEAM_FILES = {
    "AGENTS.md": "# Packet team\nTRAIT_FIXTURE_TEAM_ROOT. Use bin/run for the whole two-worker task; a root Agent run is not this team's entry.\n",
    "README.md": "# Packet team fixture\nInvoke bin/run GOAL_FILE WORK STATE EVIDENCE LITERAL. It calls independent writer and reviewer Agents.\n"
                 "tools/upper-packet is a native C helper compiled by tools/build for the current host. It uppercases stdin bytes to stdout.\n"
                 "Requires a POSIX shell, Agent, cmp, cp, mkdir and tr at runtime; cc at build time. Both child acceptance checks and final independent cases are required.\n"
                 "Fixture-only execution evidence; no semantic quality claim. Positive: pear becomes PEAR. Negative: unchanged pear is rejected.\n",
    "bin/check": "#!/bin/sh\nset -eu\ntest -f entry-ran.txt\ntest -f completed.txt\nLC_ALL=C tr '[:lower:]' '[:upper:]' < input.txt | cmp -s - answer.txt\n",
    "src/upper-packet.c": '#include <stdio.h>\nint main(void) { int c; while ((c = getchar()) != EOF) { if (c >= \'a\' && c <= \'z\') c -= \'a\' - \'A\'; if (putchar(c) == EOF) return 7; } if (ferror(stdin) || fflush(stdout)) return 7; fputs("TRAIT_NATIVE_HELPER_RAN\\n", stderr); return 0; }\n',
    "tools/build": "#!/bin/sh\nset -eu\ncd \"$(dirname \"$0\")\"\ncc -std=c99 -Wall -Wextra -Werror ../src/upper-packet.c -o upper-packet\n",
    "bin/run": r'''#!/bin/sh
set -eu
test "$#" -eq 5
goal=$1 work=$2 state=$3 evidence=$4
test "$5" = 'literal $(touch TRAIT_ARG_EXECUTED)'
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd -P)
test -x "$root/tools/upper-packet" || exit 127
mkdir -p "$work/writer" "$work/reviewer" "$state/writer" "$state/reviewer"
printf 'actual team entry\n' > "$work/entry-ran.txt"
cp "$work/input.txt" "$work/writer/input.txt"
export TRAIT_TEAM_EVIDENCE="$evidence" TRAIT_OTHER_WORK="$work/reviewer"
agent run -B -C "$work/writer" -state "$state/writer" -evidence "$evidence/writer" \
  -m openai/fixture -turns 4 -timeout 2m -goal-file "$goal" \
  -record-input input.txt -record-output packet.txt "$root/agents/writer" < /dev/null >&2
cp "$work/writer/packet.txt" "$work/reviewer/packet.txt"
if [ -f "$work/stop.txt" ]; then cp "$work/stop.txt" "$work/reviewer/stop.txt"; fi
export TRAIT_OTHER_WORK="$work/writer"
agent run -B -C "$work/reviewer" -state "$state/reviewer" -evidence "$evidence/reviewer" \
  -m openai/fixture -turns 4 -timeout 2m -goal-file "$goal" \
  -record-input packet.txt -record-output answer.txt "$root/agents/reviewer" < /dev/null >&2
cp "$work/reviewer/answer.txt" "$work/answer.txt"
printf 'fresh model action\n' > "$work/completed.txt"
"$root/bin/check"
cat "$work/answer.txt"
''',
}
for _role in ("writer", "reviewer"):
    TEAM_FILES[f"agents/{_role}/AGENTS.md"] = (f"# {_role}\nTRAIT_TEAM_{_role.upper()}. "
                                               + ("Use ../../tools/upper-packet on input.txt to produce packet.txt.\n" if _role == "writer"
                                                  else "Use only the handed-off packet.txt to produce answer.txt.\n"))
    TEAM_FILES[f"agents/{_role}/README.md"] = "# Fixture specialist\nIndependent Agent definition with an exact-byte check.\n"
    TEAM_FILES[f"agents/{_role}/skills/packet-work/SKILL.md"] = EXPERT_FILES["skills/packet-work/SKILL.md"]
    TEAM_FILES[f"agents/{_role}/bin/check"] = ("#!/bin/sh\nset -eu\n" +
        ("test -f packet.txt\nLC_ALL=C tr '[:lower:]' '[:upper:]' < input.txt | cmp -s - packet.txt\n" if _role == "writer"
         else "test ! -f stop.txt || exit 7\ntest -f answer.txt\ncmp -s packet.txt answer.txt\n"))


def team_create_action():
    files = {"expert/" + path: content for path, content in TEAM_FILES.items()}
    files["CAPABILITIES.md"] = "# Fixture capabilities\nA compiled C transformation, two separate Agent specialists, explicit packet handoff and executable checks form this bounded team.\n"
    files["evaluation/run.json"] = json.dumps({"entry": "bin/run", "args": ["{goal}", "{work}", "{state}", "{evidence}",
                                                                                  "literal $(touch TRAIT_ARG_EXECUTED)"],
                                               "requires": ["agent", "cmp", "cp", "mkdir", "tr"]}) + "\n"
    return (files_action(files) + "\nchmod 755 expert/bin/* expert/tools/build expert/agents/*/bin/check\n"
            "expert/tools/build\nexpert/tools/upper-packet < /dev/null")


def case(packet, name, purpose, *, upper=False, reject=False, check_exit=None, payload=None):
    root = packet / "cases" / name
    data = payload if payload is not None else b'pear apple $(touch CASE_TEXT_EXECUTED)\n'
    expected = data.upper() if upper else data
    put(root / "goal.md", f"TRAIT_CASE_{name}. Process the selected packet and verify answer.txt.\n")
    put(root / "case.json", json.dumps({"purpose": purpose, "outputs": ["answer.txt", "completed.txt"]}) + "\n")
    put(root / "input/input.txt", data)
    put(root / "input/mode.txt", "upper\n" if upper else "exact\n")
    # Deliberately satisfy the worker's pre-check before work. Trait must still
    # start a fresh model action with -B and produce completed.txt.
    put(root / "input/answer.txt", expected)
    put(root / "expected.txt", b"independently rejected expectation\n" if reject else expected)
    put(root / "completed.txt", b"fresh model action\n")
    checker = "#!/bin/sh\nset -eu\ntest \"$#\" -eq 0\n"
    checker += "if printf forbidden > answer.txt; then exit 97; fi\n"
    checker += "if printf forbidden > \"$TRAIT_CASE_DIR/expected.txt\"; then exit 98; fi\n"
    checker += "cmp -s completed.txt \"$TRAIT_CASE_DIR/completed.txt\"\n"
    checker += f"exit {check_exit}\n" if check_exit is not None else "cmp -s answer.txt \"$TRAIT_CASE_DIR/expected.txt\"\n"
    put(root / "check", checker, True)


def evaluation_action(scratch, team=False, refresh=False):
    template = scratch / ("refresh evaluation template" if refresh else ("team evaluation template" if team else "worker evaluation template"))
    if not template.exists():
        template.mkdir()
        if refresh:
            case(template, "transfer-fresh", "transfer", upper=True, payload=b"Fresh plum packet, 42!\n")
        else:
            case(template, "regression", "regression", upper=team)
            case(template, "transfer", "transfer", upper=True)
        if team:
            for checker in (template / "cases").glob("*/check"):
                text = checker.read_text().replace("set -eu\n", "set -eu\ntest -f entry-ran.txt\n")
                put(checker, text, True)
    files = {"evaluation/" + p.relative_to(template).as_posix(): p.read_text()
             for p in sorted(template.rglob("*")) if p.is_file()}
    return files_action(files) + "\nchmod 755 evaluation/cases/*/check"


def packet(directory, expert=None, request=None, training=None, cases=False):
    directory.mkdir()
    if expert:
        shutil.copytree(expert, directory / "expert")
        capabilities = expert.parent / "CAPABILITIES.md"
        put(directory / "CAPABILITIES.md", capabilities.read_bytes() if capabilities.is_file()
            else b"# Fixture capabilities\nSelected byte-processing definition and exact checks.\n")
    if request:
        put(directory / "REQUEST.md", request + "\n")
    if training:
        put(directory / "train.json", json.dumps(training) + "\n")
    if cases:
        case(directory, "regression", "regression")
        case(directory, "transfer", "transfer", upper=True)
    return directory


def verify_records(bins, reports, scratch, env):
    receipt_count = 0
    conversation_count = 0
    case_work = set()
    case_sessions = set()
    for report in reports:
        records = Path(report["records"])
        require(records.is_dir() and not records.is_relative_to(Path(report["output"])), "controller records overlap output")
        for process in report["processes"]:
            require(Path(process["stdout"]).is_file() and Path(process["stderr"]).is_file(), "missing separate subordinate streams")
            require(Path(process["program"]).is_absolute(), "process executable identity was not resolved")
        for item in report["cases"]:
            require(item["work"] not in case_work, "separate evaluation cases reused mutable work")
            case_work.add(item["work"])
            sessions = [p for p in Path(item["evidence"]).rglob("*.jsonl") if p.parent.name == "runs"]
            expected = 1 if report.get("entry") == "agent" else (2 if report.get("entry") == "bin/run" else 0)
            if item.get("entry_exit", item["agent_exit"]) == 0:
                require(len(sessions) == expected, "case lacks distinct actual worker conversations")
            require(all(str(p) not in case_sessions for p in sessions), "case reused a model conversation")
            case_sessions.update(map(str, sessions))
        for session in records.rglob("*.jsonl"):
            replay = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=scratch, env=env)
            events = [json.loads(line) for line in replay.stdout.splitlines()]
            if any(e["type"] == "assistant" for e in events):
                conversation_count += 1
            if any(e["type"] == "note" and e["data"].get("kind") == "record.intent/v1" for e in events):
                terminals = [e["data"]["body"] for e in events if e["type"] == "note"
                             and e["data"].get("kind") == "record.terminal/v1"]
                complete = len(terminals) == 1 and terminals[0].get("complete") is True
                require(complete or report["exit"] == 125, "successful run retained an incomplete process receipt")
                invoke([bins / "record", "check", "-ask", bins / "ask", "-f", session], cwd=scratch, env=env,
                       code=0 if complete else 125)
                receipt_count += int(complete)
    require(conversation_count >= 8 and receipt_count >= 20, "missing real author/case conversations or process receipts")


def check(bins, scratch, env):
    prefix = scratch / "runtime source"
    stage_runtime(bins, prefix)
    # The packager receives a disposable copy of its application core and
    # builder, so the resulting app cannot depend on those original files.
    packaging_prefix = scratch / "packaging commands"
    stage_runtime(bins, packaging_prefix, ("trait", "bundle"))
    packaging_bins = packaging_prefix / "bin"
    application = scratch / "single trait executable"
    invoke([sys.executable, ROOT / "scripts/package-trait", "--runtime", prefix,
            "--output", application, "--bin-dir", packaging_bins], cwd=scratch, env=env)
    shutil.rmtree(prefix)
    shutil.rmtree(packaging_prefix)
    app_env = dict(env, PATH=os.defpath)
    info = invoke([application, "--bundle-info"], cwd=scratch, env=app_env)
    require(json.loads(info.stdout)["app"]["interface"] == "argv", "Trait distribution lost application-owned argv")
    require(b"trait - create" in invoke([application, "help"], cwd=scratch, env=app_env).stdout, "Trait help was intercepted")
    reports = []
    mode = {}
    worker_requests = []
    reviewed_proposals = []
    capability_reviews = []
    fixture_errors = []

    def respond(request):
        instructions = request.get("instructions", "")
        if "You are choosing which skill" in instructions:
            return "packet-work" if "packet-work" in json.dumps(request) else "checking-deliverables"
        if any("Review Trait capability and evaluation before execution." in text for text in text_values(request)):
            selected = review_packet(request, {"definition_sha256", "evaluation_sha256", "definition", "evaluation", "execution"})
            require(selected.get("capabilities") and selected.get("evaluation_origin") in ("supplied", "generated", "mixed"),
                    "capability review lost its assessment or case origin")
            require(selected.get("entry_boundary") in ("agent", "host", "cage"), "review lost the selected execution authority")
            for kind in ("definition", "evaluation"):
                require(selected[kind], "capability review omitted " + kind)
                require(inventory_digest(selected[kind]) == selected[kind + "_sha256"],
                        "review inventory does not match the selected " + kind + " digest")
                for entry in selected[kind]:
                    require(entry["bytes"] >= 0 and entry["mode"] and entry["sha256"], "review lost file identity")
                    if "text" in entry:
                        raw = entry["text"].encode()
                        require(len(raw) == entry["bytes"] and hashlib.sha256(raw).hexdigest() == entry["sha256"],
                                "review did not receive exact textual file bytes")
                    else:
                        require(entry.get("binary") is True, "review hid a non-text file without identifying it")
            require(any(entry["path"].endswith("/check") for entry in selected["evaluation"]), "review omitted the actual independent check")
            capability_reviews.append(selected)
            mode["capability_reviews"] += 1
            return json.dumps({"approved": mode["name"] != "review-reject", "reason": "Offline source inspection fixture; no human approval or quality certification."})
        if instructions.startswith("You are reading one recorded run of an agent."):
            require(mode["name"] == "recovery-train", "unexpected Hone wording call")
            require("wrong fixture bytes" in json.dumps(request), "Hone wording lost the actual rejected operation")
            mode["wording_calls"] += 1
            return "- " + RECOVERY_LESSON
        if any("Review this exact proposed worker lesson" in text for text in text_values(request)):
            require(mode["name"] == "recovery-train", "unexpected exact-proposal review")
            selected = review_packet(request)
            match = re.search(r"^artifact: (.+)$", selected["exact_proposal"], re.MULTILINE)
            require(match is not None, "Hone show did not identify the reviewed artifact")
            path = Path(match.group(1))
            proposal_bytes = path.read_bytes()
            proposal = json.loads(proposal_bytes)
            require(hashlib.sha256(proposal_bytes).hexdigest() == selected["proposal_sha256"],
                    "Ask reviewed a different proposal digest")
            require(proposal["document"] in selected["exact_proposal"] and RECOVERY_LESSON in proposal["document"],
                    "Ask review omitted exact proposed skill bytes")
            require("wrong fixture bytes" in selected["qualifying_evidence"] and selected["scope"] == "general",
                    "Ask review lost actual recovery evidence or selected scope")
            before = Path(proposal["target"]).read_bytes()
            require(RECOVERY_LESSON.encode() not in before, "Hone changed the target before proposal review")
            require(selected["existing_instructions"]["skills/packet-work/SKILL.md"].encode() == before,
                    "Ask review did not receive the unchanged target instructions")
            reviewed_proposals.append({"path": path, "bytes": proposal_bytes, "document": proposal["document"], "before": before})
            mode["review_calls"] += 1
            return json.dumps({"approved": True, "reason": "Offline fixture approves these exact supported bytes; this is model review, not human approval."})
        if "You are Hire's expert builder" in instructions:
            if any("Prepare evaluation cases for Trait" in text for text in text_values(request)):
                mode["evaluation_turns"] += 1
                if mode["evaluation_turns"] % 2 == 0:
                    return "The fixture cases are prepared outside the unchanged copied definition."
                prepared = evaluation_action(scratch, mode["name"].startswith("team"), mode["name"] == "refresh-train")
                prepared += "\nprintf '# Fixture capabilities\\nSelected byte contract, actual tools and exact fresh cases.\\n' > CAPABILITIES.md"
                prepared += "\nprintf '# Fixture cases\\nVisible authored regression examples, not independent model quality evidence.\\n' > evaluation/CASES.md"
                return "```ply\n" + prepared + "\n```"
            mode["author_turns"] += 1
            if mode["author_turns"] % 2 == 0:
                return "The fixture definition is authored; structural validation is separate from case quality."
            if mode["name"] == "create":
                action = create_action()
            elif mode["name"] == "team-create":
                action = team_create_action()
            elif mode["name"] == "edit":
                action = ("printf '\\nTRAIT_EDITED. Preserve the selected byte contract.\\n' >> expert/AGENTS.md\n"
                          "printf '# Fixture capabilities\\nPreserve exact byte contracts and selected uppercase tools.\\n' > CAPABILITIES.md")
            elif mode["name"] in ("train", "bad-train", "auto-train", "refresh-train"):
                action = "printf '\\nTRAIT_TRAINED_UPPERCASE. Use ASCII uppercase in upper mode, otherwise preserve exact bytes.\\n' >> expert/AGENTS.md"
                if mode["name"] == "bad-train":
                    action += "\nprintf '#!/bin/sh\\nexit 0\\n' > expert/bin/check"
            else:
                raise RuntimeError("unexpected creator call during " + mode["name"])
            return "```ply\n" + action + "\n```"
        role = next((name for name in ("writer", "reviewer") if "TRAIT_TEAM_" + name.upper() in instructions), None)
        if role:
            other = "reviewer" if role == "writer" else "writer"
            require("TRAIT_TEAM_" + other.upper() not in instructions and "TRAIT_FIXTURE_TEAM_ROOT" not in instructions,
                    "team child inherited another definition's context")
            require("TRAIT_FIXTURE_SKILL" in instructions and "You are Hire's expert builder" not in instructions,
                    "team child lost Brief selection or inherited authoring context")
            require(len(set(re.findall(r"TRAIT_CASE_[a-z-]+", json.dumps(request)))) == 1,
                    "team child reused another case history")
            mode[role + "_turns"] += 1
            mode["worker_turns"] += 1
            worker_requests.append(request)
            if mode[role + "_turns"] % 2 == 0:
                return f"The fixture {role} contribution is checked."
            action = ('if printf forbidden > "$AGENT_HOME/forbidden-child-write"; then exit 99; fi\n'
                      'if printf forbidden > "$TRAIT_TEAM_EVIDENCE/forbidden-child-record"; then exit 99; fi\n'
                      'if printf forbidden > "$TRAIT_OTHER_WORK/forbidden-sibling-write"; then exit 99; fi\n')
            action += ('"$AGENT_HOME/../../tools/upper-packet" < input.txt > packet.txt\n' if role == "writer"
                       else 'cp packet.txt answer.txt\n')
            return "```ply\n" + action + "\n```"
        require("TRAIT_FIXTURE_WORKER" in instructions and "TRAIT_FIXTURE_SKILL" in instructions,
                "fresh case lost the worker definition or Brief-selected procedure")
        require("You are Hire's expert builder" not in instructions, "case inherited the creator's model context")
        if mode["name"] not in ("create", "edit"):
            require("TRAIT_TRAINED_UPPERCASE" in instructions, "case did not load the selected taught definition")
        if mode["name"] == "recovery-train":
            require(RECOVERY_LESSON in instructions, "fresh recovery-retention case omitted the admitted lesson")
        require(len(set(re.findall(r"TRAIT_CASE_[a-z-]+", json.dumps(request)))) == 1,
                "case inherited another assignment's model history or lost its current goal")
        worker_requests.append(request)
        mode["worker_turns"] += 1
        if mode["name"] == "recovery-seed" and mode["worker_turns"] == 1:
            return "```ply\nprintf 'wrong fixture bytes\\n' > answer.txt\nprintf 'fresh model action\\n' > completed.txt\ncmp input.txt answer.txt\n```"
        if mode["name"] == "recovery-seed" and mode["worker_turns"] == 2:
            return "First fixture candidate has deliberately wrong bytes."
        if mode["worker_turns"] % 2 == 0:
            return "The selected current packet is checked."
        return ("```ply\n"
                'if printf forbidden > "$AGENT_HOME/forbidden-definition"; then exit 99; fi\n'
                "if [ \"$(cat mode.txt)\" = upper ]; then LC_ALL=C tr '[:lower:]' '[:upper:]' < input.txt > answer.txt; "
                "else cp input.txt answer.txt; fi\n"
                "printf 'fresh model action\\n' > completed.txt\n```")

    def fixture_response(request):
        try:
            return respond(request)
        except (RuntimeError, ValueError, OSError, KeyError) as error:
            # Keep fixture assertion failures local. Closing the HTTP request
            # would instead exercise Ask's transport retries and hide the cause.
            fixture_errors.append(str(error))
            return "none"

    with contracts.model_fixture(app_env, fixture_response) as (fixture_env, calls):
        def action(name, source, output_name, *, expected=0, fixture_mode=None, entry_boundary=None, request=None):
            before = tree(source)
            mode.update(name=fixture_mode or name, author_turns=0, evaluation_turns=0, worker_turns=0,
                        wording_calls=0, review_calls=0, capability_reviews=0, writer_turns=0, reviewer_turns=0)
            destination = scratch / output_name
            try:
                flags = ["-entry-boundary", entry_boundary] if entry_boundary else []
                if request is not None:
                    flags.extend(("-request", request))
                process = invoke([application, name, "-m", "openai/fixture", "-turns", "4", "-timeout", "2m", *flags, source, destination],
                                 cwd=scratch, env=fixture_env, code=expected)
            except RuntimeError:
                if fixture_errors:
                    raise RuntimeError("provider fixture: " + fixture_errors[0]) from None
                raise
            require(not fixture_errors, "provider fixture: " + "; ".join(fixture_errors))
            require(tree(source) == before, f"{name} mutated the caller's input packet")
            report = json.loads(process.stdout)
            require(process.stdout == (destination / "result.json").read_bytes(), "stdout is not exactly the controller JSON report")
            require(report["exit"] == expected and report["action"] == name, "report differs from observed action/status")
            if mode["capability_reviews"]:
                require(report["evaluated_sha256"] == capability_reviews[-1]["definition_sha256"],
                        "executed definition identity differs from source review")
                require(report["entry_boundary"] == capability_reviews[-1]["entry_boundary"],
                        "executed boundary differs from source review")
            if mode["evaluation_turns"]:
                retained = Path(report["records"]) / "evaluation-author/expert"
                selected = source / "expert" if name == "train" else destination / "expert"
                if name == "eval":
                    selected = Path(report["records"]) / "input/expert"
                require(tree(retained) == tree(selected), "case preparation modified its copied expert")
            require((destination / "EVALUATION.md").is_file(), "missing readable evaluation record")
            reports.append(report)
            return destination, report

        create = packet(scratch / "create request")
        creation_goal = "TRAIT_CREATE_CASE. Create the explicitly bounded packet worker. $(touch REQUEST_EXECUTED)"
        created, report = action("create", create, "worker v1", request=creation_goal)
        require(report["status"] == "accepted_on_cases" and report["structurally_valid"] and report["authored"]
                and not report["retained_on_cases"], "create lost its authoring or fresh-case status")
        require(mode["author_turns"] == 2 and mode["evaluation_turns"] == 2 and mode["worker_turns"] == 4
                and mode["capability_reviews"] == 1, "create skipped authoring, preparation, exact review or fresh execution")
        require(report["evaluation_origin"] == "generated" and (created / "evaluation/cases/transfer/check").is_file()
                and (created / "CAPABILITIES.md").is_file() and (created / "REVIEW.md").is_file(), "create omitted its prepared reusable recipe")
        require((created / "REQUEST.md").read_text() == creation_goal, "literal -request was not retained exactly")
        edit = packet(scratch / "edit request", created / "expert", "TRAIT_EDIT_CASE. Clarify the existing operating instructions.")
        put(edit / "worker.lock.json", '{"fixture":"original-source-pin"}\n')
        edited, report = action("edit", edit, "worker v2")
        require(mode["evaluation_turns"] == 2 and mode["worker_turns"] == 4 and mode["capability_reviews"] == 1,
                "edit did not prepare and exercise its changed definition")
        require("TRAIT_EDITED" in (edited / "expert/AGENTS.md").read_text(), "edit did not publish its new candidate")
        require((Path(report["records"]) / "input/worker.lock.json").read_bytes() == (edit / "worker.lock.json").read_bytes(), "edit lost original source provenance")
        require(not (edited / "worker.lock.json").exists(), "edit published an unchanged source lock for changed bytes")
        train = packet(scratch / "training request", edited / "expert", "TRAIT_TRAIN_CASE. Teach the selected uppercase procedure and preserve exact-mode behavior.",
                       {"kind": "knowledge", "scope": "general"}, True)
        put(train / "sources/lesson.md", "Fixture source, 2026-10-01: upper mode means ASCII uppercase. Other modes preserve exact bytes.\n")
        trained, training_report = action("train", train, "worker v3")
        require(training_report["status"] == "retained_on_cases" and training_report["retained_on_cases"], "train did not report accepted fresh cases")
        require(mode["author_turns"] == 2 and mode["worker_turns"] == 4, "train skipped fresh model work over passing pre-checks")
        require(mode["evaluation_turns"] == 0 and mode["capability_reviews"] == 1, "supplied training recipe was replaced or not reviewed")
        require({x["purpose"] for x in training_report["cases"]} == {"transfer", "regression"}, "training lost case coverage")
        require(all(x["status"] == "accepted" and x["check_exit"] == 0 for x in training_report["cases"]), "training omitted independent acceptance")
        require((trained / "expert/bin/check").read_bytes() == (train / "expert/bin/check").read_bytes(), "knowledge training changed its executable contract")
        print("ok trait: packaged embedded Hire create/edit/knowledge train, source immutability, fresh transfer/regression work", flush=True)

        automatic = packet(scratch / "automatic training request", edited / "expert",
                           "TRAIT_TRAIN_CASE. Teach ASCII uppercase and retain exact-byte mode.")
        _, auto_report = action("train", automatic, "automatic training result", fixture_mode="auto-train")
        require(mode["evaluation_turns"] == 2 and mode["author_turns"] == 2 and mode["worker_turns"] == 4,
                "outcome-only training failed to prepare cases before authoring")
        sequence = [p["name"] for p in auto_report["processes"]]
        require(sequence.index("evaluation-author") < sequence.index("author"), "training cases were authored after the amendment")
        require("Scope: private" in (Path(auto_report["records"]) / "evaluation-goal.md").read_text(),
                "omitted train.json did not use the private knowledge scope")
        refreshed, refresh_report = action("train", created, "fresh training from prior result", fixture_mode="refresh-train",
                                           request="TRAIT_TRAIN_CASE. Teach ASCII uppercase using a fresh transfer input; preserve earlier cases.")
        require(refresh_report["evaluation_origin"] == "mixed" and mode["evaluation_turns"] == 2
                and mode["author_turns"] == 2 and mode["worker_turns"] == 6, "prior-result teaching reused old cases without fresh transfer work")
        for name in ("regression", "transfer"):
            require(tree(refreshed / "evaluation/cases" / name) == tree(created / "evaluation/cases" / name),
                    "fresh teaching rewrote previously selected cases")
        require({c["id"] for c in refresh_report["cases"]} == {"regression", "transfer", "transfer-fresh"},
                "fresh teaching did not execute both retained cases and the new transfer")
        sequence = [p["name"] for p in refresh_report["processes"]]
        require(sequence.index("evaluation-author") < sequence.index("author"), "prior-output cases were not frozen before teaching")

        evaluation = packet(scratch / "evaluation request", trained / "expert", cases=True)
        evaluated, report = action("eval", evaluation, "accepted evaluation")
        require(report["status"] == "accepted_on_cases" and not report["authored"] and not report["retained_on_cases"], "eval claimed authoring or new teaching")
        require(not (evaluated / "expert").exists() and mode["worker_turns"] == 4, "eval changed source or skipped fresh cases")
        _, reused = action("eval", trained, "prior output evaluation")
        require(mode["evaluation_turns"] == 0 and mode["worker_turns"] == 4 and reused["evaluation_origin"] == "supplied",
                "prior output failed to reuse its recipe with fresh work")
        rejected = packet(scratch / "rejected request", trained / "expert")
        case(rejected, "near-miss", "general", reject=True)
        _, report = action("eval", rejected, "rejected evaluation", expected=2)
        require(report["status"] == "rejected_on_cases" and report["cases"][0]["agent_exit"] == 0
                and report["cases"][0]["check_exit"] == 1, "independent rejection was hidden by the worker check")
        broken = packet(scratch / "broken-check request", trained / "expert")
        case(broken, "broken", "general", check_exit=7)
        _, report = action("eval", broken, "broken evaluation", expected=7)
        require(report["cases"][0]["status"] == "broken" and report["cases"][0]["check_exit"] == 7, "broken check was relabeled a task rejection")
        print("ok trait: accepted, rejected and broken independent checks; native read-only check Cage and exact statuses", flush=True)

        _, review_rejected = action("eval", evaluation, "refused source review", expected=2, fixture_mode="review-reject")
        require(review_rejected["status"] == "capability_review_rejected" and mode["worker_turns"] == 0
                and not review_rejected["cases"], "rejected source review reached execution")

        team_request = packet(scratch / "team creation request", request="TRAIT_TEAM_CREATE. Build the fixture two-specialist packet team with a compiled uppercase helper and checked handoff.")
        team_created, team_report = action("create", team_request, "native team", fixture_mode="team-create")
        require(team_report["entry"] == "bin/run" and team_report["status"] == "accepted_on_cases",
                "team creation did not evaluate the actual authored entry")
        require(mode["author_turns"] == 2 and mode["evaluation_turns"] == 2 and mode["writer_turns"] == 4
                and mode["reviewer_turns"] == 4 and mode["capability_reviews"] == 1, "team omitted compilation, preparation or separate child execution")
        helper = team_created / "expert/tools/upper-packet"
        require(helper.read_bytes()[:4] in (b"\x7fELF", b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf", b"\xce\xfa\xed\xfe", b"\xfe\xed\xfa\xce"),
                "creator did not compile an actual host native helper")
        require((team_created / "expert/src/upper-packet.c").is_file() and (team_created / "expert/tools/build").is_file(),
                "native helper lost its reviewable source or build recipe")
        reviewed_helper = next(entry for entry in capability_reviews[-1]["definition"] if entry["path"] == "tools/upper-packet")
        require(reviewed_helper.get("binary") and reviewed_helper["sha256"] == hashlib.sha256(helper.read_bytes()).hexdigest(),
                "native helper was not bound to its exact reviewed identity")
        probe = invoke([helper], cwd=scratch, env=fixture_env, data=b"aBc\n")
        require(probe.stdout == b"ABC\n" and probe.stderr == b"TRAIT_NATIVE_HELPER_RAN\n", "compiled helper violated its public stream contract")
        for item in team_report["cases"]:
            require((Path(item["work"]) / "entry-ran.txt").read_bytes() == b"actual team entry\n", "root Agent substituted for the real team entry")
            entry = next(p for p in team_report["processes"] if p["name"] == "case-" + item["id"] + "-entry")
            require(b"TRAIT_NATIVE_HELPER_RAN" in Path(entry["stderr"]).read_bytes(), "worker contribution did not run the native helper")
            require((Path(item["evidence"]) / "writer/runs").is_dir() and (Path(item["evidence"]) / "reviewer/runs").is_dir(),
                    "team children did not retain separate original Agent evidence")
        _, team_rerun = action("eval", team_created, "native team rerun", fixture_mode="team-eval")
        require(mode["author_turns"] == 0 and mode["evaluation_turns"] == 0 and mode["worker_turns"] == 8,
                "existing team recipe was reauthored or reused old child work")
        if sys.platform == "darwin":
            # Seatbelt cannot be reapplied from inside this outer Cage. Prove
            # the explicit selection fails closed; never disable child Cage.
            _, nested_report = action("eval", team_created, "nested Cage refusal", expected=125,
                                      fixture_mode="team-cage", entry_boundary="cage")
            require(len(nested_report["cases"]) == 1 and nested_report["cases"][0]["entry_exit"] == 125
                    and mode["writer_turns"] == 1 and mode["reviewer_turns"] == 0,
                    "nested boundary failure retried or fell through to unconfined child execution")
            nested_process = next(p for p in nested_report["processes"] if p["name"].endswith("-entry"))
            require(b"confinement setup" in Path(nested_process["stderr"]).read_bytes(), "nested Cage refusal lost its actual diagnostic")

        def team_packet(name):
            selected = packet(scratch / name, team_created / "expert")
            put(selected / "evaluation/run.json", (team_created / "evaluation/run.json").read_bytes())
            return selected

        team_wrong = team_packet("team wrong-result request")
        case(team_wrong, "team-wrong", "general", upper=True, reject=True)
        _, wrong_team_report = action("eval", team_wrong, "team independent rejection", expected=2, fixture_mode="team-reject")
        require(wrong_team_report["cases"][0]["entry_exit"] == 0 and wrong_team_report["cases"][0]["check_exit"] == 1
                and mode["worker_turns"] == 4, "successful parts bypassed the independent team result check")
        team_stopped = team_packet("team failed-child request")
        case(team_stopped, "team-stop", "general", upper=True)
        put(team_stopped / "cases/team-stop/input/stop.txt", "selected broken-check fixture\n")
        # Record can preserve a child's exact nonzero status only when its
        # declared output evidence exists. Missing required bytes correctly make
        # the recording incomplete (125), not a fabricated complete receipt.
        put(team_stopped / "cases/team-stop/input/completed.txt", "preexisting failure fixture\n")
        _, stop_report = action("eval", team_stopped, "team exact child failure", expected=1, fixture_mode="team-stop")
        require(stop_report["cases"][0]["entry_exit"] == 1 and "check_exit" not in stop_report["cases"][0]
                and mode["writer_turns"] == 2 and mode["reviewer_turns"] == 2, "team hid or retried its child's public broken-check status")
        stopped_process = next(p for p in stop_report["processes"] if p["name"].endswith("-entry"))
        require(b"the check is broken: exit 7" in Path(stopped_process["stderr"]).read_bytes(),
                "public Agent status lost the underlying verifier failure diagnostic")
        team_missing = team_packet("team missing-helper request")
        case(team_missing, "team-missing", "general", upper=True)
        put(team_missing / "cases/team-missing/input/completed.txt", "preexisting failure fixture\n")
        (team_missing / "expert/tools/upper-packet").unlink()
        _, missing_report = action("eval", team_missing, "team missing helper", expected=127, fixture_mode="team-missing")
        require(mode["worker_turns"] == 0 and missing_report["cases"][0]["entry_exit"] == 127,
                "team fabricated a native capability or retried a missing prerequisite")
        confined = team_packet("confined native-entry request")
        case(confined, "native-entry", "general", upper=True)
        put(confined / "expert/bin/native-entry", '''#!/bin/sh
set -eu
if printf forbidden >> "$TRAIT_GOAL"; then exit 98; fi
if printf forbidden > "$TRAIT_EXPERT/forbidden-controller-write"; then exit 99; fi
"$TRAIT_EXPERT/tools/upper-packet" < input.txt > answer.txt
printf 'fresh model action\\n' > completed.txt
''', True)
        put(confined / "evaluation/run.json", json.dumps({"entry": "bin/native-entry", "args": [], "requires": []}) + "\n")
        _, confined_report = action("eval", confined, "confined deterministic entry", fixture_mode="native-entry", entry_boundary="cage")
        require(mode["worker_turns"] == 0 and confined_report["cases"][0]["entry_exit"] == 0
                and confined_report["cases"][0]["check_exit"] == 0, "explicit Cage lane failed to run and check a deterministic native entry")
        refusal = team_packet("expected refusal request")
        case(refusal, "refusal", "regression")
        put(refusal / "expert/bin/refuse", "#!/bin/sh\nprintf 'missing specialist\\n' > disposition.txt\nexit 2\n", True)
        put(refusal / "evaluation/run.json", json.dumps({"entry": "bin/refuse", "args": [], "requires": []}) + "\n")
        put(refusal / "cases/refusal/case.json", json.dumps({"purpose": "regression", "outputs": [], "expected_exit": 2}) + "\n")
        put(refusal / "cases/refusal/check", "#!/bin/sh\nset -eu\ntest \"$TRAIT_EXECUTION_EXIT\" = 2\ntest -f \"$TRAIT_EXECUTION_STATUS\"\ntest \"$(cat disposition.txt)\" = 'missing specialist'\ntest ! -e result.json\n", True)
        _, refusal_report = action("eval", refusal, "checked expected refusal", fixture_mode="expected-refusal", entry_boundary="cage")
        require(refusal_report["cases"][0]["entry_exit"] == 2 and refusal_report["cases"][0]["expected_exit"] == 2
                and refusal_report["cases"][0]["check_exit"] == 0 and refusal_report["cases"][0]["status"] == "accepted",
                "expected refusal changed actual entry status or bypassed the independent check")
        put(refusal / "expert/bin/refuse", "#!/bin/sh\nprintf 'missing specialist\\n' > disposition.txt\nexit 0\n", True)
        _, unexpected = action("eval", refusal, "unexpected successful refusal", expected=2, fixture_mode="expected-refusal", entry_boundary="cage")
        require(unexpected["cases"][0]["entry_exit"] == 0 and unexpected["cases"][0]["status"] == "unexpected_success"
                and "check_exit" not in unexpected["cases"][0], "unexpected success was credited as expected refusal")
        print("ok trait: explicit checked refusal, exact status and rejection of unexpected success", flush=True)
        print("ok trait: automatic capability/case preparation, exact source review, real two-Agent team entry, compiled helper, handoff and failure propagation", flush=True)

        bad_output, report = action("train", train, "refused code training", expected=2, fixture_mode="bad-train")
        require(not report["retained_on_cases"] and not report["cases"] and not (bad_output / "expert").exists(), "training published/evaluated an altered checker")
        require("contract" in report.get("error", "") or "executable" in report.get("error", ""), "training refusal lacked its executable boundary")
        missing = packet(scratch / "missing-transfer request", trained / "expert", "Teach this method.", {"kind": "knowledge", "scope": "general"})
        case(missing, "regression", "regression")
        before = len(calls)
        result = invoke([application, "train", "-m", "openai/fixture", missing, scratch / "unadmitted output"],
                        cwd=scratch, env=fixture_env, code=2)
        require(not result.stdout and len(calls) == before and not (scratch / "unadmitted output").exists(), "missing training evidence reached model work")

        # A genuine successful Agent session with no rejected candidate is not
        # recovery evidence. Hone must refuse honestly without a wording call.
        sessions = list((Path(training_report["cases"][0]["evidence"]) / "runs").glob("*.jsonl"))
        require(len(sessions) == 1, "fresh case has no unique Ask conversation")
        session_bytes = sessions[0].read_bytes()
        recovery = packet(scratch / "no-recovery request", trained / "expert", "Learn only from the selected actual checked recovery.",
                          {"kind": "recovery", "scope": "general", "session": str(sessions[0]), "skill": "packet-work"}, True)
        before = len(calls)
        recovered, report = action("train", recovery, "no-recovery result", expected=1)
        require(report["status"] == "no_qualifying_recovery" and not report["authored"] and not report["retained_on_cases"], "Hone no-recovery became a teaching claim")
        require(len(calls) == before and sessions[0].read_bytes() == session_bytes and not (recovered / "expert").exists(), "no-recovery rewrote source, called a model or fell through to authoring")
        require([p["name"] for p in report["processes"]] == ["recovery-inspect"], "unexpected work after Hone refusal")
        print("ok trait: immutable executable training contract, early case admission, real Hone no-recovery refusal without fallback", flush=True)

        # Explicit synthetic offline failure/repair to exercise the positive
        # protocol. Agent/Ply create every real verdict and receipt; no log is
        # edited or fabricated. This is not a claim about useful model learning.
        seed = packet(scratch / "synthetic recovery fixture", trained / "expert")
        case(seed, "synthetic-repair", "general")
        _, seed_report = action("eval", seed, "recorded synthetic recovery", fixture_mode="recovery-seed")
        require(mode["worker_turns"] == 4 and seed_report["cases"][0]["status"] == "accepted",
                "synthetic fixture did not execute the rejected-then-repaired case")
        seed_sessions = list((Path(seed_report["cases"][0]["evidence"]) / "runs").glob("*.jsonl"))
        require(len(seed_sessions) == 1, "synthetic recovery has no unique actual Agent session")
        seed_session = seed_sessions[0]
        seed_bytes = seed_session.read_bytes()
        replay = invoke([bins / "ask", "replay", "-check", "-json", seed_session], cwd=scratch, env=fixture_env)
        verdicts = [event["data"]["body"] for event in map(json.loads, replay.stdout.splitlines())
                    if event["type"] == "note" and event["data"].get("kind") == "ply.verifier/v2"]
        require([v["outcome"] for v in verdicts] == ["rejected", "accepted"], "actual verifier receipts lack the selected recovery")
        require(len({v["verifier_sha256"] for v in verdicts}) == 1 and all(v["phase"] == "candidate" for v in verdicts),
                "synthetic recovery changed its verifier or used a pre-check as rejected work")
        positive = packet(scratch / "positive recovery request", trained / "expert",
                          "Teach the packet-specific method supported by this selected synthetic protocol recovery. Preserve the executable check.",
                          {"kind": "recovery", "scope": "general", "session": str(seed_session), "skill": "packet-work"}, True)
        learned, positive_report = action("train", positive, "recovery-trained candidate", fixture_mode="recovery-train")
        require(positive_report["status"] == "retained_on_cases" and positive_report["retained_on_cases"], "positive recovery did not finish fresh acceptance")
        require(mode["author_turns"] == 0 and mode["wording_calls"] == 1 and mode["review_calls"] == 1 and mode["worker_turns"] == 4,
                "recovery bypassed or repeated wording/review, called Hire, or skipped fresh cases")
        require(len(reviewed_proposals) == 1, "recovery did not review exactly one proposal")
        reviewed = reviewed_proposals[0]
        actual_proposal = Path(positive_report["records"]) / "lesson.json"
        require(actual_proposal == reviewed["path"] and actual_proposal.read_bytes() == reviewed["bytes"], "admission changed the exact reviewed proposal")
        require((learned / "expert/skills/packet-work/SKILL.md").read_bytes() == reviewed["document"].encode(),
                "published lesson differs from the exact reviewed document")
        require((positive / "expert/skills/packet-work/SKILL.md").read_bytes() == reviewed["before"], "recovery mutated its source target")
        require((learned / "expert/bin/check").read_bytes() == (positive / "expert/bin/check").read_bytes(), "recovery altered the executable acceptance contract")
        require(seed_session.read_bytes() == seed_bytes, "recovery altered its original Agent evidence")
        sequence = [p["name"] for p in positive_report["processes"]]
        require(sequence[:7] == ["recovery-inspect", "recovery-prepare", "recovery-show", "recovery-review", "recovery-admit", "recovery-lint", "verify"],
                "recovery did not use the public inspection/preparation/review/admission sequence")
        require(all(c["status"] == "accepted" and c["check_exit"] == 0 for c in positive_report["cases"]), "recovery omitted independent fresh-case acceptance")
        print("ok trait: actual synthetic recovery, Hone prepare/show, exact Ask model review/admit, unchanged check and fresh loaded lesson", flush=True)
        require(not any(scratch.rglob("REQUEST_EXECUTED")) and not any(scratch.rglob("CASE_TEXT_EXECUTED"))
                and not any(scratch.rglob("TRAIT_ARG_EXECUTED")), "literal packet data or entry argv was executed")
        require(all(path == "/v1/responses" for path, _ in calls), "fixture used an unexpected provider route")
    verify_records(bins, reports, scratch, env)
    print("ok trait: original public-command conversations and Record process receipts replay offline", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build/bin")
    bins = parser.parse_args().bin_dir.expanduser().resolve()
    for name in ("trait", "bundle", *RUNTIME):
        require((bins / name).is_file() and os.access(bins / name, os.X_OK), f"missing executable: {bins / name}")
    require((ROOT / "scripts/package-trait").is_file(), "missing scripts/package-trait")
    require(shutil.which("cc", path=os.defpath) is not None, "native helper fixture requires a system C compiler (cc)")
    with tempfile.TemporaryDirectory(prefix="bench-trait-integration-") as temporary:
        scratch = Path(temporary).resolve()
        home, temp = scratch / "home", scratch / "tmp"
        home.mkdir()
        temp.mkdir()
        keep = ("PATH", "SYSTEMROOT", "GOCACHE", "GOMODCACHE", "GOPATH", "CC", "CXX", "SDKROOT", "DEVELOPER_DIR")
        env = {key: os.environ[key] for key in keep if key in os.environ}
        env.update(HOME=str(home), TMPDIR=str(temp), ASK_LIVE="", PYTHONDONTWRITEBYTECODE="1", GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local",
                   XDG_CONFIG_HOME=str(home / ".config"), XDG_STATE_HOME=str(home / ".local/state"), XDG_CACHE_HOME=str(home / ".cache"))
        invoke([bins / "cage", "check"], cwd=scratch, env=env)
        check(bins, scratch, env)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, ValueError, OSError) as error:
        print(f"check-trait: {error}", file=sys.stderr)
        raise SystemExit(1)
