# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /platform ./cmd/platform

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S techlane && adduser -S -G techlane techlane
WORKDIR /app
COPY --from=builder /platform /app/platform
COPY --from=builder /src/go.mod /app/go.mod
COPY --from=builder /src/services /app/services
USER techlane
EXPOSE 8080
ENV HTTP_ADDR=:8080
CMD ["/app/platform"]
