VERSION ?= 1.8.2
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/msVPNco/rovunet-panel/internal/version.Version=$(VERSION) \
           -X github.com/msVPNco/rovunet-panel/internal/version.Commit=$(COMMIT) \
           $(if $(RELEASE_TAG),-X github.com/msVPNco/rovunet-panel/internal/version.ReleaseTag=$(RELEASE_TAG))

# RELEASE_TAG is set by the build-dev target below. It must be evaluated
# at recipe-expansion time (via $(if ...)), not parse time — the previous
# `ifdef RELEASE_TAG` form ran before target-specific variables applied,
# so dev builds silently dropped the ReleaseTag ldflag and the resulting
# binary reported itself as stable.

.PHONY: build clean test install release build-dev print-version

print-version:
	@echo $(VERSION)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o rovunet .

build-linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o rovunet-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o rovunet-linux-arm64 .

build-dev: RELEASE_TAG = dev-$(COMMIT)
build-dev: build-linux
	@echo "Dev binaries built (release tag: dev-$(COMMIT))"
	@ls -la rovunet-linux-*

clean:
	rm -f rovunet rovunet-linux-*

test:
	go test ./...

install: build
	sudo cp rovunet /usr/local/bin/
	@echo "Installed to /usr/local/bin/rovunet"

release: clean build-linux
	@echo "Built release binaries"
	@ls -la rovunet-linux-*
