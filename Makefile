GO ?= go
PKG := ./...

.PHONY: fmt tidy vet lint test check install-tools release-patch release-minor hooks

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

vet:
	$(GO) vet $(PKG)

lint:
	golangci-lint run
	staticcheck ./...

test:
	$(GO) test $(PKG)

check: fmt tidy vet lint test

install-tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$$(go env GOPATH)/bin"

hooks:
	git config core.hooksPath .githooks

release-patch:
	./scripts/release.sh patch

release-minor:
	./scripts/release.sh minor
