PYTHON ?= python3
TEND ?= tend
MAY ?= may
AGENDA ?= agenda

.PHONY: check
check:
	@command -v "$(TEND)" >/dev/null || { echo 'Select an installed Tend with TEND=/absolute/bin/tend' >&2; exit 1; }
	@command -v "$(MAY)" >/dev/null || { echo 'Select standalone May with MAY=/absolute/bin/may' >&2; exit 1; }
	@command -v "$(AGENDA)" >/dev/null || { echo 'Select Agenda with AGENDA=/absolute/bin/agenda' >&2; exit 1; }
	@"$(MAY)" check
	@PROCESS_TEST_TEND="$(TEND)" PROCESS_TEST_AGENDA="$(AGENDA)" PYTHONDONTWRITEBYTECODE=1 $(PYTHON) -m unittest discover -s tests -v
