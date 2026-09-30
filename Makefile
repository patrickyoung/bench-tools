.DEFAULT_GOAL := help

# This file coordinates development only. Each program remains independent.
PYTHON ?= python3
TOOLS ?=
PREFIX ?= $(HOME)/.local
EVAL_PYTHON ?= $(PYTHON)
EVAL_ARGS ?=

.PHONY: help build test check check-docs check-examples check-harnesses check-worker-portability check-workers check-team-process check-agenda build-interfaces check-interfaces install uninstall list

help:
	@echo 'make build                 Build all public commands into .build/bin'
	@echo 'make test                  Run everyday checks (no race or integration)'
	@echo 'make check                 Run the full verification gate'
	@echo 'make check-docs            Check guide links and heading anchors'
	@echo 'make check-examples        Build starter tools and run offline examples'
	@echo 'make check-harnesses       Verify portable skills and MCP host compatibility'
	@echo 'make check-worker-portability Verify worker skill exports and original checks'
	@echo 'make check-workers         Run worker/team offline evaluations (see docs/WORKER-EVALUATIONS.md)'
	@echo 'make check-team-process    Check standing-team commitments, calendars and real Tend boundaries'
	@echo 'make check-agenda          Check Agenda and its interface through public commands'
	@echo 'make build-interfaces      Build the independent browser interfaces'
	@echo 'make check-interfaces      Check browser interfaces offline (tests, race, vet)'
	@echo 'make install               Build and install under ~/.local'
	@echo 'make uninstall             Remove verified installs made here'
	@echo 'make list                  List components and public commands'
	@echo ''
	@echo 'Select components: make test TOOLS="ask ply"'
	@echo 'Choose a prefix:   make install PREFIX="/path/to/prefix"'
	@echo 'Native Cage proof: ./scripts/check --native-cage'
	@echo 'Make is optional; scripts/build, scripts/check and scripts/install work directly.'

build:
	@$(PYTHON) scripts/build $(TOOLS)

test:
	@$(PYTHON) scripts/check --quick $(TOOLS)

check: check-docs check-worker-portability check-interfaces check-team-process check-agenda
	@$(PYTHON) -m unittest discover -s scripts/tests -v
	@$(PYTHON) scripts/check $(TOOLS)

check-docs:
	@$(PYTHON) scripts/check-docs.py

check-examples:
	@$(PYTHON) scripts/build ask brief context cite tend agent hire ply cage
	@$(PYTHON) scripts/check-examples.py --bin-dir .build/bin

check-harnesses:
	@$(PYTHON) scripts/build brief mcp
	@$(PYTHON) scripts/check-harnesses.py --bin-dir .build/bin

check-worker-portability:
	@$(PYTHON) scripts/build brief
	@$(PYTHON) scripts/check-worker-portability.py --bin-dir .build/bin

check-workers:
	@$(PYTHON) scripts/check-worker-evaluations.py --python "$(EVAL_PYTHON)" $(EVAL_ARGS)

check-team-process:
	@$(PYTHON) scripts/build tend may agenda
	@$(MAKE) -C examples/team-process check PYTHON="$(PYTHON)" TEND="$(CURDIR)/.build/bin/tend" MAY="$(CURDIR)/.build/bin/may" AGENDA="$(CURDIR)/.build/bin/agenda"

build-interfaces:
	@$(MAKE) -C interfaces/hire build
	@$(MAKE) -C interfaces/agenda build

check-interfaces:
	@$(MAKE) -C interfaces/hire check
	@$(MAKE) -C interfaces/agenda check

check-agenda:
	@$(PYTHON) scripts/build agenda mcp
	@$(PYTHON) scripts/check agenda
	@AGENDA_TEST_BIN="$(CURDIR)/.build/bin/agenda" MCP_TEST_BIN="$(CURDIR)/.build/bin/mcp" MCPSERVE_TEST_BIN="$(CURDIR)/.build/bin/mcpserve" $(MAKE) -C interfaces/agenda check

install:
	@$(PYTHON) scripts/install $(TOOLS) --prefix "$(PREFIX)"

uninstall:
	@$(PYTHON) scripts/uninstall $(TOOLS) --prefix "$(PREFIX)"

list:
	@$(PYTHON) scripts/build --list
