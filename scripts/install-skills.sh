#!/bin/bash

# Polyant Skills Installer for Claude Code
# This script installs Polyant skills for Claude Code

set -e

echo "Installing Polyant skills for Claude Code..."

# Create skills directory if it doesn't exist
SKILLS_DIR="${HOME}/.claude/skills"
mkdir -p "$SKILLS_DIR"

# Copy skills
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SOURCE_DIR="$(dirname "$SCRIPT_DIR")/skills"

if [ ! -d "$SKILLS_SOURCE_DIR" ]; then
    echo "Error: Skills directory not found at $SKILLS_SOURCE_DIR"
    exit 1
fi

for skill in polyant-config.md polyant-search.md polyant-save.md polyant-learn.md; do
    if [ -f "$SKILLS_SOURCE_DIR/$skill" ]; then
        cp "$SKILLS_SOURCE_DIR/$skill" "$SKILLS_DIR/"
        echo "Installed: $skill"
    else
        echo "Warning: $skill not found"
    fi
done

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Configure your Polyant node:"
echo "   export POLYANT_API_URL='https://your-node.example.com:8080'"
echo "   export POLYANT_API_KEY='sk_live_your_api_key'"
echo ""
echo "2. Verify connection:"
echo "   pactl status"
echo ""
echo "3. Start using skills:"
echo "   - Search: 'Search Polyant for...'"
echo "   - Save: 'Save to Polyant...'"
echo "   - Learn: 'Learn from Polyant...'"
