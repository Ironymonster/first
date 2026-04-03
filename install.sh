#!/usr/bin/env bash
# install.sh - ChainAgent 一键安装脚本
# 用法：bash install.sh

set -e

echo "========================================"
echo "  ChainAgent 安装脚本"
echo "========================================"
echo ""

# ── 检查 Go ──
echo "[1/3] 检查 Go..."
if ! command -v go &>/dev/null; then
  echo "❌ 未找到 Go，请先安装 Go >= 1.22："
  echo "   https://go.dev/dl/"
  exit 1
fi
GO_VER=$(go version | awk '{print $3}')
echo "✅ Go $GO_VER"

# ── 安装 claude CLI ──
echo "[2/3] 安装 claude CLI..."
if command -v claude &>/dev/null; then
  CLAUDE_VER=$(claude --version 2>&1 | head -1)
  echo "✅ claude CLI 已安装：$CLAUDE_VER"
else
  if ! command -v node &>/dev/null; then
    echo "❌ 未找到 Node.js（claude CLI 安装需要 Node.js >= 18）"
    echo "   https://nodejs.org/"
    exit 1
  fi
  echo "   正在安装 @anthropic-ai/claude-code ..."
  npm install -g @anthropic-ai/claude-code
  echo "✅ claude CLI 安装完成"
fi

# ── 安装 OpenSpec CLI ──
echo "[3/3] 安装 OpenSpec CLI..."
if command -v openspec &>/dev/null; then
  OPENSPEC_VER=$(openspec --version 2>&1 | head -1)
  echo "✅ openspec 已安装：$OPENSPEC_VER"
else
  echo "   正在安装 @fission-ai/openspec ..."
  npm install -g @fission-ai/openspec@latest
  echo "✅ openspec 安装完成"
fi

echo ""
echo "========================================"
echo "  安装完成！"
echo "========================================"
echo ""
echo "后续步骤："
echo "  1. 运行 'claude login' 完成 Claude 账号授权"
echo "  2. 安装 chainagent binary："
echo "     go install github.com/chainagent-oss/chainagent/cmd/chainagent@latest"
echo "  3. 将 ChainAgent 复制到你的项目目录中，然后执行："
echo "     chainagent run --req 001"
echo ""
echo "详细文档请阅读 README.md"
