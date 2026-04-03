# ChainAgent

> 基于 Claude CLI 的多 Agent 编排框架（Go 版）

ChainAgent 是一个开箱即用的多 Agent 协作开发框架，通过 `claude` CLI 驱动多个专项 Agent（Manager、Spec、Frontend、Backend、Test）并行工作，自动完成从需求分析、架构设计到代码实现、测试验收的完整开发流程。

每个 Agent 以 **Skill** 形式封装（`skills/<role>/`），包含角色定义、模型配置和规范文件，自描述、自包含、可插拔。

## ✨ 功能特性

### 多 Agent 协作，全流程自动化
Manager Agent 作为总调度，根据用户需求自动拆解任务并并行驱动 Spec、Frontend、Backend、Test 等专项 Agent 协同完成，无需人工介入中间步骤。

### Spec 驱动开发（OpenSpec 工作流）
每个需求先由 Spec Agent 生成结构化 artifacts（proposal → specs → design → tasks），再驱动开发 Agent 按规格实现，确保需求、设计、实现三者一致。

### Git Worktree 多任务并行隔离
每个需求或 Bug 修复运行在独立的 git worktree（`.worktrees/<name>/`）中，拥有独立的文件系统和分支，多个任务可以**同时并行执行互不干扰**，主工作区始终保持干净的 master。

### 自动化测试闭环
Test Agent 根据 OpenSpec 的接受标准对前后端进行验收测试，发现问题自动生成修复请求，驱动 Frontend / Backend Agent 修复并重测，最多循环 10 轮，测试通过后自动进入代码质量优化。

### 开发规范自学习
每次修复 bug 或代码优化后，Spec Agent 会将经验沉淀到项目规范文件（`rules/frontend-rule.mdc` / `rules/backend-rule.mdc`），后续开发 Agent 读取规范后自动规避同类问题。

### Skill 插件化，零代码扩展
每个 Agent 角色以独立的 Skill 目录封装（`skills/<role>/agent.md`），新增或替换角色无需修改任何 Go 代码，复制目录即可生效。

---

## 🚀 快速开始

### 1. 安装依赖

```bash
# 安装 Go >= 1.22
# https://go.dev/dl/

# 安装 Node.js >= 18（claude CLI 和 openspec 依赖）
# https://nodejs.org/
```

### 2. 一键安装（macOS / Linux）

```bash
bash install.sh
```

脚本会自动完成：
- ✅ 检查 Go >= 1.22 / Node.js >= 18
- ✅ 安装 claude CLI（`@anthropic-ai/claude-code`）
- ✅ 安装 OpenSpec CLI（`@fission-ai/openspec`）
- ✅ 编译并安装 `chainagent` 二进制到 `$GOPATH/bin`

或手动分步安装：

```bash
# 安装 claude CLI
npm install -g @anthropic-ai/claude-code
claude login

# 安装 OpenSpec CLI
npm install -g @fission-ai/openspec@latest

# 安装 chainagent
go install github.com/Ironymonster/chainAgent/cmd/chainagent@latest
```

### 3. 初始化目标项目

将 ChainAgent 的配置目录复制到你的项目根目录：

```bash
# 克隆 ChainAgent 仓库（或直接 download ZIP）
git clone https://github.com/Ironymonster/chainAgent.git
cd chainAgent

# 将配置目录复制到目标项目
cp -r skills/ prompts/ /path/to/your-project/
cd /path/to/your-project/

# 确保目标项目是 Git 仓库（worktree 隔离需要 Git）
git init  # 如果还不是 git 仓库

# 初始化 OpenSpec 工作流
openspec init
```

> **`rules/` 不需要手动复制**。Manager Agent 在第一个需求开始时会自动执行 Rules Init（阶段 2），扫描项目实际代码（`frontend/`、`backend/`）和规范模板（`skills/*/rules/*-template.md`），生成**针对当前项目技术栈定制**的 `rules/frontend-rule.mdc` 和 `rules/backend-rule.mdc`。

### 4. 启动 Manager Agent

```bash
claude --system-prompt-file skills/manager/agent.md --model claude-opus-4-5
```

启动后即可直接与 Manager 对话，沟通需求、确认方案。Manager 会根据对话内容自动调度 Spec、Frontend、Backend、Test 等子 Agent 完成整个开发流程。

---

## � 命令参考

### 一键全流程

```bash
# 从策划到测试验收，全自动运行
chainagent run --req 001 --title "用户登录功能" --git-commit
```

### 分阶段执行

```bash
# Phase 1：OpenSpec 策划（Manager → Spec Agent 生成 artifacts）
chainagent plan --req 001 --title "用户登录功能" --git-commit

# Phase 2：并行开发（Frontend + Backend 同时运行）
chainagent develop --req 001 --git-commit

# Phase 3：验收测试
chainagent test --req 001 --git-commit

# Phase 4：修复循环（fix → test，最多 10 轮）
chainagent fix --req 001 --max-rounds 5 --git-commit

# Phase 5：代码质量优化
chainagent pref --req 001 --target frontend --git-commit
chainagent pref --req 001 --target backend  --git-commit
```

### 其他命令

```bash
# 生成前端 HTML Demo 页面
chainagent demo --req 001 --git-commit

# 针对性 Bug 修复（B 流）
chainagent bugfix --agent frontend --description "登录页面按钮无响应" --worktree fix-bug-001 --git-commit
chainagent bugfix --agent backend  --description "JWT 过期时间计算错误"  --worktree fix-bug-002 --git-commit

# 查看流水线进度
chainagent status           # 列出所有需求的进度
chainagent status --req 001 # 查看指定需求的进度

# Git Worktree 管理
chainagent worktree setup  --name req-001      # 创建隔离工作区
chainagent worktree setup  --name fix-bug-001  # Bug 修复专用工作区
chainagent worktree list                        # 列出所有活跃工作区
chainagent worktree remove --name req-001      # MR 合并后清理
```

---

## �📁 项目结构

```
chainagent/
├── README.md
├── LICENSE
├── install.sh                        # 一键安装脚本（macOS / Linux）
├── .gitignore
├── go.mod                            # Go 模块定义
├── cmd/
│   └── chainagent/
│       └── main.go                   # CLI 入口（cobra）
├── internal/
│   ├── runner/runner.go              # claude 子进程 + stream-json 解析
│   ├── orchestrator/orchestrator.go  # 流水线编排
│   ├── worktree/worktree.go          # git worktree 生命周期管理
│   ├── skill/loader.go               # Skill 目录扫描
│   └── status/status.go              # 状态文件读写
├── skills/                           # Agent Skill 插件目录（需复制到目标项目）
│   ├── manager/
│   │   ├── SKILL.md                  # 角色元数据（name/model/description）
│   │   └── agent.md                  # system prompt
│   ├── spec/
│   │   ├── SKILL.md
│   │   └── agent.md
│   ├── frontend/
│   │   ├── SKILL.md
│   │   ├── agent.md
│   │   └── rules/
│   │       └── frontend-rule-template.md  # 前端规范模板（供 genRule 参考）
│   ├── backend/
│   │   ├── SKILL.md
│   │   ├── agent.md
│   │   └── rules/
│   │       └── backend-rule-template.md   # 后端规范模板（供 genRule 参考）
│   └── test/
│       ├── SKILL.md
│       └── agent.md
└── prompts/                          # Agent 任务提示词模板（需复制到目标项目）
    ├── genRule.md
    ├── updateRule.md
    ├── addrule.md
    ├── adjustRule.md
    ├── useRule.md
    └── pref.md
```

**目标项目运行时目录结构（chainagent 部署后）：**

```
your-project/
├── skills/                           # 从 ChainAgent 复制（必须，含 rules 模板）
├── prompts/                          # 从 ChainAgent 复制（必须）
├── rules/                            # ⚡ Manager 阶段 2 自动生成，无需手动创建
│   ├── frontend-rule.mdc             # 扫描 frontend/ 代码后定制生成
│   └── backend-rule.mdc              # 扫描 backend/ 代码后定制生成
├── openspec/                         # openspec init 自动创建
│   ├── config.yaml
│   └── changes/
│       └── req-001/                  # 每个需求的 OpenSpec artifacts
│           ├── proposal.md
│           ├── specs/
│           ├── design.md
│           ├── tasks.md
│           └── report.md
├── docs/
│   ├── requirements/                 # 需求文档（REQ-001.md 等）
│   ├── contracts/                    # API 契约（OpenAPI 3.0）
│   └── index.json                    # 项目需求索引（Agent 自动维护）
├── inbox/                            # Agent 间通信文件
│   ├── frontend/                     # TASK-*.md / FIX-*.md / MSG-*.md
│   ├── backend/                      # TASK-*.md / FIX-*.md / MSG-*.md
│   └── test/                         # DONE-frontend-*.md / DONE-backend-*.md
├── reports/                          # 测试报告、修复报告
│   ├── test-report-*.md
│   └── fix-requests/
│       └── FIX-*.md
├── frontend/                         # 前端代码
├── backend/                          # 后端代码
└── .worktrees/                       # git worktree 隔离目录（gitignore）
    ├── req-001/                      # REQ-001 的独立工作区
    └── req-002/                      # REQ-002 的独立工作区（并行）
```

---

## 🤖 Agent Skill 说明

每个 Skill 是自包含的插件单元：

```
skills/frontend/
├── SKILL.md     ← frontmatter 定义模型和描述
└── agent.md     ← system prompt（角色能力和规范）
```

| Skill | 角色 | 模型 |
|-------|------|------|
| **manager** | 全流程编排、需求分析、Git 管理、进度汇报 | claude-opus-4-5 |
| **spec** | 生成 OpenSpec artifacts、API 契约、任务分发 | claude-sonnet-4-5 |
| **frontend** | React/TypeScript 前端代码实现 | claude-opus-4-5 |
| **backend** | Go/Gin 后端代码实现 | claude-sonnet-4-5 |
| **test** | 测试验收、生成测试报告、输出修复请求 | claude-sonnet-4-5 |

**新增角色**：创建 `skills/<role>/SKILL.md` + `agent.md`，无需修改任何代码。

---

## ⚙️ 运行原理

ChainAgent 通过 Go 编排器调用 `claude` CLI 子进程来驱动各 Agent。每次执行命令时，底层实际调用：

```bash
claude -p "<任务 prompt>" \
  --system-prompt-file skills/<role>/agent.md \
  --model <SKILL.md 中配置的模型> \
  --output-format stream-json \
  --dangerously-skip-permissions
```

Manager Agent 作为总调度，由编排器首先启动，再根据流水线阶段自动调度 Spec、Frontend、Backend、Test 等子 Agent。

---

## 📋 开发需求流水线 (Feature Pipeline)

```
阶段 1：环境检查 & Worktree 准备（git pull + chainagent worktree setup）
阶段 2：初始化开发规范（Rules Init，每个项目只需一次）
阶段 3：需求沟通 → Spec Agent 生成 artifacts → 用户确认
阶段 4：Frontend + Backend 并行开发（chainagent develop）
阶段 5：Test Agent 验收测试（chainagent test）
阶段 6：失败则循环修复（chainagent fix），最多 10 轮
阶段 6.5：代码质量优化（chainagent pref）
阶段 7：完成汇报，推送 MR，生成 report.md
```

## 🐛 Bug 修复流水线 (Bug Fix Pipeline)

```
阶段 B0：复现 & 定位（分析日志、读取源码、确认根因）
阶段 B1：Spec Agent 生成修复方案（proposal → design → tasks）
阶段 B2：Frontend / Backend Agent 执行修复（chainagent bugfix）
阶段 B3：Test Agent 专项验证（chainagent test）
阶段 B4：失败则循环修复，最多 5 轮（超过则人工介入）
阶段 B5：规范沉淀（将 bug 根因写入 rules），完成汇报
```

---

## 🔑 环境要求

| 依赖 | 版本要求 | 必选 |
|------|---------|------|
| Go | >= 1.22 | ✅（编译 / go install） |
| Node.js | >= 18 | ✅（claude CLI / openspec） |
| claude CLI | 最新版 | ✅ |
| openspec | 最新版 | ✅（`@fission-ai/openspec`） |
| Git | >= 2.5 | ✅（worktree 隔离需要） |

---

## 📄 许可证

MIT License — 详见 [LICENSE](./LICENSE)
