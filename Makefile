GO ?= go
VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY ?= $(shell test -z "$$(git status --porcelain 2>/dev/null)" && echo false || echo true)
LDFLAGS = -s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$(VERSION) -X github.com/jsabak/gauge-allure-report/internal/version.Commit=$(COMMIT) -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=$(BUILD_DATE) -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=$(DIRTY) -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$(VERSION)

.PHONY: bootstrap fmt lint test cover-core licenses test-race test-e2e-python test-e2e-js test-allure2 test-allure3 build install-local package package-all verify-package docs clean

bootstrap:
	$(GO) mod download

fmt:
	$(GO) fmt ./...

lint:
	$(GO) vet ./...

test:
	$(GO) test -coverprofile=coverage.out ./...

cover-core:
	sh scripts/check-core-coverage.sh

licenses:
	sh scripts/check-licenses.sh

test-race:
	$(GO) test -race ./...

test-e2e-python:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-e2e.ps1 -Runner python

test-e2e-js:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-e2e.ps1 -Runner js

test-allure2:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-allure.ps1 -Major 2

test-allure3:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-allure.ps1 -Major 3

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/allure-report ./cmd/allure-report

install-local: build
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-local.ps1

package: build
	$(GO) run ./build/package -binary bin/allure-report -os $$(go env GOOS) -arch $$(go env GOARCH) -version $(VERSION) -out dist

verify-package:
	@for archive in dist/*.zip; do $(GO) run ./build/verify -archive "$$archive" -version $(VERSION); done

docs:
	$(GO) run ./cmd/allure-report validate-config examples/allure-report.yaml

package-all:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-all.ps1 -Version $(VERSION) -Commit $(COMMIT) -BuildDate $(BUILD_DATE) -Dirty $(DIRTY)

clean:
	$(GO) clean -cache -testcache
