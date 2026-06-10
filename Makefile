VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null)
ifeq "$(VERSION)" ""
	VERSION := UNKNOWN
endif

LDFLAGS=\
	-X github.com/Felixoid/pomo/pkg/internal.Version=$(VERSION)

.PHONY: \
	test \
	docs \
	pomo-build \
	readme \
	lint \
	hooks-install \
	bin/pomo

default: bin/pomo test

build: bin/pomo

clean:
	[[ -f bin/pomo ]] && rm bin/pomo || true

bin/pomo: hooks-install
	cd cmd/pomo && \
	go build -ldflags '${LDFLAGS}' -o ../../$@

test: hooks-install
	go test ./...

lint:
	@command -v golangci-lint >/dev/null || { \
		echo "golangci-lint not found — install: https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	}
	golangci-lint run

hooks-install:
	@[ "$$(git config --local --get core.hooksPath 2>/dev/null)" = ".githooks" ] || { \
		git config --local core.hooksPath .githooks && \
		echo "git hooks installed (.githooks)"; \
	}

install:
	go install ./cmd/...

man/pomo.1: man/pomo.1.scd
	scdoc < $< > $@

manpages: man/pomo.1

docs: www/data/readme.json
	cd www && hugo -d ../docs

www/data/readme.json: www/data README.md
	cat README.md | python -c 'import json,sys; print(json.dumps({"content": sys.stdin.read()}))' > $@

www/data bin:
	mkdir -p $@
