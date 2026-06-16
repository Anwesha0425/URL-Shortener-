FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
# Copy source first so go mod tidy can resolve all imports
COPY . .
# Download declared deps then tidy to generate go.sum
RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o url-service ./cmd/main.go

FROM alpine:3.18
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/url-service .
EXPOSE 8001
CMD ["./url-service"]
