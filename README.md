# ChainAgent

> 基于 Claude CLI 的多 Agent 编排框架（Go 版）

ChainAgent 是一个开箱即用的多 Agent 协作开发框架，通过 `claude` CLI 驱动多个专项 Agent（Manager、Spec、Frontend、Backend、Test）并行工作，自动完成从需求分析、架构设计到代码实现、测试验收的完整开发流程。

每个 Agent 以 **Skill** 形式封装（`skills/<role>/`），包含角色定义、模型配置和规范文件，自描述、自包含、可插拔。

## 🚀 快速开始

### 1. 安装依赖

```bash
# 安装 Go >= 1.22
# https://go.dev/dl/

# 安装 Claude CLI，https://www.npmjs.com/package/@anthropic-ai/claude-code
npm install -g @anthropic-ai/claude-code
claude login

# 安装 OpenSpec CLI（OpenSpec 工作流必须），参考：https://github.com/Fission-AI/OpenSpec
npm install -g @fission-ai/openspec@latest
```

### 2. 安装 chainagent

```bash
go install github.com/Ironymonster/chainAgent/cmd/chainagent@latest
```

或使用一键安装脚本（macOS / Linux）：

```bash
bash install.sh
```

### 3. 初始化目标项目

将 ChainAgent 的配置目录复制到你的项目根目录（`chainagent` 二进制在运行时会从当前目录查找 `skills/`）：

```bash
# 从 ChainAgent 仓库复制必要的配置目录到你的项目
cp -r skills/ prompts/ your-project/
cd your-project
```

> 首次启动 Manager Agent 时会自动检测并执行 `openspec init`（创建 `openspec/config.yaml`），你也可以手动初始化：
> ```bash
> openspec init
> ```

### 4. 启动 Manager Agent

```bash
claude --system-prompt-file skills/manager/agent.md --model claude-opus-4-5
```

启动后即可直接与 Manager 对话，沟通需求、确认方案。Manager 会根据对话内容自动调度 Spec、Frontend、Backend、Test 等子 Agent 完成整个开发流程。

---

## 📁 项目结构

```
chainagent/
├── README.md
├── LICENSE
├── install.sh                      # 一键安装脚本
├── go.mod                          # Go 模块定义
├── cmd/
│   └── chainagent/
│       └── main.go                 # CLI 入口（cobra）
├── internal/
│   ├── runner/runner.go            # claude subprocess + stream-json 解析
│   ├── orchestrator/orchestrator.go # 流水线编排
│   ├── skill/loader.go             # Skill 目录扫描
│   └── status/status.go            # 状态文件读写
└── skills/                         # Agent Skill 插件目录（跟项目走）
    ├── manager/
    │   ├── SKILL.md                # 角色元数据（name/model/description）
    │   └── agent.md                # system prompt
    ├── spec/
    │   ├── SKILL.md
    │   └── agent.md
    ├── frontend/
    │   ├── SKILL.md
    │   ├── agent.md
    │   └── rules/
    │       └── frontend-rule-template.md
    ├── backend/
    │   ├── SKILL.md
    │   ├── agent.md
    │   └── rules/
    │       └── backend-rule-template.md
    └── test/
        ├── SKILL.md
        └── agent.md
```

---

## 🤖 Agent Skill 说明

每个 Skill 是自包含的插件单元：

```
skills/frontend/
├── SKILL.md     ← frontmatter 定义模型和描述
├── agent.md     ← system prompt（角色能力和规范）
└── rules/       ← 该角色专用规范文件（可选）
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

## ⚙️ 运行原理与命令参考

### 运行原理

ChainAgent 通过 Go 编排器调用 `claude` CLI 子进程来驱动各 Agent。每次执行命令时，底层实际调用：

```bash
claude -p "<任务 prompt>" \
  --system-prompt-file skills/<role>/agent.md \
  --model <SKILL.md 中配置的模型> \
  --output-format stream-json \
  --dangerously-skip-permissions
```

Manager Agent 作为总调度，由编排器首先启动，再根据流水线阶段自动调度 Spec、Frontend、Backend、Test 等子 Agent。


## 📋 开发需求流水线 (Feature Pipeline)

```
阶段 0：环境检查 & Git 分支准备
阶段 0.5：初始化开发规范（Rules Init，每个项目只需一次）
阶段 1：需求沟通 → Spec Agent 生成 artifacts → 用户确认
阶段 2：Frontend + Backend 并行开发（chainagent develop）
阶段 3：Test Agent 验收测试（chainagent test）
阶段 4：失败则循环修复（chainagent fix），最多 10 轮
阶段 4.5：代码质量优化（chainagent pref）
阶段 5：完成汇报，生成 report.md
```

## 🐛 Bug 修复流水线 (Bug Fix Pipeline)

```
阶段 B0：复现 & 定位（分析日志、读取源码、确认根因）
阶段 B1：Spec Agent 生成修复方案（proposal → design → tasks）
阶段 B2：Frontend / Backend Agent 执行修复（按需单独或并行）
阶段 B3：Test Agent 验收测试
阶段 B4：失败则循环修复，最多 10 轮
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

---

## 📄 许可证

MIT License — 详见 [LICENSE](./LICENSE)
