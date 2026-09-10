PREFIX ?= /usr/local
DESTDIR ?=
GO ?= go
PYTHON ?= python3

.PHONY: all check bootstrap check-examples install release clean
all: weave

weave: main.go go.mod
	$(GO) build -trimpath -buildvcs=false -o $@ .

check:
	@test -z "$$(gofmt -l *.go)" || { echo 'Go sources need formatting' >&2; exit 1; }
	$(GO) test ./...
	$(GO) test -race ./...
	$(GO) vet ./...
	$(PYTHON) -m unittest discover -s tests -p test_business.py
	$(PYTHON) -m unittest discover -s tests -p test_protocol.py
	$(PYTHON) -m unittest discover -s tests -p test_receipts.py
	$(PYTHON) -m unittest discover -s tests -p test_research_domain.py
	$(PYTHON) -m unittest discover -s tests -p test_release.py

bootstrap:
	$(PYTHON) scripts/bootstrap.py

check-examples:
	./tests/check

install: weave
	install -d "$(DESTDIR)$(PREFIX)/bin" "$(DESTDIR)$(PREFIX)/share/man/man1"
	install -m 755 weave "$(DESTDIR)$(PREFIX)/bin/weave"
	install -m 644 weave.1 "$(DESTDIR)$(PREFIX)/share/man/man1/weave.1"

release:
	$(PYTHON) scripts/release.py

clean:
	rm -f weave
