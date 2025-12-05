ARG NODE_VERSION=22
ARG GO_VERSION=1.24

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS frontend
WORKDIR /app
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web ./web
RUN cd web && npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
# Embed frontend
COPY --from=frontend /app/web/dist ./internal/static/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /clusteruptime ./cmd/dashboard

FROM --platform=$TARGETPLATFORM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=backend /clusteruptime /app/clusteruptime
ENV LISTEN_ADDR=:9090
EXPOSE 9090
ENTRYPOINT ["/app/clusteruptime"]
