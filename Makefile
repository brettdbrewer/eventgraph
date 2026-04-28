.PHONY: verify verify-go verify-ts verify-python verify-rust build-go test-go vet-go build-ts test-ts test-python test-rust

CARGO ?= $(HOME)/.cargo/bin/cargo

verify: verify-go verify-ts verify-python verify-rust

verify-go: build-go test-go vet-go

build-go:
	cd go && go build ./...

test-go:
	cd go && go test ./...

vet-go:
	cd go && go vet ./...

verify-ts: build-ts test-ts

build-ts:
	cd ts && npm run build

test-ts:
	cd ts && npm run test

verify-python: test-python

test-python:
	cd python && python3 -m pytest

verify-rust: test-rust

test-rust:
	cd rust && $(CARGO) test
