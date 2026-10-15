GO_VERSION ?= 1.26.3
COVERAGE   := coverage.out
COVER_THRESHOLD := 75

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

# cover: порог суммарного покрытия pkg/cdek (CI-гейт использует тот же порог)
.PHONY: cover
cover:
	go test -count=1 -coverprofile=$(COVERAGE) ./pkg/...
	@go tool cover -func=$(COVERAGE) | grep -E 'pkg/cdek/(service|dto_mapper)\.go:' || true
	@total=$$(awk '$$1 ~ /\/pkg\/cdek\// { stmts += $$2; if ($$3 > 0) hit += $$2 } END { printf "%.1f", 100 * hit / stmts }' $(COVERAGE)); \
	echo "pkg/cdek total: $${total}% (threshold $(COVER_THRESHOLD)%)"; \
	awk -v threshold=$(COVER_THRESHOLD) -v pct="$${total}" 'BEGIN { if (pct < threshold) { printf "coverage %.1f%% is below threshold %d%%\n", pct, threshold; exit 1 } }'

.PHONY: build
build: fmt
	go build ./...

.PHONY: codegen
codegen:
	oapi-codegen -config oapi-codegen-config.yaml api/cdek-api.yaml

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
