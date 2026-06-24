BIN := skillforge
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
LDFLAGS := -s -w -X github.com/mrdulasolutions/skillforge/cmd.version=$(VERSION)

.PHONY: build test fmt vet install snapshot clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

install: build
	install -m 0755 $(BIN) $(HOME)/.local/bin/$(BIN)

# Cross-platform release build (requires goreleaser).
snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist $(BIN)
