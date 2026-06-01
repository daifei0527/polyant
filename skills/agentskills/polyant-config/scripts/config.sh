#!/bin/bash
set -e
CONFIG_DIR="$HOME/.polyant"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"
show_config() {
    echo "当前 Polyant 配置:"
    echo "---"
    if [ -f "$CONFIG_FILE" ]; then cat "$CONFIG_FILE"; else echo "配置文件不存在"; fi
}
set_url() {
    local url="$1"
    if [ -z "$url" ]; then echo "用法: config.sh set-url <url>"; exit 1; fi
    if [ -f "$CONFIG_FILE" ]; then local config=$(cat "$CONFIG_FILE"); else local config='{}'; fi
    echo "$config" | jq --arg url "$url" '.base_url = $url' > "$CONFIG_FILE"
    echo "已设置 API URL: $url"
}
set_key() {
    local key="$1"
    if [ -z "$key" ]; then echo "用法: config.sh set-key <key>"; exit 1; fi
    if [ -f "$CONFIG_FILE" ]; then local config=$(cat "$CONFIG_FILE"); else local config='{}'; fi
    echo "$config" | jq --arg key "$key" '.api_key = $key' > "$CONFIG_FILE"
    echo "已设置 API Key"
}
case "${1:-show}" in
    show) show_config ;;
    set-url) set_url "$2" ;;
    set-key) set_key "$2" ;;
    *) echo "用法: config.sh [show|set-url|set-key] [value]"; exit 1 ;;
esac
