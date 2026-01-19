# Multi-stage build for Go bot application
# Supports both AMD64 and ARM64 architectures

FROM --platform=$BUILDPLATFORM golang:1.24.4-alpine AS build

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for cross-compilation
ARG TARGETOS=linux
ARG TARGETARCH

# Build the application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags='-w -s' \
    -o bot ./cmd/bot

# Final stage - minimal distroless image
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static-debian13:nonroot

# Copy binary and configs from build stage
COPY --from=build --chown=nonroot:nonroot /app/bot /
COPY --from=build --chown=nonroot:nonroot /app/configs ./configs

# Use nonroot user (already defined in distroless image)
USER nonroot

# Run the bot application
CMD ["/bot"]