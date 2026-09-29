PREFIX ?= /usr/local
DESTDIR ?=
GO ?= go

.PHONY: all check install clean
all: agenda

agenda: main.go go.mod
	$(GO) build -trimpath -buildvcs=false -o $@ .

check:
	@test -z "$$(gofmt -l *.go)" || { echo 'Go sources need formatting' >&2; exit 1; }
	$(GO) test ./...
	$(GO) test -race ./...
	$(GO) vet ./...

install: agenda
	install -d "$(DESTDIR)$(PREFIX)/bin" "$(DESTDIR)$(PREFIX)/share/man/man1"
	install -m 755 agenda "$(DESTDIR)$(PREFIX)/bin/agenda"
	install -m 644 agenda.1 "$(DESTDIR)$(PREFIX)/share/man/man1/agenda.1"

clean:
	rm -f agenda
