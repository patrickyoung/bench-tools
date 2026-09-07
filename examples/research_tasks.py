"""Trusted task procedures for the bounded invoice experiment recipe."""
import json

import research_domain as domain
from protocol import digest, encoded, read


def agent_task(task):
    return task["input"]["kind"] == "review"


def review_schema():
    configs = domain.all_configs()
    properties = {}
    for key in configs[0]:
        values = list(dict.fromkeys(config[key] for config in configs))
        properties[key] = {"type": "boolean" if type(values[0]) is bool else "string", "enum": values}
    candidate = {"type": "object", "properties": properties,
                 "required": list(properties), "additionalProperties": False}
    properties = {"mode": {"type": "string", "enum": ["model"]}, "candidate": candidate,
                  "hypothesis": {"type": "string"}, "expected_tradeoff": {"type": "string"}}
    return {"type": "object", "properties": properties,
            "required": list(properties), "additionalProperties": False}


def benchmark(data):
    raw = read(data["benchmark_path"])
    if digest(raw) != data["benchmark_file_sha256"]:
        raise ValueError("frozen benchmark file changed")
    obj = json.loads(raw)
    domain.validate_benchmark(obj)
    if domain.benchmark_digest(obj) != data["benchmark_sha256"]:
        raise ValueError("benchmark identity changed")
    return obj


def execute(task, dependencies):
    data = task["input"]
    if agent_task(task):
        raise ValueError("research proposals require actual model output; no reference answer")
    obj = benchmark(data)
    config = data.get("candidate")
    if "proposal" in data:
        config = dependencies[data["proposal"]]["candidate"]
    config = domain.candidate(config)
    if data["kind"] == "evaluate":
        return {"mode": "deterministic-simulation", "candidate": config,
                "development": domain.evaluate(obj, config, "development"),
                "validation": domain.evaluate(obj, config, "feedback-validation")}
    if data["kind"] == "confirm":
        # The controller freezes all arms before it admits any confirmation.
        frozen = read(data["selection_path"])
        if digest(frozen) != data["selection_sha256"]:
            raise ValueError("frozen selections changed")
        selections = json.loads(frozen)["selections"]
        selected = next(s for s in selections if s["trial"] == data["trial"] and s["arm"] == data["arm"])
        if encoded(selected["candidate"]) != encoded(config):
            raise ValueError("confirmation differs from the frozen arm selection")
        return {"mode": "deterministic-simulation", "candidate": config,
                "selection_sha256": data["selection_sha256"],
                "holdout": domain.evaluate(obj, config, "final-holdout")}
    raise ValueError("unknown research task kind")


def check(task, result, dependencies):
    if set(dependencies) != set(task["needs"]):
        raise ValueError("research dependency set mismatch")
    if not agent_task(task):
        if encoded(result) != encoded(execute(task, dependencies)):
            raise ValueError("evaluation does not match frozen evidence and contract")
        return
    if not isinstance(result, dict) or set(result) != {"mode", "candidate", "hypothesis", "expected_tradeoff"}:
        raise ValueError("proposal must contain exactly mode, candidate, hypothesis, expected_tradeoff")
    if result["mode"] != "model":
        raise ValueError("proposal requires model provenance")
    domain.candidate(result["candidate"])
    for field in ("hypothesis", "expected_tradeoff"):
        if not isinstance(result[field], str) or not 12 <= len(result[field].strip()) <= 1200:
            raise ValueError("proposal requires bounded, substantive " + field)


def prompt(task, dependencies):
    if not agent_task(task) or dependencies:
        raise ValueError("unexpected proposal task")
    data = task["input"]
    payload = {"study": "weave-invoice-research-v1", "slot": data["slot"],
               "role": data["role"], "public_evidence": data["public_evidence"],
               "baseline": data["baseline"], "feedback": data["feedback"],
               "candidate_space": domain.all_configs()}
    return ("Propose exactly one concrete invoice-process intervention configuration for this bounded synthetic study. "
            "Return one JSON object satisfying the supplied schema; mode=model. "
            "Use a short hypothesis and expected_tradeoff (12..1200 characters each). "
            "Choose from the exact discrete candidate space. Handling times, detection, staffing and case truth are fixed evidence, not editable parameters. "
            "Use prior development and validation results to improve the next experiment. "
            "An eligible candidate must satisfy the fixed absolute feasibility gates and relative improvement rule. "
            "If none appears eligible, still propose the most informative untested candidate; do not invent success. "
            "Avoid configurations already tested by this arm; duplicates consume the same experiment allowance. "
            "Do not claim measured real business gains. The final-holdout cases and results are unavailable during search.\n"
            + json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=False))
