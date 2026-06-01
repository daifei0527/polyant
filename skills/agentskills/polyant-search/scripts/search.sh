#!/bin/bash
set -e
POLYANT_API_URL="${POLYANT_API_URL:-http://localhost:8080}"
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。请先安装 Polyant CLI。"
    exit 1
fi
QUERY="${1:-}"
if [ -z "$QUERY" ]; then
    echo "用法: search.sh <query> [category] [limit]"
    exit 1
fi
CATEGORY="${2:-}"
LIMIT="${3:-5}"
CMD="pactl search \"$QUERY\" --limit $LIMIT"
if [ -n "$CATEGORY" ]; then
    CMD="$CMD --category \"$CATEGORY\""
fi
echo "搜索 Polyant 知识库: $QUERY"
echo "---"
eval $CMD
