# Minimal Registry Server Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go docs.go ./
# Build statically
RUN CGO_ENABLED=0 GOOS=linux go build -o /registry-server .

# Run Stage
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /registry-server /app/registry-server
RUN apk add --no-cache ca-certificates

USER 1000:1000
EXPOSE 8080

CMD ["/app/registry-server"]
