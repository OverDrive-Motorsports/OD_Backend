FROM golang:1.22 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN rm -rf resources/db \
    && mkdir -p resources/db \
    && go run github.com/steebchen/prisma-client-go generate --schema resources/schema.prisma
RUN go build -o /app/backend ./cmd/api

FROM golang:1.22 AS dbsync

WORKDIR /app

COPY --from=build /app /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates netcat-openbsd \
    && rm -rf /var/lib/apt/lists/* \

ENV DATABASE_URL="postgres://user:pass@db:5432/OverDriveDB"
CMD ["sh", "-c", "until nc -z db 5432; do echo \"waiting for db:5432\"; sleep 2; done; go run github.com/steebchen/prisma-client-go db push --schema resources/schema.prisma"]

FROM debian:bookworm-slim AS run

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /app/backend /app/backend
COPY --from=build /app/resources /app/resources

WORKDIR /app
ENV DATABASE_URL="postgres://user:pass@db:5432/OverDriveDB"

CMD ["/app/backend"]
