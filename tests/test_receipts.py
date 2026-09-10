"""Versioned Ply output evidence stays byte exact before acceptance."""
import base64
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "examples"))
import run as recipe
from protocol import digest


class VerifierReceiptTests(unittest.TestCase):
    def test_legacy_text_and_v2_binary_output(self):
        for kind, output in (("ply.verifier/v1", b"/w=="), ("ply.verifier/v2", b"\xff\x00x\n")):
            with self.subTest(kind=kind):
                wire_output = base64.b64encode(output).decode() if kind.endswith("v2") else output.decode()
                receipt = {"outcome": "accepted", "exit_code": 0, "output": wire_output,
                           "output_sha256": digest(output), "output_bytes": len(output)}
                self.assertTrue(recipe.accepted_ply_receipt(kind, receipt))
                for field, value in (("killed", True), ("interrupted", True), ("output_incomplete", True), ("start_error", True),
                                     ("elided_bytes", 1), ("exit_code", 1), ("outcome", "rejected"),
                                     ("output_bytes", len(output) + 1), ("output_sha256", digest(b"wrong")),
                                     ("output", "!")):
                    with self.subTest(field=field):
                        self.assertFalse(recipe.accepted_ply_receipt(kind, {**receipt, field: value}))
                self.assertFalse(recipe.accepted_ply_receipt("ply.verifier/v3", receipt))


if __name__ == "__main__":
    unittest.main()
