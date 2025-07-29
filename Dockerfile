# Use Go 1.21 or newer
FROM golang:1.21-alpine as builder

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY src/ ./src/

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o goProxy ./src/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary
COPY --from=builder /app/goProxy .

# Copy the templates.json file to multiple possible locations the app checks for
COPY --from=builder /app/src/domains/templates.json ./templates.json
COPY --from=builder /app/src/domains/ ./domains/
COPY --from=builder /app/src/domains/ ./src/domains/

CMD ["./goProxy"]
