#!/bin/bash

README_FILE="README.md"

echo "=========================================="
echo "WASM Benchmark: JSValue"
echo "=========================================="

# Check if wasmbrowsertest is installed
if ! command -v wasmbrowsertest &> /dev/null; then
    echo "⚠️  wasmbrowsertest not found. Install it with:"
    echo "   go install github.com/agnivade/wasmbrowsertest@latest"
    echo "   export PATH=\$PATH:\$(go env GOPATH)/bin"
    exit 1
fi

echo ""
echo "Running WASM benchmarks..."
echo ""

# Run benchmarks and capture output
# We filter for Benchmark, PASS, and header rows, ensuring we capture relevant info
BENCH_OUTPUT=$(GOOS=js GOARCH=wasm go test -bench=. -benchmem -benchtime=5s -tags wasm 2>&1 | grep -E "(Benchmark|PASS|pkg:|goos:|goarch:)")

if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo ""
    echo "❌ Benchmark failed"
    exit 1
fi

# Display results
echo "$BENCH_OUTPUT"

echo ""
echo "✅ Benchmark completed!"
echo ""

# Update README with results
if [ -f "$README_FILE" ]; then
    # Ensure the section header exists, append if not
    if ! grep -q "## Performance Results" "$README_FILE"; then
        echo "" >> "$README_FILE"
        echo "## Performance Results" >> "$README_FILE"
    fi

    echo "Updating $README_FILE with latest results..."
    
    # Create temporary file
    TEMP_FILE=$(mktemp)
    
    # Extract everything before the Performance Results section
    # If the file ends with the header, this might need care, but appending header above ensures it exists.
    # The reference script uses `sed -n '1,/## Performance Results/p' ... | head -n -1`.
    # This keeps everything UP TO the line BEFORE "## Performance Results".
    sed -n '1,/## Performance Results/p' "$README_FILE" | head -n -1 > "$TEMP_FILE"
    
    # Add Performance Results section
    echo "## Performance Results" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "Last updated: $(date '+%Y-%m-%d %H:%M:%S')" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo '```text' >> "$TEMP_FILE"
    echo "$BENCH_OUTPUT" >> "$TEMP_FILE"
    echo '```' >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    
    # Add rest of file if there was anything after the old block (heuristic based on previous script)
    # The previous script assumes a specific structure (code block immediately following header).
    # Since we are just adding it for the first time or updating, this is robust enough for now.
    # If we want to preserve content AFTER the table (if any), we need to skip the old table.
    
    sed -n '/## Performance Results/,$ p' "$README_FILE" | sed '1d' | sed '/^```/,/^```/d' >> "$TEMP_FILE"
    
    # Replace original file
    mv "$TEMP_FILE" "$README_FILE"
    
    echo "✅ README updated successfully!"
else
    echo "⚠️  README file not found at $README_FILE"
fi
