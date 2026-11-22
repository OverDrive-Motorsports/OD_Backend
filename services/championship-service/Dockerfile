# syntax=docker/dockerfile:1
#
# Production image of championship-service. Build from the repository root (the build needs
# shared/ next to the module):
#   docker build -f services/championship-service/Dockerfile -t overdrive/championship-service .
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
COPY services/championship-service/go.* services/championship-service/
RUN --mount=type=cache,target=/go/pkg/mod cd services/championship-service && go mod download

COPY services/championship-service/ services/championship-service/

# The Prisma client is gitignored and excluded from the context: generate it here, for the
# engine of this Debian base (debian-openssl-3.0.x), which the runtime image can execute.
# The Prisma CLI downloads its binaries into /root/.cache: the mount is locked so parallel image
# builds (docker compose up --build) never read an engine another build is still writing (a
# truncated engine embedded in the client segfaults at start).
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache,id=prisma-cache,sharing=locked \
    cd services/championship-service && go run github.com/steebchen/prisma-client-go generate --schema resources/schema.prisma

# The runtime base ships libssl3 only: fail here, not at start, if the Prisma CLI picked another
# engine target (it falls back to openssl-1.1.x when it cannot detect OpenSSL).
RUN test -f services/championship-service/resources/db/query-engine-debian-openssl-3.0.x_gen.go \
    || { echo "unexpected Prisma engine target:"; ls services/championship-service/resources/db; exit 1; }

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd services/championship-service && go build -trimpath -ldflags="-s -w" -o /out/championship-service .

# glibc, libssl3 and libgcc for the Prisma query engine, which the binary unpacks into /tmp
# and runs at start.
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /out/championship-service /usr/local/bin/championship-service
USER nonroot:nonroot
EXPOSE 3003
ENTRYPOINT ["/usr/local/bin/championship-service"]
