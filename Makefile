.PHONY: build test vet lint run clean install tidy release release-snapshot dist-build mcpb mcpb-clean

BINARY = ssh-mcp
CMD_DIR = ./cmd/ssh-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST_DIR = dist
MCPB_DIR = mcpb

# Cross-platform build targets. Each entry is GOOS/GOARCH and produces a
# binary in $(DIST_DIR)/<goos>-<goarch>/$(BINARY).
BUILD_TARGETS := windows-amd64 darwin-amd64 darwin-arm64 linux-amd64 linux-arm64

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) $(CMD_DIR)

test:
	go test ./... -v

test-short:
	go test ./... -v -short

vet:
	go vet ./...

lint:
	golangci-lint run ./...

run: build
	./$(BINARY)

install: build
	cp $(BINARY) $(GOPATH)/bin/

clean:
	rm -f $(BINARY)
	go clean ./...

tidy:
	go mod tidy

# release builds and publishes via goreleaser (requires goreleaser installed
# and a git remote). Use release-snapshot for a local, non-published build.
release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --snapshot --clean

# dist-build produces cross-platform binaries without goreleaser, placing
# each binary in $(DIST_DIR)/<goos>-<goarch>/. Used by the mcpb target and
# usable directly when goreleaser is not installed.
dist-build: $(BUILD_TARGETS)

$(BUILD_TARGETS):
	@mkdir -p $(DIST_DIR)/$@
	$(eval GOOS := $(word 1,$(subst -, ,$@)))
	$(eval GOARCH := $(word 2,$(subst -, ,$@)))
	@echo "building $(GOOS)/$(GOARCH)"
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 \
		go build -ldflags "-X main.version=$(VERSION)" \
		-o $(DIST_DIR)/$@/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,) $(CMD_DIR)

# mcpb builds per-platform MCPB bundles (zip of manifest.json + the binary
# under server/) into $(DIST_DIR). Requires zip on PATH.
mcpb: dist-build mcpb-clean
	@for target in $(BUILD_TARGETS); do \
		goos=$$(echo $$target | cut -d- -f1); \
		staging="$(DIST_DIR)/mcpb-$$target"; \
		mkdir -p "$$staging/server"; \
		cp $(MCPB_DIR)/manifest.json "$$staging/manifest.json"; \
		bin=$(BINARY); \
		if [ "$$goos" = "windows" ]; then bin=$(BINARY).exe; fi; \
		cp $(DIST_DIR)/$$target/$$bin "$$staging/server/$$bin"; \
		( cd "$$staging" && zip -r "../ssh-mcp-$$target.mcpb" manifest.json server/ >/dev/null ); \
		echo "built $(DIST_DIR)/ssh-mcp-$$target.mcpb"; \
	done

mcpb-clean:
	rm -rf $(DIST_DIR)/mcpb-*

