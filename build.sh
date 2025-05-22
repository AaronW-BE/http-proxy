#!/bin/bash

# Default output name
DEFAULT_NAME="go-proxy"

# Use the first argument as the output name if provided, otherwise use the default
OUTPUT_NAME="${1:-$DEFAULT_NAME}"

echo "Building Go proxy server..."
go build -o "$OUTPUT_NAME" main.go

if [ $? -eq 0 ]; then
  echo "Build successful! Output: $OUTPUT_NAME"
  echo "Making the binary executable..."
  chmod +x "$OUTPUT_NAME"
  echo "Done."
else
  echo "Build failed."
  exit 1
fi
