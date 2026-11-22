# syntax=docker/dockerfile:1
#
# Production image of ingestion-service. Build from the repository root (the build needs
# shared/ next to the module):
#   docker build -f services/ingestion-service/Dockerfile -t overdrive/ingestion-service .
#
# The image holds the binary only (no source, no .env): configuration comes from
# the environment. It runs as the distroless nonroot user (65532). Commands:
#   (none)        serve on HTTP_PORT
#   healthcheck   probe GET /health on HTTP_PORT (container health check)

FROM golang:1.27 AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0 GOFLAGS=-buildvcs=false

# Dependencies first: cached until a go.mod or go.sum changes. (go.* also matches a
# module without go.sum.)
COPY shared/ shared/
COPY services/ingestion-service/go.* services/ingestion-service/
RUN --mount=type=cache,target=/go/pkg/mod cd services/ingestion-service && go mod download

COPY services/ingestion-service/ services/ingestion-service/

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd services/ingestion-service && go build -trimpath -ldflags="-s -w" -o /out/ingestion-service .

# Pure Go binary: static, no libc needed.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ingestion-service /usr/local/bin/ingestion-service
USER nonroot:nonroot
EXPOSE 3005
ENTRYPOINT ["/usr/local/bin/ingestion-service"]
