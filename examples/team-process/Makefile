PYTHON ?= python3
TEND ?= tend

.PHONY: check
check:
	@command -v "$(TEND)" >/dev/null || { echo 'Select an installed Tend with TEND=/absolute/bin/tend' >&2; exit 1; }
	@PROCESS_TEST_TEND="$(TEND)" PYTHONDONTWRITEBYTECODE=1 $(PYTHON) -m unittest discover -s tests -v
