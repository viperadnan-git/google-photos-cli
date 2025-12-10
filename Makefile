.PHONY: build clean install uninstall

PREFIX ?= /usr/local
GO ?= $(shell which go)

build:
	cd cmd/gpcli && $(GO) build -o ../../gpcli .

clean:
	rm -f gpcli

install: build
	install -d $(PREFIX)/bin
	install -m 755 gpcli $(PREFIX)/bin/gpcli

uninstall:
	rm -f $(PREFIX)/bin/gpcli
