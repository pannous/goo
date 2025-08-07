#!/bin/bash

# Build .goo package into Go package
# Usage: ./build_goo_package.sh <package_dir> <output_dir>

PACKAGE_DIR=$1
OUTPUT_DIR=$2

if [[ -z "$PACKAGE_DIR" || -z "$OUTPUT_DIR" ]]; then
    echo "Usage: $0 <package_dir> <output_dir>"
    exit 1
fi

echo "Building .goo package from $PACKAGE_DIR to $OUTPUT_DIR"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Copy .goo files as .go files to output directory
for goo_file in "$PACKAGE_DIR"/*.goo; do
    if [[ -f "$goo_file" ]]; then
        base_name=$(basename "$goo_file" .goo)
        cp "$goo_file" "$OUTPUT_DIR/${base_name}.go"
        echo "Converted $goo_file -> $OUTPUT_DIR/${base_name}.go"
    fi
done

echo "Package build complete"