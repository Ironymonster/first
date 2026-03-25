# ChainAgent

> 基于 CodeMaker 的多 Agent 全自动开发流水线

🤖 **5 Agent 协作** &nbsp;|&nbsp; 📋 **OpenSpec 规范驱动** &nbsp;|&nbsp; ⚡ **前后端并行开发** &nbsp;|&nbsp; 🔁 **自动测试修复循环** &nbsp;|&nbsp; 📡 **WebSocket 实时监控**

---

## 功能概览

| 功能 | 说明 |
|---|---|
| **5 Agent 协作体系** | Manager / Spec / Frontend / Backend / Test 各司其职，全自动流水线 |
| **OpenSpec 规范驱动** | spec-driven 工作流，需求 → proposal → specs → design → tasks 完整生命周期 |
| **前端 Demo 预览** | 开发前生成 HTML Demo，与用户确认设计方向后再进入实际开发 |
| **前后端并行开发** | Frontend + Backend Agent 同步启动，最大化开发效率 |
| **自动测试验收** | Test Agent 对照 OpenSpec 接受标准执行验收测试，生成 QA 报告 |
| **智能修复循环** | 测试失败自动触发修复，最多 10 轮，同步更新开发规范防止复现 |
| **代码质量优化** | 测试通过后执行 pref 优化阶段，清理冗余代码、提升复用性和性能 |
| **Git 全生命周期管理** | 自动创建特性分支、原子 commit、push 并通过 `-o` 参数创建 MR |
| **WebSocket 实时监控** | `/ws/monitor` 端点广播 Agent 进度、流水线状态、token 消耗 |
| **LangGraph 兼容协议** | 实现 `/threads/{id}/runs/stream`、`/info`、`/assistants` 接口 |
| **开发规范自动生成** | 扫描代码库生成 frontend-rule.mdc / backend-rule.mdc，错误时自动更新 |
| **彩色终端 + token 统计** | 每个 Agent 独立颜色，实时展示工具调用、步骤进度、费用明细 |
| **状态持久化** | 运行时状态写入 `.orchestrator/`，断点可查，live.json 实时同步 |

---

## 系统架构

```
用户 (CodeMaker TUI / Web UI)
  │
  ▼
Manager Agent                        claude-opus-4-6
  │  全程主导，与用户对话，驱动流水线
  │
  ├─── spawn ──► Spec Agent           claude-sonnet-4-6
  │              生成需求文档、OpenSpec artifacts、API 契约、任务分发文件
  │
  └─── bash ───► orchestrator.py      Python 编排器
                  │  调度子 Agent，管理 codemaker serve 生命周期
                  │
                  ├── agent_runner.py  封装 codemaker HTTP API 调用
                  │
                  ├── 并行 ──► Frontend Agent    claude-opus-4-6
                  │            实现 React/TypeScript 前端代码
                  │
                  ├── 并行 ──► Backend Agent     claude-sonnet-4-6
                  │            实现 Python FastAPI 后端代码
                  │
                  └────────► Test Agent          claude-sonnet-4-6
                             验收测试、生成 QA 报告、产出修复请求
```

**codemaker serve**（端口 4096）是所有 Agent 的底层驱动，orchestrator.py 通过其 HTTP API 启动和监控每个 Agent session；FastAPI 后端（端口 2024）通过订阅 codemaker serve 的 SSE `/event` 流，将事件转换后通过 WebSocket 推送给前端监控面板。

### 完整流水线（6 阶段 + 代码优化）

```
Phase 0   Git 分支创建     同步 main，创建 feat/req-<id> 特性分支
Phase 0.5 Rules 初始化     Spec Agent 扫描代码库生成前后端开发规范（首次）
Phase 1   需求策划         Manager 沟通需求 → Spec Agent 生成完整 OpenSpec artifacts
          └─ Demo 预览     Frontend Agent 生成 HTML Demo，用户确认后进入开发
Phase 2   并行开发         Frontend Agent + Backend Agent 同步执行
Phase 3   验收测试         Test Agent 对照 spec 接受标准执行自动化验收
Phase 4   迭代修复         fix 命令触发修复循环（最多 10 轮），同步更新开发规范
Phase 4.5 代码质量优化     pref 命令驱动前后端优化：复用性、性能、冗余清理
Phase 5   完成汇报         Spec Agent 生成 report.md，推送分支并创建 MR
```

---

## 技术栈

**前端**

| 技术 | 版本 | 用途 |
|---|---|---|
| Next.js | 15 | App Router 框架 |
| React | 19 | UI 库 |
| TypeScript | 5.7 | 类型系统 |
| TailwindCSS | v4 | 样式框架 |
| Radix UI | latest | 无障碍基础组件 |
| @langchain/langgraph-sdk | ^1.0.0 | LangGraph 客户端，对接后端流式接口 |
| framer-motion | ^12 | 动画 |
| react-markdown | ^10 | Markdown 渲染（含 KaTeX 数学公式） |
| nuqs | ^2 | URL 查询参数状态管理 |
| recharts | ^2 | 数据图表 |
| pnpm | 10.5.1 | 包管理器 |

**后端**

| 技术 | 版本 | 用途 |
|---|---|---|
| Python | 3.12 | 运行时 |
| FastAPI | latest | API 框架 |
| LangGraph | ^1.0 | 多 Agent 状态图 |
| uvicorn | latest | ASGI 服务器 |
| Pydantic v2 | latest | 数据校验 |

**Agent 编排**

| 技术 | 用途 |
|---|---|
| codemaker CLI | Agent 驱动底层，`codemaker serve` 提供 HTTP API |
| orchestrator.py | 多 Agent 调度、并行控制、状态追踪 |
| agent_runner.py | 封装 codemaker HTTP API 调用，实时事件解析 |

**文档工作流**

| 技术 | 用途 |
|---|---|
| OpenSpec (spec-driven) | 结构化需求 → 设计 → 任务的完整工作流 |
| openspec CLI | 创建 change、生成 artifact 模板、检查状态 |

**AI 模型**

| Agent | 模型 |
|---|---|
| Manager、Frontend | `netease-codemaker/claude-opus-4-6` |
| Spec、Backend、Test | `netease-codemaker/claude-sonnet-4-6` |

---

## 环境要求

| 依赖 | 版本 | 说明 |
|---|---|---|
| codemaker CLI | 最新版 | **核心依赖，必须安装** |
| Python | 3.12+ | 编排器运行环境 |
| Node.js | 18+ | 前端开发服务 |
| pnpm | 10+ | 前端包管理器 |
| Git | 2.x+ | 分支和提交管理 |
| openspec CLI | 最新版 | OpenSpec 工作流支持 |

---

## 安装与启动

### 1. 安装 codemaker CLI

**macOS / Linux：**

```bash
curl -fsSL https://codemaker.netease.com/package/codemaker-cli/install | bash
```

**Windows（PowerShell）：**

```powershell
irm https://codemaker.netease.com/package/codemaker-cli/install.ps1 | iex
```

**Windows（CMD）：**

```cmd
powershell -Command "irm https://codemaker.netease.com/package/codemaker-cli/install.ps1 | iex"
```

验证安装：

```bash
codemaker --version
```

### 2. 一键启动

**Windows：**

```cmd
start.bat
```

**macOS / Linux：**

```bash
bash start.sh
```

启动脚本自动完成以下 4 步：

1. **Step 1** — 启动 `codemaker serve --port 4096`（内部 Agent 驱动服务）
2. **Step 2** — 启动 FastAPI 后端（`python -m uvicorn server:app --port 2024`）
3. **Step 3** — 启动 Next.js 前端（`npm run dev`，端口 3000，约需 10 秒）
4. **Step 4** — 前台启动 Manager Agent（交互式 TUI）

启动后服务地址：

| 服务 | 地址 | 说明 |
|---|---|---|
| Manager Agent TUI | 当前终端 | 直接输入需求开始使用 |
| 前端 Web UI | http://localhost:3000 | 实时监控面板 |
| 后端 API | http://localhost:2024 | FastAPI + WebSocket 监控 |
| codemaker serve | http://localhost:4096 | 内部 Agent 驱动（无需直接访问） |
| API 文档 | http://localhost:2024/docs | FastAPI 自动文档 |
| 日志目录 | `.codemaker/logs/` | 各服务日志文件 |

---

## 使用方式

### 交互式模式（推荐）

通过 `start.bat` / `start.sh` 启动后，在 Manager Agent TUI 中直接输入需求描述，Manager 会自动引导整个流程：

```
你：我需要一个用户管理页面，支持增删改查和分页
Manager：好的，我来帮你分析需求...（开始策划）
```

Manager Agent 会按流水线阶段自动执行，无需手动干预。

### Orchestrator 命令行模式

在项目根目录执行各阶段命令，适合手动介入或恢复中断的任务：

```bash
# 策划阶段（Manager 已完成沟通后）
python orchestrator/orchestrator.py plan --req 001 --git-commit

# 生成 Demo 预览
python orchestrator/orchestrator.py demo --req 001

# 并行开发（阻塞等待两个 Agent 完成）
python orchestrator/orchestrator.py develop --req 001 --git-commit

# 测试验收
python orchestrator/orchestrator.py test --req 001 --git-commit

# 修复（测试失败时）
python orchestrator/orchestrator.py fix --req 001 --git-commit

# 代码优化
python orchestrator/orchestrator.py pref --req 001 --target frontend --git-commit
python orchestrator/orchestrator.py pref --req 001 --target backend --git-commit

# 查看进度
python orchestrator/orchestrator.py status --req 001
```

### 全自动模式

无人值守，从开发到 MR 一键完成（适合 CI 场景）：

```bash
python orchestrator/orchestrator.py run --req 001
```

---

## Orchestrator 命令参考

### 子命令

| 命令 | 用途 | 使用阶段 |
|---|---|---|
| `run` | 全自动流水线：develop → test → fix → push MR | 无人值守 |
| `plan` | 触发 Spec Agent 生成 OpenSpec artifacts 和 API 契约 | 阶段 1 |
| `demo` | 启动 Frontend Agent 生成纯 HTML Demo 页面 | 阶段 1.3 |
| `develop` | 并行启动 Frontend + Backend Agent，阻塞等待完成 | 阶段 2 |
| `test` | 启动 Test Agent 执行验收测试，产出 QA 报告 | 阶段 3 |
| `fix` | 读取 FIX 文件，并行启动 Frontend + Backend 修复 | 阶段 4 |
| `pref` | 驱动前端或后端执行代码质量优化 | 阶段 4.5 |
| `status` | 查看所有需求或指定需求的实时进度 | 随时 |

### 公共参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--req <id>` | 需求 ID（对应 `docs/requirements/REQ-<id>.md`） | — |
| `--git-commit` | 完成后自动执行 `git add -A && git commit` | 关闭 |
| `--no-git` | 禁用所有 Git 操作（仅 `run` 命令） | 关闭 |
| `--max-fix-rounds <n>` | 修复最大轮次 | `10` |
| `--port <port>` | codemaker serve 端口，`0` 强制 subprocess 模式 | `4096` |
| `--base-branch <branch>` | 基础分支（仅 `run` 命令） | `main` |
| `--target <frontend\|backend>` | 优化目标（仅 `pref` 命令） | 两者 |

### 输出格式

每个命令完成后，stdout 中输出 `@@ORCHESTRATOR_RESULT@@` 标记，随后跟随 JSON 结果供 Manager Agent 解析：

```json
{
  "command": "develop",
  "req_id": "001",
  "frontend": { "status": "ok", "exit_code": 0, "elapsed_seconds": 120.5 },
  "backend":  { "status": "ok", "exit_code": 0, "elapsed_seconds": 150.3 }
}
```

```json
{
  "command": "test",
  "req_id": "001",
  "passed": true,
  "bug_count": 0,
  "unresolved_files": [],
  "summary": "所有测试通过"
}
```

---

## Agent 体系

### 职责一览

| Agent | 角色 | 模型 | 定义文件 |
|---|---|---|---|
| **Manager** | 项目经理：需求沟通、流水线编排、向用户汇报 | claude-opus-4-6 | `.codemaker/agents/manager.md` |
| **Spec** | 技术文档：生成 OpenSpec artifacts、API 契约、Rules | claude-sonnet-4-6 | `.codemaker/agents/spec.md` |
| **Frontend** | 前端开发：实现 React/TypeScript 代码、组件设计 | claude-opus-4-6 | `.codemaker/agents/frontend.md` |
| **Backend** | 后端开发：实现 FastAPI 接口、业务逻辑、数据模型 | claude-sonnet-4-6 | `.codemaker/agents/backend.md` |
| **Test** | 质量保障：验收测试、QA 报告、修复请求分发 | claude-sonnet-4-6 | `.codemaker/agents/test.md` |

### 权限矩阵

| 权限 | Manager | Spec | Frontend | Backend | Test |
|---|:---:|:---:|:---:|:---:|:---:|
| read | ✅ | ✅ | ✅ | ✅ | ✅ |
| edit | ❌ | ✅ | ✅ | ✅ | ✅ |
| bash | ✅ | ✅ | ✅ | ✅ | ✅ |
| glob / grep | ✅ | ✅ | ✅ | ✅ | ✅ |
| task (spawn) | ✅ | ❌ | ❌ | ❌ | ❌ |
| webfetch | ✅ | ❌ | ✅ | ✅ | ❌ |
| question | ✅ | ❌ | ✅ | ✅ | ✅ |

Manager 是唯一能 spawn 子 Agent 的角色（`task(subagent_type="spec")`），Spec Agent 是唯一能写入 `openspec/` 和 `docs/` 的角色，Frontend/Backend 只能修改各自的代码目录。

---

## OpenSpec 工作流

ChainAgent 使用 OpenSpec 的 `spec-driven` 模式，将需求转化为结构化设计文档后再驱动开发。

### Artifact 生成顺序

```
需求输入
  │
  ▼
docs/requirements/REQ-<id>.md          需求文档
  │
  ▼ openspec new change "req-<id>"
  │
  ├─► proposal.md                       Why + What + Capabilities + Impact
  ├─► specs/<capability>/spec.md        每个能力点的接受标准和技术约束（各一个文件）
  ├─► design.md                         技术架构、数据模型、API 设计、前后端交互
  └─► tasks.md                          可勾选任务清单（区分 frontend / backend / testing）
```

### 生成的附属产物

- `docs/contracts/api-req-<id>.yaml` — OpenAPI 3.0 格式 API 契约，Frontend/Backend/Test 严格遵守
- `inbox/frontend/TASK-<id>.md` — 前端任务分发文件（含 artifacts 引用路径）
- `inbox/backend/TASK-<id>.md` — 后端任务分发文件

### 典型 Spec Agent 调用

Manager 通过以下方式 spawn Spec Agent（禁止使用 `category` 参数）：

```python
task(subagent_type="spec", run_in_background=False, prompt="""
## 任务指令
为 req-001 创建完整的 OpenSpec artifacts（proposal → specs → design → tasks），
生成 API 契约和任务分发文件。

## 需求内容
<详细需求描述>

## 关键决策
<用户确认的技术选型和设计决策>
""")
```

---

## 项目目录结构

```
chainAgent/
├── start.bat                        # Windows 一键启动脚本
├── start.sh                         # macOS/Linux 一键启动脚本
│
├── .codemaker/
│   ├── agents/                      # Agent 定义文件
│   │   ├── manager.md               # Manager Agent（流水线主控）
│   │   ├── spec.md                  # Spec Agent（文档生成）
│   │   ├── frontend.md              # Frontend Agent（React/TS 开发）
│   │   ├── backend.md               # Backend Agent（FastAPI 开发）
│   │   └── test.md                  # Test Agent（验收测试）
│   ├── orchestrator/                # 编排器
│   │   ├── orchestrator.py          # 主入口：命令解析、阶段编排、Git 集成
│   │   └── agent_runner.py          # Agent 调用封装：HTTP API + 事件解析
│   ├── prompts/                     # Prompt 工具链
│   │   ├── genRule.md               # 从零生成规范文件
│   │   ├── updateRule.md            # 局部更新规范
│   │   ├── addrule.md               # 新增规范条目
│   │   ├── adjustRule.md            # 按模板重整规范格式
│   │   ├── useRule.md               # 加载并遵循规范
│   │   ├── pref.md                  # 用户偏好记录
│   │   ├── readMe.md                # README 生成
│   │   └── ask.md                   # 需求澄清提问
│   ├── rules/                       # 自动生成的开发规范
│   │   ├── frontend-rule.mdc        # 前端开发规范（含示例）
│   │   ├── backend-rule.mdc         # 后端开发规范（含示例）
│   │   ├── frontend-rule-template.md # 前端规范模板
│   │   └── backend-rule-template.md  # 后端规范模板
│   └── logs/                        # 运行日志（自动生成）
│       ├── codemaker-serve.log
│       ├── backend.log
│       └── frontend.log
│
├── backend/
│   ├── server.py                    # FastAPI 主服务（含 WebSocket 监控）
│   ├── graph.py                     # LangGraph 多 Agent 状态图
│   └── tests/                       # 后端测试文件
│
├── frontend/
│   ├── package.json
│   └── src/
│       ├── app/                     # Next.js App Router
│       │   └── api/[..._path]/      # LangGraph API 代理
│       ├── components/
│       │   ├── chain-agent/         # 多 Agent 监控 UI 组件
│       │   │   ├── AgentSidebar.tsx # Agent 状态侧边栏
│       │   │   ├── PipelinePanel.tsx # 流水线进度面板
│       │   │   ├── TaskPanel.tsx    # 任务详情面板
│       │   │   ├── ReqSwitcher.tsx  # 需求切换器
│       │   │   ├── ChainAgentProvider.tsx # WebSocket 状态 Provider
│       │   │   └── types.ts         # 类型定义（与后端严格对齐）
│       │   ├── thread/              # 会话线程 UI（消息、工具调用展示）
│       │   └── ui/                  # shadcn/ui 基础组件
│       ├── providers/               # LangGraph 客户端 + Stream Provider
│       └── hooks/                   # 自定义 Hooks
│
├── openspec/
│   ├── config.yaml                  # OpenSpec 配置（spec-driven 模式）
│   └── changes/                     # 每个需求的 artifacts
│       └── req-<id>/
│           ├── proposal.md
│           ├── specs/<capability>/spec.md
│           ├── design.md
│           ├── tasks.md
│           ├── frontend-report.md   # 前端开发报告
│           ├── backend-report.md    # 后端开发报告
│           └── report.md            # 项目完成报告
│
├── docs/
│   ├── requirements/REQ-<id>.md     # 需求文档
│   └── contracts/api-req-<id>.yaml  # OpenAPI 3.0 API 契约
│
├── inbox/                           # Agent 间通信目录
│   ├── frontend/TASK-<id>.md        # 前端任务分发
│   ├── backend/TASK-<id>.md         # 后端任务分发
│   └── test/DONE-*.md               # 开发完成信号
│
├── reports/                         # 测试和流程报告
│   ├── test-report-<id>.md          # QA 测试报告
│   ├── pipeline-report-<id>.md      # 流程执行报告（含耗时和成本）
│   └── fix-requests/FIX-<id>-*.md  # 修复请求
│
└── .orchestrator/                   # 运行时状态（自动生成）
    ├── live.json                    # 实时 Agent 活动状态
    └── streams/                     # Agent 输出流缓存
```

---

## 开发规范体系

ChainAgent 通过 Spec Agent 自动生成和维护两份开发规范文件，确保 Frontend/Backend Agent 的输出质量一致。

### 规范文件

| 文件 | 覆盖范围 |
|---|---|
| `.codemaker/rules/frontend-rule.mdc` | React 组件结构、TypeScript 用法、API 调用规范、状态管理、样式规范 |
| `.codemaker/rules/backend-rule.mdc` | FastAPI 路由规范、异步编程、Pydantic 模型、错误处理、数据库操作 |

每条规范包含 ✅ 正确示例和 ❌ 错误示例，代码精简不超过 20 行，注释全部使用中文。

### Prompt 工具链

| Prompt | 用途 | 触发时机 |
|---|---|---|
| `genRule.md` | 扫描代码库从零生成规范 | 首次初始化（Phase 0.5）|
| `updateRule.md` | 局部更新指定章节 | 测试发现 bug 后自动触发 |
| `addrule.md` | 插入新规范条目 | 发现新错误模式 |
| `adjustRule.md` | 按模板重整格式 | 规范文件格式混乱时 |
| `useRule.md` | 加载规范供 Agent 遵循 | 每次开发任务开始前 |
| `pref.md` | 记录用户个人偏好 | Phase 4.5 代码优化 |

**自动更新机制**：当 Test Agent 发现 bug 并产出修复请求时，Manager 同步 spawn Spec Agent 分析错误模式，将防止同类问题的规范条目更新到对应规范文件，下一轮开发时自动生效。

---

## Web UI 实时监控

### WebSocket 端点

`ws://localhost:2024/ws/monitor` — 全局监控通道，连接后立即推送全量快照，随后持续推送增量事件：

| 事件类型 | 内容 |
|---|---|
| `snapshot` | 全量状态快照（req_states）|
| `agent_status` | Agent 任务状态变更（来自 live.json 轮询）|
| `agent_token` | 实时 token 流（来自 codemaker serve SSE）|
| `pipeline_phase` | 流水线阶段状态推断结果 |
| `ping` | 30 秒保活心跳 |

### 前端监控组件

| 组件 | 职责 |
|---|---|
| `ChainAgentProvider` | WebSocket 连接管理、状态聚合、事件分发 |
| `AgentSidebar` | 显示各 Agent 状态、当前步骤、token 消耗、最近日志 |
| `PipelinePanel` | 可视化 6 阶段流水线进度（含时间轴） |
| `TaskPanel` | 展示当前活跃任务的子任务列表和日志详情 |
| `ReqSwitcher` | 多需求切换，查看不同 REQ 的进度 |

### LangGraph 兼容接口

后端实现了完整的 LangGraph SDK 协议，前端可直接使用 `@langchain/langgraph-sdk` 对接：

| 接口 | 说明 |
|---|---|
| `GET /info` | Server 信息，含 assistants 列表 |
| `GET /assistants` | 返回 5 个 Agent 的配置（含颜色、图标、描述）|
| `POST /assistants/search` | 按 graph_id 过滤 assistant |
| `POST /threads` | 创建会话 |
| `POST /threads/search` | 搜索会话 |
| `POST /threads/{id}/runs/stream` | 流式执行（SSE，兼容 LangGraph stream_mode=values）|
| `GET /threads/{id}/state` | 获取会话当前状态 |
| `POST /threads/{id}/history` | 获取状态历史 |
| `WS /ws/monitor` | 实时监控广播通道 |

---

## 日志与调试

### 日志位置

| 日志文件 | 内容 |
|---|---|
| `.codemaker/logs/codemaker-serve.log` | codemaker serve 服务日志 |
| `.codemaker/logs/backend.log` | FastAPI 后端日志 |
| `.codemaker/logs/frontend.log` | Next.js 前端日志 |

### 终端彩色输出

orchestrator.py 运行时实时输出结构化日志：

```
[18:30:01] >  启动 codemaker serve (port=4096)...
[18:30:02] +  codemaker serve 就绪: http://127.0.0.1:4096

[frontend] ▶ Step 1
[frontend]   ⚡ bash: 创建组件目录结构 (1200ms)
[frontend]   📝 edit: UserTable.tsx (820ms)
[frontend]   └ step: in:1200 out:450 | $0.0023

[backend]  ▶ Step 1
[backend]   ⚡ bash: 初始化 API 路由 (2100ms)
```

- `[frontend]` 黄色 / `[backend]` 紫色 / `[test]` 绿色 / `[manager]` 青色
- `⚡ bash` / `📝 edit` / `🔍 grep` / `📖 read` — 工具调用类型和耗时
- `└ step:` — 当前步骤的 token 消耗和费用

### 状态持久化

`.orchestrator/live.json` 实时记录所有 Agent 的运行状态（步骤、token、耗时），后端每 2 秒轮询并通过 WebSocket 广播变更，支持断点查看和 Dashboard 展示。

---

## License

MIT

## 致谢

- [CodeMaker](https://codemaker.netease.com) — AI 编程助手，提供 Agent 驱动底层
- [OpenSpec](https://openspec.dev) — 规范驱动开发工作流
- [LangGraph](https://github.com/langchain-ai/langgraph) — 多 Agent 状态图框架
- [agent-chat-ui](https://github.com/langchain-ai/agent-chat-ui) — 前端 UI 基础框架
