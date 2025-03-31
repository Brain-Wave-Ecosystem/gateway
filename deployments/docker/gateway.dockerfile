FROM golang:1.24.1-alpine AS builder

WORKDIR /app

COPY ../../go.mod ../../go.sum ./
RUN go clean -modcache &&  \
    go mod download

COPY ../.. ./
COPY ../../internal ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -o server

FROM alpine:latest AS final

COPY --from=builder /app/server /app/server

ENTRYPOINT ["/app/server"]