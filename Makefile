.PHONY: backend frontend build docker test clean dev-backend dev-frontend dev-bundle lint lint-frontend lint-backend security

BACKEND_ENV ?= LISTEN_ADDR=:9096
BIN_DIR ?= $(PWD)/bin
BINARY ?= $(BIN_DIR)/clusteruptime

dev-backend:
	ADMIN_SECRET=clusteruptime-e2e-magic-key $(BACKEND_ENV) go run ./cmd/dashboard

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

build-backend:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/dashboard

build: deps build-frontend build-backend

run: build
	$(BACKEND_ENV) $(BINARY)

dev-bundle: build
	$(BACKEND_ENV) $(BINARY)

docker:
	docker build -t clusteruptime/clusteruptime .

test:
	go test ./...

lint-frontend:
	cd web && npm run lint

lint-backend:
	golangci-lint run

lint: lint-frontend lint-backend

security:
	@command -v gosec >/dev/null 2>&1 || { echo "Installing gosec..."; go install github.com/securego/gosec/v2/cmd/gosec@latest; }
	gosec -exclude-dir=web ./...

clean:
	rm -rf web/node_modules web/dist $(BIN_DIR)
	rm -rf internal/static/dist/*
	touch internal/static/dist/.gitkeep

e2e:
	cd web && npm run test:e2e

e2e-ui:
	cd web && npm run test:e2e:ui
