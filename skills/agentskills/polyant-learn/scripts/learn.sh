#!/bin/bash
set -e
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。"
    exit 1
fi
TOPIC="${1:-}"
LIMIT="${2:-10}"
if [ -z "$TOPIC" ]; then
    echo "用法: learn.sh <topic> [limit]"
    exit 1
fi
echo "搜索学习材料: $TOPIC"
echo "---"
pactl search "$TOPIC" --limit $LIMIT
