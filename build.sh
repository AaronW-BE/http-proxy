#!/bin/bash

# Default values
DEFAULT_NAME="go-proxy"
DEFAULT_OS="linux" # Native OS for .sh script

# Parameters:
# $1: Output name (optional, defaults to DEFAULT_NAME)
# $2: Target OS (optional, 'linux' or 'windows', defaults to DEFAULT_OS)

OUTPUT_NAME="${1:-$DEFAULT_NAME}"
TARGET_OS="${2:-$DEFAULT_OS}"

# Set Go environment variables for cross-compilation
export GOARCH="amd64" # Assuming amd64 for simplicity

echo "Building Go proxy server..."
echo "Target OS: $TARGET_OS"
echo "Output Name (base): $OUTPUT_NAME"

if [ "$TARGET_OS" = "windows" ]; then
  export GOOS="windows"
  if [[ "$OUTPUT_NAME" != *.exe ]]; then
    FINAL_OUTPUT_NAME="${OUTPUT_NAME}.exe"
  else
    FINAL_OUTPUT_NAME="$OUTPUT_NAME"
  fi
  echo "Building for Windows: $FINAL_OUTPUT_NAME"
elif [ "$TARGET_OS" = "linux" ]; then
  export GOOS="linux"
  # Remove .exe if it exists for Linux target
  FINAL_OUTPUT_NAME="${OUTPUT_NAME%.exe}"
  echo "Building for Linux: $FINAL_OUTPUT_NAME"
else
  echo "Error: Unsupported target OS '$TARGET_OS'. Supported: 'linux', 'windows'."
  exit 1
fi

go build -o "$FINAL_OUTPUT_NAME" main.go

if [ $? -eq 0 ]; then
  echo "Build successful! Output: $FINAL_OUTPUT_NAME"
  if [ "$TARGET_OS" = "linux" ]; then
    echo "Making the binary executable..."
    chmod +x "$FINAL_OUTPUT_NAME"
  fi
  echo "Done."
else
  echo "Build failed."
  exit 1
fi
