# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o arbiter ./cmd/arbiter

# Stage 2: Final image
FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/arbiter .

EXPOSE 9099

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["/app/arbiter", "healthcheck"]

ENTRYPOINT ["/app/arbiter"]
