FROM golang:1.26.6-alpine AS builder

WORKDIR /src
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOMAXPROCS=1 \
    GOMEMLIMIT=512MiB
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN go build -p=1 -trimpath -ldflags='-s -w' -o /out/koom-server ./cmd/server
RUN go build -p=1 -trimpath -ldflags='-s -w' -o /out/koom-migrate ./cmd/migrate

FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser && mkdir -p /app/data && chown -R appuser:appuser /app/data
WORKDIR /app
COPY --from=builder /out/koom-server /app/koom-server
COPY --from=builder /out/koom-migrate /app/koom-migrate
COPY migrations /app/migrations
USER appuser

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/koom-server"]

