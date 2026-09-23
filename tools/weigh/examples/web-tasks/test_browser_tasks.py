import unittest

import browser_tasks as app


def choice(value, probabilities, confidence=.96):
    return {"answers": {"matches": {"value": .02}, "next": {
        "value": value, "probabilities": probabilities}}, "metadata": {"confidence": {"next": confidence}}}


def page(address="https://example.test/", links=None, records=None):
    return {"version": 1, "requested_url": address, "url": address, "title": "Example",
            "text": "Observed source", "links": links or [], "records": records or []}


class Tests(unittest.TestCase):
    def test_observed_links_only_same_origin_and_no_cycles(self):
        p = page(links=[{"label": "Again", "url": "https://example.test/#again"},
                        {"label": "Guide", "url": "https://EXAMPLE.test:443/guide"},
                        {"label": "Duplicate", "url": "https://example.test/guide#top"},
                        {"label": "Outside", "url": "https://outside.test/"},
                        {"label": "Script", "url": "javascript:alert(1)"},
                        {"label": "Private", "url": "https://key@example.test/"}])
        request, candidates = app.navigation_request(p, "Find a guide", {p["url"]})
        self.assertEqual(candidates, {"link_1": {"url": "https://example.test/guide", "label": "Guide"}})
        self.assertEqual(set(request["questions"]["next"]["options"]), {"link_1", "stop"})
        self.assertNotIn("Outside", str(request))

    def test_no_candidates_asks_only_about_current_page(self):
        request, candidates = app.navigation_request(page(), "Find guide", set())
        self.assertEqual(candidates, {})
        self.assertEqual(set(request["questions"]), {"matches"})

    def test_link_overflow_is_not_silent_truncation(self):
        p = page(links=[{"label": "Guide", "url": f"https://example.test/{i}"} for i in range(255)])
        with self.assertRaises(app.Failure):
            app.navigation_request(p, "Find guide", set())

    def test_records_remain_independent_with_unknown_support(self):
        records = [{"text": "Quiet room. Refunds not stated.", "links": []},
                   {"text": "Refundable room next to a loud club.", "links": []}]
        request, observed = app.shortlist_request(page(records=records), {"quiet": "Quiet workspace", "refund": "Refundable"})
        self.assertEqual(len(request["questions"]), 4)
        self.assertEqual(observed["record_1"], records[0])
        for q in request["questions"].values():
            self.assertEqual(set(q["options"]), {"yes", "no", "unknown"})
            self.assertIn("neighboring records", q["question"])

    def test_ambiguous_json_refused(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{} {}'):
            with self.assertRaises((app.Failure, ValueError)):
                app.parse(raw)

    def test_private_or_invalid_urls_refused(self):
        for address in ("https://key@example.test/", "file:///etc/passwd", "https://e.test:bad/", "https://e.test/\n"):
            with self.assertRaises((app.Failure, ValueError)):
                app.url(address)

    def test_navigation_uncertainty_and_page_budget(self):
        from types import SimpleNamespace
        args = SimpleNamespace(url="https://example.test/", goal="Find guide", max_pages=1,
                               accept_at=.9, min_confidence=.8, expect_text=None)

        class Commands:
            def observe(self, address):
                return page(address, links=[{"label": "Guide", "url": address + "guide"}])

            def judge(self, request):
                return choice("link_1", {"link_1": .95, "stop": .05}, .1)

        commands = Commands()
        self.assertEqual(app.navigate(args, commands)["reason"], "uncertain_navigation")
        commands.judge = lambda _: choice("link_1", {"link_1": .95, "stop": .05})
        self.assertEqual(app.navigate(args, commands)["reason"], "page_budget")
        commands.judge = lambda _: choice("link_1", {"link_1": .95, "stop": .05}, None)
        self.assertEqual(app.navigate(args, commands)["reason"], "uncertain_navigation")

    def test_model_found_does_not_override_exact_check(self):
        from types import SimpleNamespace
        args = SimpleNamespace(url="https://example.test/", goal="Find guide", max_pages=1,
                               accept_at=.9, min_confidence=.8, expect_text="unobserved claim")

        class Commands:
            def observe(self, address):
                return page(address)

            def judge(self, request):
                return {"answers": {"matches": {"value": .99}}}

        self.assertEqual(app.navigate(args, Commands())["status"], "review")


if __name__ == "__main__":
    unittest.main()
