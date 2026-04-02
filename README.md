# ChainAgent

> 基于 Claude CLI 的多 Agent 编排框架（Go 版）

ChainAgent 是一个开箱即用的多 Agent 协作开发框架，通过 `claude` CLI 驱动多个专项 Agent（Manager、Spec、Frontend、Backend、Test）并行工作，自动完成从需求分析、架构设计到代码实现、测试验收的完整开发流程。

每个 Agent 以 **Skill** 形式封装（`skills/<role>/`），包含角色定义、模型配置和规范文件，自描述、自包含、可插拔。

## 🚀 快速开始

### 1. 安装依赖

```bash
# 安装 Go >= 1.22
# https://go.dev/dl/

# 安装 Claude CLI
npm install -g @anthropic-ai/claude-code
claude login

# 安装 opencli（提供 openspec 命令，OpenSpec 工作流必须）
npm install -g opencli
opencli login
```

### 2. 安装 chainagent

```bash
go install github.com/chainagent-oss/chainagent/cmd/chainagent@latest
```

或使用一键安装脚本（macOS / Linux）：

```bash
bash install.sh
```

### 3. 初始化项目

将 ChainAgent 的 `skills/`、`prompts/`、`openspec/` 目录复制到你的项目根目录，然后执行：

```bash
# 全自动流水线
chainagent run --req 001
```

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

## ⚙️ 命令参考

```bash
# 全自动流水线（plan → develop → test → fix → pref）
chainagent run --req <id>

# 并行启动前端 + 后端开发
chainagent develop --req <id>

# 启动测试 Agent 验收
chainagent test --req <id>

# 自动修复循环（fix → test → 重复，最多10轮）
chainagent fix --req <id>

# OpenSpec 策划（生成 proposal/design/tasks）
chainagent plan --req <id>

# 生成前端 HTML Demo
chainagent demo --req <id>

# 代码质量优化
chainagent pref --req <id> --target <frontend|backend>

# Bug 专项修复
chainagent bugfix --agent <frontend|backend> --description "..."

# 查看实时进度
chainagent status [--req <id>]
```

所有命令支持 `--git-commit` 标志，执行完成后自动 git commit。

---

## 📋 开发流水线

```
阶段 1：需求沟通 → Spec Agent 生成 artifacts → 用户确认
阶段 2：Frontend + Backend 并行开发（chainagent develop）
阶段 3：Test Agent 验收测试（chainagent test）
阶段 4：失败则循环修复（chainagent fix），最多 10 轮
阶段 4.5：代码质量优化（chainagent pref）
```

---

## 🔑 环境要求

| 依赖 | 版本要求 | 必选 |
|------|---------|------|
| Go | >= 1.22 | ✅（编译 / go install） |
| Node.js | >= 18 | ✅（claude CLI / opencli） |
| claude CLI | 最新版 | ✅ |
| opencli | 最新版 | ✅（提供 openspec 命令） |

---

## 📄 许可证

MIT License — 详见 [LICENSE](./LICENSE)
