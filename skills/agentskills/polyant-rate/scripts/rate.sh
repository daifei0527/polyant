#!/bin/bash
set -e
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。"
    exit 1
fi
ENTRY_ID="${1:-}"
SCORE="${2:-}"
COMMENT="${3:-}"
if [ -z "$ENTRY_ID" ] || [ -z "$SCORE" ]; then
    echo "用法: rate.sh <entry-id> <score> [comment]"
    exit 1
fi
if [ "$SCORE" -lt 1 ] || [ "$SCORE" -gt 5 ]; then
    echo "错误: 评分必须在 1-5 之间"
    exit 1
fi
CMD="pactl entry rate \"$ENTRY_ID\" --score $SCORE"
if [ -n "$COMMENT" ]; then CMD="$CMD --comment \"$COMMENT\""; fi
echo "评价条目: $ENTRY_ID (评分: $SCORE/5)"
echo "---"
eval $CMD
