FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o sentry ./cmd/sentry

FROM alpine:latest

COPY --from=builder /app/sentry /usr/local/bin/
RUN mkdir -p /config
ENTRYPOINT ["sentry"] 