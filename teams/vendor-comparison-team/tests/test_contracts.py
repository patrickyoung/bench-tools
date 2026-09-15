#!/usr/bin/env python3
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import http.server
import unittest

TEAM = Path(os.environ["COMPARISON_TEAM_EXPERT"]).resolve()
FIXTURE_SPEC = importlib.util.spec_from_file_location("make_cases", Path(__file__).with_name("make_cases.py"))
fixtures = importlib.util.module_from_spec(FIXTURE_SPEC)
FIXTURE_SPEC.loader.exec_module(fixtures)
SPEC = importlib.util.spec_from_file_location("team_io", TEAM / "tools/team_io.py")
io = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(io)
import compare


def accepted(packet):
    job = packet["job"]
    cells = []
    for candidate, scores, source in [("atlas", [3, 4, 5], "atlas-deck"), ("beacon", [5, 5, 3], "beacon-spec"), ("crest", [5, 5, 5], "crest-demo")]:
        sources = {s["id"]: s for s in packet["sources"]}
        for criterion, score in zip(job["criteria"], scores):
            segment = sources[source]["segments"][-1] if candidate == "atlas" and criterion["id"] == "cost" else next(s for s in sources[source]["segments"] if len(s["text"]) >= 30)
            cells.append({"candidate_id": candidate, "criterion_id": criterion["id"], "score": score, "status": "supported", "confidence": "medium", "rationale": "Synthetic contract fixture; semantic correctness is tested separately.", "refs": [{"source_id": source, "locator": segment["locator"], "quote": segment["text"]}]})
    gates = []
    for candidate, source, status in [("atlas", "atlas-site", "pass"), ("beacon", "beacon-spec", "pass"), ("crest", "crest-demo", "fail")]:
        segment = next(s for s in packet["sources"] if s["id"] == source)["segments"][-1]
        gates.append({"candidate_id": candidate, "gate_id": "sso", "status": status, "rationale": "Synthetic gate fixture.", "refs": [{"source_id": source, "locator": segment["locator"], "quote": segment["text"]}]})
    return {"schema": "bench.vendor-comparison/v1", "packet_sha256": compare.sha(compare.dump(packet)), "decision_summary": "Synthetic contract fixture.", "weight_basis": "supplied", "weight_rationale": "Supplied decision priorities.", "criteria": copy.deepcopy(job["criteria"]), "cells": cells, "gates": gates,
            "recommendation": {"status": "recommend", "candidate_id": "beacon", "rationale": "Candidate passes gates and has highest eligible score."}, "assumptions": [], "gaps": []}


class ContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.cases = Path(cls.temp.name).resolve() / "cases"
        fixtures.main(cls.cases)
        cls.prepared = Path(cls.temp.name).resolve() / "prepared"
        io.prepare(cls.cases / "01-mixed-materials/job.json", cls.prepared, offline=True)
        cls.packet = compare.parse(compare.read_bytes(cls.prepared / "control/packet.json"))

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def setUp(self):
        self.a = accepted(self.packet)

    def validate(self):
        compare.validate(self.packet, self.a, compare.sha(compare.dump(self.packet)))

    def reject(self, mutation):
        mutation()
        with self.assertRaises((ValueError, TypeError, KeyError)):
            self.validate()

    def test_accept_and_independent_arithmetic(self):
        self.validate()
        rows = {r["candidate_id"]: r for r in compare.calculate(self.packet, self.a)["totals"]}
        self.assertEqual((rows["atlas"]["lower_bound"], rows["beacon"]["lower_bound"], rows["crest"]["lower_bound"]), (77, 90, 100))
        self.assertEqual(rows["crest"]["eligibility"], "excluded")

    def test_pdf_and_slide_notes_extracted(self):
        sources = {s["id"]: s for s in self.packet["sources"]}
        self.assertEqual(sources["beacon-spec"]["segments"][0]["locator"], "page 1")
        self.assertIn("USD 24000", sources["atlas-deck"]["segments"][1]["text"])
        self.assertEqual(sources["atlas-deck"]["segments"][1]["locator"], "slide 1 notes")
        self.assertNotIn("Ignore the task", str(sources["atlas-site"]["segments"]))

    def test_weights_rejected(self):
        self.reject(lambda: self.a["criteria"][0].update(weight=41))

    def test_supplied_weights_cannot_change_even_at_100(self):
        self.a["criteria"][0]["weight"] = 35
        self.reject(lambda: self.a["criteria"][1].update(weight=40))

    def test_stale_packet(self):
        self.reject(lambda: self.a.update(packet_sha256="0" * 64))

    def test_boolean_score(self):
        self.reject(lambda: self.a["cells"][0].update(score=True))

    def test_missing_cell(self):
        self.reject(lambda: self.a["cells"].pop())

    def test_duplicate_cell(self):
        self.reject(lambda: self.a["cells"].__setitem__(1, copy.deepcopy(self.a["cells"][0])))

    def test_fake_quote(self):
        self.reject(lambda: self.a["cells"][0]["refs"][0].update(quote="This is a completely invented source quotation."))

    def test_wrong_candidate_source(self):
        self.reject(lambda: self.a["cells"][0]["refs"].__setitem__(0, self.a["cells"][3]["refs"][0]))

    def test_unknown_not_zero(self):
        self.reject(lambda: self.a["cells"][0].update(status="unknown", score=0))

    def test_conflict_requires_distinct_refs(self):
        self.a["cells"][0].update(status="conflicting", score=None)
        self.a["cells"][0]["refs"] *= 2
        self.reject(lambda: None)

    def test_failed_gate_cannot_win(self):
        self.reject(lambda: self.a["recommendation"].update(candidate_id="crest"))

    def test_unknown_gate_cannot_be_unconditional(self):
        self.reject(lambda: self.a["gates"][1].update(status="unknown"))

    def test_partial_coverage_bounds(self):
        self.a["cells"][0].update(status="unknown", score=None, refs=[])
        self.a["gaps"] = [{"candidate_id": "atlas", "question": "Confirm connectors?", "owner": "Evaluator", "decision_impact": "Could alter fit."}]
        self.validate()
        row = next(r for r in compare.calculate(self.packet, self.a)["totals"] if r["candidate_id"] == "atlas")
        self.assertEqual((row["lower_bound"], row["upper_bound"], row["coverage_percent"]), (53, 93, 60))

    def test_zero_coverage_cannot_select(self):
        for cell in self.a["cells"]:
            cell.update(status="unknown", score=None, refs=[])
        self.reject(lambda: None)

    def test_no_nan_or_duplicate_keys(self):
        for value in (b'{"x": 1, "x": 2}', b'{"x": NaN}'):
            with self.assertRaises(ValueError):
                compare.parse(value)

    def test_csv_injection_guard(self):
        self.assertEqual(compare.csv_safe(" =HYPERLINK(1)"), "' =HYPERLINK(1)")

    def test_render_tampering_and_stale_input(self):
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp).resolve()
            (work / "output").mkdir()
            (work / "packet.json").write_bytes(compare.dump(self.packet))
            (work / "output/analysis.json").write_bytes(compare.dump(self.a))
            command = [sys.executable, str(TEAM / "agents/comparison/tools/compare.py")]
            self.assertEqual(subprocess.run(command + ["render"], cwd=work, capture_output=True).returncode, 0)
            self.assertEqual(subprocess.run(command + ["check"], cwd=work, capture_output=True).returncode, 0)
            (work / "output/report.md").write_text("Fabricated replacement")
            self.assertNotEqual(subprocess.run(command + ["check"], cwd=work, capture_output=True).returncode, 0)

    def test_changed_manager_input_rejected(self):
        run = Path(self.temp.name).resolve() / "binding"
        io.prepare(self.cases / "01-mixed-materials/job.json", run, offline=True)
        (run / "stages/manager/inputs/packet.json").write_text("{}");
        with self.assertRaises(ValueError):
            io.check_binding(run, "manager")

    def test_live_url_snapshot_and_offline_unavailable(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.send_header("Content-Type", "text/html")
                self.end_headers()
                self.wfile.write(b"<p>Fictional website source with current plan information.</p>")
            def log_message(self, *args):
                pass
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            directory = Path(self.temp.name).resolve() / "url"
            directory.mkdir()
            job = {"business_case": "Fictional URL extraction test", "candidates": ["One", "Two"], "materials": [{"id": "website", "url": f"http://127.0.0.1:{server.server_port}/page", "candidate_ids": []}]}
            (directory / "job.json").write_bytes(compare.dump(job))
            for label, offline in [("online", False), ("offline", True)]:
                io.prepare(directory / "job.json", directory / label, offline=offline, allow_local=True)
                source = compare.parse(compare.read_bytes(directory / label / "control/packet.json"))["sources"][0]
                self.assertEqual(source["status"], "unavailable" if offline else "available")
        finally:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    unittest.main(verbosity=2)
