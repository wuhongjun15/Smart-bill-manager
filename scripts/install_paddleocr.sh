#!/bin/bash
# Install PaddleOCR dependencies

echo "Installing PaddleOCR..."

# Check if pip is available
if command -v pip3 &> /dev/null; then
    pip3 install paddlepaddle paddleocr
elif command -v pip &> /dev/null; then
    pip install paddlepaddle paddleocr
else
    echo "Error: pip not found. Please install Python first."
    exit 1
fi

echo "PaddleOCR installed successfully!"
