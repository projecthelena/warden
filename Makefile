.PHONY: backend frontend build docker test test-frontend test-all clean dev-backend dev-frontend dev-bundle lint lint-frontend lint-backend security govuln vuln secrets audit hooks check docs

BACKEND_ENV ?= LISTEN_ADDR=:9096
BIN_DIR ?= $(PWD)/bin
BINARY ?= $(BIN_DIR)/warden

dev-backend:
	ADMIN_SECRET=warden-e2e-magic-key $(BACKEND_ENV) go run ./cmd/dashboard

backend: dev-backend

dev-frontend:
	cd web && npm install && npm run dev

frontend: dev-frontend

deps:
	cd web && npm install
	go mod download

build-frontend:
	cd web && npm run build
	rm -rf internal/static/dist/*
	cp -r web/dist/* internal/static/dist/
	touch internal/static/dist/.gitkeep

docs:
	@command -v swag >/dev/null 2>&1 || { echo "Installing swag..."; go install github.com/swaggo/swag/cmd/swag@latest; }
	swag init -g cmd/dashboard/main.go -o internal/docs --parseDependency

build-backend:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/dashboard

build: deps build-frontend build-backend

run: build
	ADMIN_SECRET=warden-e2e-magic-key $(BACKEND_ENV) $(BINARY)

dev-bundle: build
	$(BACKEND_ENV) $(BINARY)

docker:
	docker build -t projecthelena/warden .

test:
	go test ./...

test-frontend:
	cd web && npm run test

test-all: test-frontend test

lint-frontend:
	cd web && npm run lint

lint-backend:
	golangci-lint run

lint: lint-frontend lint-backend

security:
	@command -v gosec >/dev/null 2>&1 || { echo "Installing gosec..."; go install github.com/securego/gosec/v2/cmd/gosec@latest; }
	gosec -exclude-dir=web ./...

govuln:
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

vuln:
	@command -v trivy >/dev/null 2>&1 || { echo "Install trivy: brew install trivy"; exit 1; }
	trivy fs --severity CRITICAL,HIGH --exit-code 1 .

secrets:
	@command -v gitleaks >/dev/null 2>&1 || { echo "Install gitleaks: brew install gitleaks"; exit 1; }
	gitleaks detect --source . -v

audit: security govuln vuln
	@echo "All security checks passed!"

clean:
	rm -rf web/node_modules web/dist $(BIN_DIR)
	rm -rf internal/static/dist/*
	touch internal/static/dist/.gitkeep

e2e:
	cd web && npm run test:e2e

e2e-headed:
	cd web && npx playwright test --headed

e2e-ui:
	cd web && npm run test:e2e:ui

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed! Pre-push will run: lint, tests, and security checks."

# Run all pre-push checks manually (same as pre-push hook)
check: lint test-all security
	@echo "All checks passed!"
