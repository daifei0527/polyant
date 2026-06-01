#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
echo "=== Polyant 智能体技能安装器 ==="
echo ""
detect_agents() {
    local agents=()
    if [ -d ~/.claude ]; then agents+=("claude-code"); fi
    if [ -d ~/.agents ]; then agents+=("codex"); fi
    if [ -d ~/.hermes ]; then agents+=("hermes"); fi
    if [ -d ~/.openclaw ]; then agents+=("openclaw"); fi
    echo "${agents[@]}"
}
install_agentskills() {
    echo "安装 agentskills.io 标准技能..."
    if [ -d ~/.agents ]; then
        mkdir -p ~/.agents/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.agents/skills/
        echo "✓ 已安装到 Codex (~/.agents/skills/)"
    fi
    if [ -d ~/.hermes ]; then
        mkdir -p ~/.hermes/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.hermes/skills/
        echo "✓ 已安装到 Hermes (~/.hermes/skills/)"
    fi
}
install_openclaw() {
    echo "安装 OpenClaw 技能..."
    if [ -d ~/.openclaw ]; then
        mkdir -p ~/.openclaw/skills
        cp -r "$PROJECT_ROOT/skills/openclaw/"* ~/.openclaw/skills/
        echo "✓ 已安装到 OpenClaw (~/.openclaw/skills/)"
    fi
}
install_claude_code() {
    echo "安装 Claude Code 技能..."
    if [ -d ~/.claude ]; then
        mkdir -p ~/.claude/skills
        cp -r "$PROJECT_ROOT/skills/polyant-*.md" ~/.claude/skills/ 2>/dev/null || true
        echo "✓ 已安装到 Claude Code (~/.claude/skills/)"
    fi
}
main() {
    local agents=$(detect_agents)
    if [ -z "$agents" ]; then
        echo "未检测到已安装的智能体"
        echo "请先安装以下智能体之一："
        echo "  - Claude Code, Codex, Hermes Agent, OpenClaw"
        exit 1
    fi
    echo "检测到以下智能体: $agents"
    echo ""
    install_agentskills
    install_openclaw
    install_claude_code
    echo ""
    echo "=== 安装完成 ==="
    echo ""
    echo "下一步：配置 Polyant 连接"
    echo "  export POLYANT_API_URL=http://your-node:8080"
    echo "  export POLYANT_API_KEY=your-api-key"
}
main "$@"
