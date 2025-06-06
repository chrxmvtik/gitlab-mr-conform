#!/bin/bash

set -e

echo "Building GitLab MR Conformity Bot..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build for current platform
echo "Building for current platform..."
go build -o bin/bot ./cmd/bot

# Build for Linux (useful for Docker)
echo "Building for Linux..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/bot-linux ./cmd/bot

echo "Build complete!"
echo "Local binary: bin/bot"
echo "Linux binary: bin/bot-linux"