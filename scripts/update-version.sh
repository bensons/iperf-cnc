#!/bin/bash
# Update version in README.md based on git tag

set -e

# Get the latest git tag
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Validate version format (vX.Y.Z)
if ! echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "Warning: Latest tag '$VERSION' does not match version format (vX.Y.Z)"
    exit 0
fi

echo "Updating README.md to version $VERSION"

# Update version in README.md
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s/^\*\*Version:\*\* v[0-9]\+\.[0-9]\+\.[0-9]\+$/\*\*Version:\*\* $VERSION/" README.md
else
    # Linux
    sed -i "s/^\*\*Version:\*\* v[0-9]\+\.[0-9]\+\.[0-9]\+$/\*\*Version:\*\* $VERSION/" README.md
fi

echo "Version updated to $VERSION in README.md"

