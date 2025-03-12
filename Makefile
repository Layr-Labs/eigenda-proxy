GIT_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d--%H:%M:%S')
GIT_TAG := $(shell git describe --tags --always --dirty)

LDFLAGSSTRING +=-X main.Commit=$(GIT_COMMIT)
LDFLAGSSTRING +=-X main.Date=$(BUILD_TIME)
LDFLAGSSTRING +=-X main.Version=$(GIT_TAG)
LDFLAGS := -ldflags "$(LDFLAGSSTRING)"

.PHONY: eigenda-proxy
eigenda-proxy:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build -v $(LDFLAGS) -o ./bin/eigenda-proxy ./cmd/server

.PHONY: docker-build
docker-build:
	# we only use this to build the docker image locally, so we give it the dev tag as a reminder
	@docker build -t ghcr.io/layr-labs/eigenda-proxy:dev .

run-memstore-server:
	./bin/eigenda-proxy --memstore.enabled --metrics.enabled

disperse-test-blob:
	curl -X POST -d my-blob-content http://127.0.0.1:3100/put/

clean:
	rm bin/eigenda-proxy

# Runs all tests, excluding e2e
test-unit:
	go test `go list ./... | grep -v ./e2e` -parallel 4

# E2E tests, leveraging op-e2e framework. Also tests the standard client against the proxy.
# If holesky tests are failing, consider checking https://dora.holesky.ethpandaops.io/epochs for block production status.
test-e2e:
	# Add the -v flag to observe logs as the run is happening on CI, given that this test takes ~5 minutes to run.
	# Good to have early feedback when needed.
	go test -v -timeout 10m ./e2e -parallel 4

# E2E test which fuzzes the proxy client server integration and op client keccak256 with malformed inputs
test-fuzz:
	go test ./fuzz -fuzz -v -fuzztime=5m

.PHONY: lint
lint:
	@if ! command -v golangci-lint  &> /dev/null; \
	then \
    	echo "golangci-lint command could not be found...."; \
		echo "You can install via 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest'"; \
		echo "or visit https://golangci-lint.run/welcome/install/ for other installation methods."; \
    	exit 1; \
	fi
	@golangci-lint run

.PHONY: format
format:
	# We also format line lengths. The length here should match that in the lll linter in .golangci.yml
	@if ! command -v golines  &> /dev/null; \
	then \
    	echo "golines command could not be found...."; \
		echo "You can install via 'go install github.com/segmentio/golines@latest'"; \
		echo "or visit https://github.com/segmentio/golines for other installation methods."; \
    	exit 1; \
	fi
	@go fmt ./...
	@golines --write-output --shorten-comments --max-len 120 .

go-gen-mocks:
	@echo "generating go mocks..."
	@GO111MODULE=on go generate --run "mockgen*" ./...

op-devnet-allocs:
	@echo "Generating devnet allocs..."
	@./scripts/op-devnet-allocs.sh

benchmark:
	go test -benchmem -run=^$ -bench . ./e2e -test.parallel 4

.PHONY: \
	clean \
	test
