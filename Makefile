GO_VERSION ?= 1.26.3
COVERAGE   := coverage.out

.DEFAULT_GOAL := build

.PHONY: fmt
fmt:
	go fix ./...
	go fmt ./...

.PHONY: lint
lint: fmt
	go vet ./...
	golangci-lint run ./...

.PHONY: test
test:
	go test -v -race -coverprofile=$(COVERAGE) ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration -v -timeout=60s ./...

.PHONY: coverage
coverage: test
	go tool cover -func=$(COVERAGE) | tail -1
	go tool cover -html=$(COVERAGE) -o coverage.html

.PHONY: build
build: fmt
	go build ./...

.PHONY: codegen
codegen:
	oapi-codegen -config oapi-codegen-config.yaml cdek-api.yaml

.PHONY: security
security:
	govulncheck ./...

.PHONY: tools-install
tools-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: hooks-install
hooks-install:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit

.PHONY: clean
clean:
	rm -f $(COVERAGE) coverage.html
