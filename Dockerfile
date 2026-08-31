# syntax=docker/dockerfile:1

# --- Builder stage ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=arm64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/coach-bot ./cmd/bot

# --- Final stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 1000 appuser

WORKDIR /app

COPY --from=builder /out/coach-bot /app/coach-bot

RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

ENV DB_PATH=/app/data/coach.db

EXPOSE 8080

ENTRYPOINT ["/app/coach-bot"]
