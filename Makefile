.DEFAULT_GOAL := help

# This file coordinates development only. Each program remains independent.
PYTHON ?= python3
TOOLS ?=
PREFIX ?= $(HOME)/.local

.PHONY: help build test check check-docs check-examples check-harnesses check-worker-portability install uninstall list

help:
	@echo 'make build                 Build all public commands into .build/bin'
	@echo 'make test                  Run everyday checks (no race or integration)'
	@echo 'make check                 Run the full verification gate'
	@echo 'make check-docs            Check guide links and heading anchors'
	@echo 'make check-examples        Build starter tools and run offline examples'
	@echo 'make check-harnesses       Verify portable skills and MCP host compatibility'
	@echo 'make check-worker-portability Verify worker skill exports and original checks'
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

check: check-docs check-worker-portability
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

install:
	@$(PYTHON) scripts/install $(TOOLS) --prefix "$(PREFIX)"

uninstall:
	@$(PYTHON) scripts/uninstall $(TOOLS) --prefix "$(PREFIX)"

list:
	@$(PYTHON) scripts/build --list
