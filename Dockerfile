FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/mobilechat-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/mobilechat-migrate ./cmd/migrate

FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/mobilechat-server /app/mobilechat-server
COPY --from=builder /out/mobilechat-migrate /app/mobilechat-migrate
COPY migrations /app/migrations
USER appuser

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/mobilechat-server"]
