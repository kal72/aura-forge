# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/aura-forge ./cmd/server

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/aura-forge /app/aura-forge
COPY --from=builder /src/web/static /app/web/static

EXPOSE 8080

ENTRYPOINT ["/app/aura-forge"]
