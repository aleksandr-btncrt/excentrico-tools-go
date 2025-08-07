#!/bin/bash

# Excentrico Tools Go - Build Script

set -e

echo "Building Excentrico Tools Go..."

# Build the application
go build -o excentrico-tools-go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "Next steps:"
    echo "1. Place your Google API credentials file as 'credentials.json' in this directory"
    echo "2. Run './excentrico-tools-go -create-config' to create a default configuration"
    echo "3. Edit the generated 'configuration.json' file with your settings"
    echo "4. Run './excentrico-tools-go' to start the application"
    echo ""
    echo "For more information, see README.md"
else
    echo "❌ Build failed!"
    exit 1
fi 