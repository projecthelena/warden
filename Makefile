.PHONY: backend frontend build docker test clean dev-backend dev-frontend dev-bundle lint lint-frontend lint-backend

BACKEND_ENV ?= LISTEN_ADDR=:9096
BIN_DIR ?= $(PWD)/bin
BINARY ?= $(BIN_DIR)/clusteruptime

dev-backend:
	$(BACKEND_ENV) go run ./cmd/dashboard

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

clean:
	rm -rf web/node_modules web/dist $(BIN_DIR)
	rm -rf internal/static/dist/*
	touch internal/static/dist/.gitkeep
