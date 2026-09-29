PYTHON ?= python3
TEND ?= tend
MAY ?= may

.PHONY: check
check:
	@command -v "$(TEND)" >/dev/null || { echo 'Select an installed Tend with TEND=/absolute/bin/tend' >&2; exit 1; }
	@command -v "$(MAY)" >/dev/null || { echo 'Select standalone May with MAY=/absolute/bin/may' >&2; exit 1; }
	@"$(MAY)" check
	@PROCESS_TEST_TEND="$(TEND)" PYTHONDONTWRITEBYTECODE=1 $(PYTHON) -m unittest discover -s tests -v
