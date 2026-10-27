# Build:                             make
# System-wide or packaged install:   make install [PREFIX=/usr/local] [DESTDIR=]
# Per-user, without make:            ./install.sh
PREFIX ?= /usr/local
BINDIR  = $(DESTDIR)$(PREFIX)/bin
SHARE   = $(DESTDIR)$(PREFIX)/share

.PHONY: build install uninstall check test lint dist

build:
	go build -trimpath -o bin/nauka ./cmd/nauka

install: build
	install -Dm755 bin/nauka $(BINDIR)/nauka
	install -Dm644 README.md $(SHARE)/doc/nauka/README.md
	install -Dm644 CHANGELOG.md $(SHARE)/doc/nauka/CHANGELOG.md
	install -Dm644 LICENSE $(SHARE)/licenses/nauka/LICENSE

uninstall:
	rm -f $(BINDIR)/nauka
	rm -rf $(SHARE)/doc/nauka $(SHARE)/licenses/nauka

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	shellcheck -S warning nauka.tmux install.sh scripts/nauka-run

# What CI runs.
check: lint test build
	./bin/nauka --version
	./bin/nauka --help >/dev/null

# Release binaries (CI does this on a tag).
dist:
	mkdir -p dist
	for arch in amd64 arm64; do \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags='-s -w' -o dist/nauka-linux-$$arch ./cmd/nauka; \
	done
