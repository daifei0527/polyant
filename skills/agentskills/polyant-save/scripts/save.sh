#!/bin/bash
set -e
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。"
    exit 1
fi
TITLE="${1:-}"
CONTENT="${2:-}"
CATEGORY="${3:-}"
TAGS="${4:-}"
if [ -z "$TITLE" ] || [ -z "$CONTENT" ]; then
    echo "用法: save.sh <title> <content> [category] [tags]"
    exit 1
fi
CMD="pactl entry create --title \"$TITLE\" --content \"$CONTENT\""
if [ -n "$CATEGORY" ]; then CMD="$CMD --category \"$CATEGORY\""; fi
if [ -n "$TAGS" ]; then CMD="$CMD --tags \"$TAGS\""; fi
echo "保存知识到 Polyant: $TITLE"
echo "---"
eval $CMD
