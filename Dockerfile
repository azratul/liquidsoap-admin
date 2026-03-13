FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o radio-admin ./cmd/server

FROM alpine:3.21

RUN adduser -D -u 1001 radio
WORKDIR /app

COPY --from=builder /build/radio-admin .
COPY --from=builder /build/web      ./web

USER radio

EXPOSE 8080
CMD ["./radio-admin"]
