ARG NODE_VERSION=22
ARG GO_VERSION=1.24

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS frontend
WORKDIR /app
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web ./web
RUN cd web && npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
# Debian images usually have git and ca-certificates. Update if needed.
# RUN apt-get update && apt-get install -y git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
# Embed frontend
COPY --from=frontend /app/web/dist ./internal/static/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-s -w -X github.com/clusteruptime/clusteruptime/internal/api.Version=${VERSION}" -o /clusteruptime ./cmd/dashboard
# Prepare data directory
RUN mkdir -p /data

FROM --platform=$TARGETPLATFORM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=backend /clusteruptime /app/clusteruptime
# Copy /data with correct permissions for nonroot
COPY --from=backend --chown=65532:65532 /data /data
ENV LISTEN_ADDR=:9090
ENV DB_PATH=/data/clusteruptime.db
EXPOSE 9090
VOLUME ["/data"]
ENTRYPOINT ["/app/clusteruptime"]
