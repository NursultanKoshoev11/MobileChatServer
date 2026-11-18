FROM golang:1.26.11-alpine3.24 AS builder

WORKDIR /src
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOMAXPROCS=1 \
    GOMEMLIMIT=512MiB \
    GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
# Never run `go mod tidy` in the image build: the build must use exactly the
# committed go.mod/go.sum, verified against the checksum database records.
RUN go mod download && go mod verify
COPY . .
RUN go build -mod=readonly -p=1 -trimpath -ldflags='-s -w' -o /out/koom-server ./cmd/server
RUN go build -mod=readonly -p=1 -trimpath -ldflags='-s -w' -o /out/koom-migrate ./cmd/migrate

FROM alpine:3.24

RUN adduser -D -H -u 10001 appuser && mkdir -p /app/data && chown -R appuser:appuser /app/data
WORKDIR /app
COPY --from=builder /out/koom-server /app/koom-server
COPY --from=builder /out/koom-migrate /app/koom-migrate
COPY migrations /app/migrations
USER appuser

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/koom-server"]
