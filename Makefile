APP_NAME = eigenda-proxy
LINTER_VERSION = v1.52.1
LINTER_URL = https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh
GET_LINT_CMD = "curl -sSfL $(LINTER_URL) | sh -s -- -b $(go env GOPATH)/bin $(LINTER_VERSION)"

GITCOMMIT ?= $(shell git rev-parse HEAD)
GITDATE ?= $(shell git show -s --format='%ct')
VERSION := v0.0.0

LDFLAGSSTRING +=-X main.GitCommit=$(GITCOMMIT)
LDFLAGSSTRING +=-X main.GitDate=$(GITDATE)
LDFLAGSSTRING +=-X main.Version=$(VERSION)
LDFLAGS := -ldflags "$(LDFLAGSSTRING)"

E2ETEST = INTEGRATION=true go test -timeout 1m -v ./e2e -parallel 4 -deploy-config ../.devnet/devnetL1.json
HOLESKYTEST = TESTNET=true go test -timeout 50m -v ./e2e  -parallel 4 -deploy-config ../.devnet/devnetL1.json

help: ## Prints this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: eigenda-proxy
eigenda-proxy: ## Builds the eigenda-proxy binary
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build -v $(LDFLAGS) -o ./bin/eigenda-proxy ./cmd/server

.PHONY: docker-build
docker-build: ## Builds eigenda-proxy Docker image
	@docker build -t $(APP_NAME) .

run-minio: ## Runs Minio server in Docker
	docker run -p 4566:9000 -d -e "MINIO_ROOT_USER=minioadmin" -e "MINIO_ROOT_PASSWORD=minioadmin" --name minio minio/minio server /data

stop-minio: ## Stops and removes Minio Docker container
	docker stop minio && docker rm minio

run-server: ## Runs the eigenda-proxy server
	./bin/eigenda-proxy

clean: ## Cleans up built eigenda-proxy binary under folder bin/
	rm bin/eigenda-proxy

test: ## Runs tests
	go test -v ./... -parallel 4 

e2e-test: run-minio ## Runs end-to-end tests
	$(E2ETEST) && \
	make stop-minio

holesky-test: run-minio ## Runs holesky tests
	$(HOLESKYTEST) && \
	make stop-minio

.PHONY: lint
lint: ## Lints the code
	@if ! test -f  &> /dev/null; \
	then \
    	echo "golangci-lint command could not be found...."; \
		echo "\nTo install, please run $(GET_LINT_CMD)"; \
		echo "\nBuild instructions can be found at: https://golangci-lint.run/usage/install/."; \
    	exit 1; \
	fi

	@golangci-lint run

gosec: ## Runs gosec security scanner
	@echo "Running security scan with gosec..."
	gosec ./...

submodules: ## Updates git submodules
	git submodule update --init --recursive

op-devnet-allocs: ## Generates devnet allocations
	@echo "Generating devnet allocs..."
	@./scripts/op-devnet-allocs.sh

.PHONY: \
	clean \
	test
