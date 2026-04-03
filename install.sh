#!/usr/bin/env bash
# install.sh - ChainAgent 一键安装脚本
# 用法：bash install.sh
# 支持：macOS / Linux

set -e

echo "========================================"
echo "  ChainAgent 一键安装脚本"
echo "========================================"
echo ""

# ── 辅助函数 ──────────────────────────────────

# 比较版本号，返回 0 表示 $1 >= $2
version_ge() {
  [ "$(printf '%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

# ── [1/4] 检查 Go >= 1.22 ────────────────────

echo "[1/4] 检查 Go >= 1.22 ..."
if ! command -v go &>/dev/null; then
  echo "❌ 未找到 Go，请先安装 Go >= 1.22："
  echo "   https://go.dev/dl/"
  exit 1
fi
GO_VER_FULL=$(go version | awk '{print $3}' | sed 's/go//')
if ! version_ge "$GO_VER_FULL" "1.22"; then
  echo "❌ Go 版本过低（当前 $GO_VER_FULL，需要 >= 1.22）"
  echo "   请升级：https://go.dev/dl/"
  exit 1
fi
echo "✅ Go $GO_VER_FULL"

# ── [2/4] 安装 claude CLI ────────────────────

echo "[2/4] 检查 claude CLI ..."
if command -v claude &>/dev/null; then
  CLAUDE_VER=$(claude --version 2>&1 | head -1)
  echo "✅ claude CLI 已安装：$CLAUDE_VER"
else
  if ! command -v node &>/dev/null; then
    echo "❌ 未找到 Node.js（claude CLI 需要 Node.js >= 18）"
    echo "   https://nodejs.org/"
    exit 1
  fi
  NODE_VER=$(node --version | sed 's/v//')
  if ! version_ge "$NODE_VER" "18.0.0"; then
    echo "❌ Node.js 版本过低（当前 v$NODE_VER，需要 >= 18）"
    echo "   https://nodejs.org/"
    exit 1
  fi
  echo "   正在安装 @anthropic-ai/claude-code ..."
  npm install -g @anthropic-ai/claude-code
  echo "✅ claude CLI 安装完成"
fi

# ── [3/4] 安装 OpenSpec CLI ─────────────────

echo "[3/4] 检查 OpenSpec CLI ..."
if command -v openspec &>/dev/null; then
  OPENSPEC_VER=$(openspec --version 2>&1 | head -1)
  echo "✅ openspec 已安装：$OPENSPEC_VER"
else
  if ! command -v node &>/dev/null; then
    echo "❌ 未找到 Node.js（openspec 安装需要 Node.js >= 18）"
    exit 1
  fi
  echo "   正在安装 @fission-ai/openspec ..."
  npm install -g @fission-ai/openspec@latest
  echo "✅ openspec 安装完成"
fi

# ── [4/4] 安装 chainagent 二进制 ─────────────

echo "[4/4] 安装 chainagent ..."
if go install github.com/Ironymonster/chainAgent/cmd/chainagent@latest; then
  echo "✅ chainagent 安装完成"
  # 验证可执行
  if command -v chainagent &>/dev/null; then
    echo "   位置：$(which chainagent)"
  else
    echo "⚠️  chainagent 已安装但未在 PATH 中，请将 Go bin 目录加入 PATH："
    echo "   export PATH=\$PATH:\$(go env GOPATH)/bin"
    echo "   建议将上面这行加入你的 ~/.bashrc 或 ~/.zshrc"
  fi
else
  echo "❌ chainagent 安装失败，请检查网络或手动执行："
  echo "   go install github.com/Ironymonster/chainAgent/cmd/chainagent@latest"
  exit 1
fi

echo ""
echo "========================================"
echo "  所有依赖安装完成 🎉"
echo "========================================"
echo ""
echo "后续步骤："
echo ""
echo "  1. 登录 Claude 账号（如尚未授权）："
echo "     claude login"
echo ""
echo "  2. 初始化目标项目（将配置文件复制到你的项目根目录）："
echo "     cp -r skills/ prompts/ your-project/"
echo "     cd your-project"
echo "     openspec init"
echo ""
echo "     注意：rules/ 无需手动复制，Manager Agent 启动后会"
echo "     自动扫描项目代码，生成针对该项目定制的规范文件。"
echo ""
echo "  3. 确保目标项目是 Git 仓库（worktree 隔离需要）："
echo "     git init   # 如果还不是 git 仓库"
echo ""
echo "  4. 启动 Manager Agent，开始第一个需求："
echo "     claude --system-prompt-file skills/manager/agent.md --model claude-opus-4-5"
echo ""
echo "  5. 或使用 chainagent 命令直接驱动流水线："
echo "     chainagent plan    --req 001 --title '你的需求标题'"
echo "     chainagent develop --req 001"
echo "     chainagent test    --req 001"
echo "     chainagent fix     --req 001"
echo "     chainagent run     --req 001  # 一键运行全流程"
echo ""
echo "详细文档请阅读 README.md"
