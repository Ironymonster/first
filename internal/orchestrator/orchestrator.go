package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Ironymonster/chainAgent/internal/runner"
	"github.com/Ironymonster/chainAgent/internal/status"
	"github.com/Ironymonster/chainAgent/internal/worktree"
)

const (
	// DefaultMaxFixRounds 是修复循环的默认最大轮数。
	// 超过此轮数后流水线报错终止，防止无限循环。
	DefaultMaxFixRounds = 10

	// jsonSummaryMarker 是 orchestrator 在 stdout 输出结果 JSON 时使用的固定前缀标记。
	// Manager Agent 通过扫描此标记来定位和解析命令执行结果。
	jsonSummaryMarker = "@@ORCHESTRATOR_RESULT@@"
)

// TestResult 保存一次测试运行的结果。
type TestResult struct {
	Passed   bool    // 测试是否通过（exit_code=0 且 Agent 输出 passed:true）
	ExitCode int     // Test Agent 进程的退出码
	Elapsed  float64 // 测试耗时（秒）
}

// Orchestrator 协调多 Agent 流水线的执行。
// 每个公开方法对应流水线中的一个阶段，可单独调用，也可通过 RunFull 串联执行。
type Orchestrator struct {
	Root   string         // 项目根目录（skills/、docs/ 等都在此目录下）
	runner *runner.Runner // Agent 执行器，负责驱动 claude CLI 子进程
}

// New 创建一个针对指定项目根目录的 Orchestrator。
// 会同时初始化 Runner 并加载所有技能定义（skills/*/SKILL.md）。
func New(root string) (*Orchestrator, error) {
	r, err := runner.New(root)
	if err != nil {
		return nil, err
	}
	return &Orchestrator{Root: root, runner: r}, nil
}

// ── 日志辅助函数 ──────────────────────────────────────────────────────────────

// log 以统一格式向 stdout 打印带时间戳和标签的日志。
// tag 常用值：RUN / OK / ERR / WARN
func log(msg, tag string) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, tag, msg)
}

// printResult 在 stdout 输出 @@ORCHESTRATOR_RESULT@@ 标记及结果 JSON。
// Manager Agent 通过解析此输出判断阶段执行结果。
// 输出格式：@@ORCHESTRATOR_RESULT@@ {"phase":"...","req_id":"...","exit_code":0,"elapsed":1.23}
func printResult(phase, reqID string, exitCode int, elapsed float64) {
	summary := map[string]any{
		"phase":     phase,
		"req_id":    reqID,
		"exit_code": exitCode,
		"elapsed":   elapsed,
	}
	data, _ := json.Marshal(summary)
	fmt.Printf("%s %s\n", jsonSummaryMarker, string(data))
}

// ── Git 辅助函数 ──────────────────────────────────────────────────────────────

// GitCommit 在项目根目录执行 git add -A && git commit -m <message>。
// 供各命令在 --git-commit 标志存在时调用。
func (o *Orchestrator) GitCommit(message string) error {
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", message},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = o.Root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// ── Worktree 辅助函数 ─────────────────────────────────────────────────────────

// worktreePathForReq 按名称查找 worktree，返回其绝对路径。
// 若未找到则返回空字符串，Agent 将在项目根目录运行（无隔离）。
func (o *Orchestrator) worktreePathForReq(name string) string {
	wts, _ := worktree.List(o.Root)
	for _, wt := range wts {
		if wt == name {
			return filepath.Join(o.Root, ".worktrees", name)
		}
	}
	return ""
}

// SetupWorktree 为给定任务名创建（或复用）git worktree。
// name 示例："req-001"、"fix-login"。
// 返回 worktree 的绝对路径。
func (o *Orchestrator) SetupWorktree(name string) (string, error) {
	branch := worktree.BranchName(name)
	wt, err := worktree.Setup(o.Root, name, branch)
	if err != nil {
		return "", fmt.Errorf("创建 worktree %s 失败: %w", name, err)
	}
	return wt.Path, nil
}

// RemoveWorktree 删除指定任务名的 worktree（MR 合并后清理）。
func (o *Orchestrator) RemoveWorktree(name string) error {
	return worktree.Remove(o.Root, name)
}

// ListWorktrees 返回当前所有活跃的 worktree 短名称列表。
func (o *Orchestrator) ListWorktrees() ([]string, error) {
	return worktree.List(o.Root)
}

// GitCommitInWorktree 在指定 worktree 路径下执行 git add -A && git commit。
// 适用于任务在隔离 worktree 中完成后的提交操作。
func (o *Orchestrator) GitCommitInWorktree(wtPath, message string) error {
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", message},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath // 注意：在 worktree 目录而非项目根目录执行
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// ── Phase 1: 策划 ─────────────────────────────────────────────────────────────

// RunPlanning 执行流水线第 1 阶段——由 Manager Agent 完成 OpenSpec 策划。
//
// 执行步骤：
//  1. 校验需求文件 docs/requirements/REQ-<id>.md 是否存在
//  2. 初始化流水线状态，Phase 设为 "planning"
//  3. 启动 Manager Agent，引导其完成 OpenSpec artifacts 创建及任务分发文件生成
//  4. 完成后将 Phase 更新为 "planning_done"，输出结果 JSON
func (o *Orchestrator) RunPlanning(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("Phase 1/4: OpenSpec 策划 (REQ-%s %s)", reqID, title), "RUN")

	// 校验需求文档是否存在，避免 Manager Agent 运行后才发现文件缺失。
	reqFile := filepath.Join(o.Root, "docs", "requirements", fmt.Sprintf("REQ-%s.md", reqID))
	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		return fmt.Errorf("需求文件不存在: %s", reqFile)
	}

	// 初始化或读取已有的流水线状态。
	changeName := "req-" + reqID
	s, err := status.Read(o.Root, reqID)
	if err != nil || s == nil {
		s, err = status.CreateInitial(o.Root, reqID, changeName, title)
		if err != nil {
			return err
		}
	}
	// 更新当前阶段为 planning，写入磁盘供 `chainagent status` 查询。
	s.Phase = "planning"
	s.ManagerStatus = "in_progress"
	s.ChangeName = changeName
	if title != "" {
		s.Title = title
	}
	s.PipelineStatus = "in_progress"
	_ = status.Write(o.Root, reqID, s)

	// 构建 Manager Agent 的任务提示词。
	// 引导 Manager 按 OpenSpec 工作流依次创建 proposal → specs → design → tasks，
	// 并最终生成前后端任务分发文件（inbox/frontend/TASK-<id>.md 等）。
	prompt := fmt.Sprintf(`请完成以下策划任务，使用 OpenSpec 工作流：

1. 阅读需求文档: docs/requirements/REQ-%s.md

2. 创建 OpenSpec change:
   openspec new change "%s"

3. 按顺序完成所有 OpenSpec artifacts:
   - 先查看状态: openspec status --change "%s"
   - 获取指引: openspec instructions <artifact-id> --change "%s"
   - 依次创建: proposal → specs (每个capability一个) → design → tasks
   - 每个 artifact 创建完后再查看状态确认

4. 在 design artifact 完成后，额外生成:
   - API 契约: docs/contracts/api-%s.yaml (OpenAPI 3.0 格式)

5. 在 tasks artifact 完成后，生成任务分发文件:
   - inbox/frontend/TASK-%s.md (引用 openspec artifacts 路径)
   - inbox/backend/TASK-%s.md (引用 openspec artifacts 路径)
   任务分发文件中设置 change_name: "%s"

所有内容使用中文。`,
		reqID, changeName, changeName, changeName,
		changeName, reqID, reqID, changeName)

	result, err := o.runner.Run(ctx, "manager", prompt, runner.RunOptions{
		ReqID: reqID, Title: "策划-REQ-" + reqID,
	})
	if err != nil {
		return err
	}

	// 重新读取状态（Manager Agent 运行期间可能有外部修改），更新最终结果。
	s, _ = status.Read(o.Root, reqID)
	if s == nil {
		s = &status.PipelineStatus{ReqID: reqID, ChangeName: changeName}
	}
	if result.ExitCode == 0 {
		s.ManagerStatus = "completed"
		s.Phase = "planning_done" // 策划完成，Phase 向前推进
		log("策划完成", "OK")
	} else {
		s.ManagerStatus = "failed"
		s.PipelineStatus = "failed"
		log(fmt.Sprintf("策划失败 (exit=%d)", result.ExitCode), "ERR")
	}
	_ = status.Write(o.Root, reqID, s)

	printResult("planning", reqID, result.ExitCode, result.Elapsed)

	if result.ExitCode != 0 {
		return fmt.Errorf("planning failed (exit=%d)", result.ExitCode)
	}
	return nil
}

// ── Phase 2: 并行开发 ─────────────────────────────────────────────────────────

// RunDevelop 执行流水线第 2 阶段——Frontend 与 Backend Agent 并行开发。
//
// 两个 Agent 各自读取 inbox/ 中的任务文件独立开发，通过 errgroup 并发运行。
// 任意一个 Agent 失败则整体失败（errgroup 语义）。
// 若存在对应的 git worktree，Agent 将在隔离目录中运行；否则在项目根目录运行。
func (o *Orchestrator) RunDevelop(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("Phase 2/4: 并行开发 (REQ-%s %s)", reqID, title), "RUN")

	// 读取已有状态，若不存在则初始化（允许单独调用 develop 命令）。
	s, _ := status.Read(o.Root, reqID)
	if s == nil {
		changeName := "req-" + reqID
		var err error
		s, err = status.CreateInitial(o.Root, reqID, changeName, title)
		if err != nil {
			return err
		}
	}
	changeName := s.ChangeName
	if changeName == "" {
		changeName = "req-" + reqID // 兼容旧状态文件
	}

	// 更新阶段状态。
	s.Phase = "development"
	s.PipelineStatus = "in_progress"
	_ = status.Write(o.Root, reqID, s)

	// 如果为该需求创建过 worktree，在隔离目录运行；否则回退到项目根目录。
	wtPath := o.worktreePathForReq("req-" + reqID)

	// 前后端任务文件路径（由 Planning 阶段的 Manager Agent 生成）。
	fePath := fmt.Sprintf("inbox/frontend/TASK-%s.md", reqID)
	bePath := fmt.Sprintf("inbox/backend/TASK-%s.md", reqID)

	fePrompt := fmt.Sprintf(`请完成以下前端开发任务。

任务文件: %s
OpenSpec change: %s

工作步骤（参见 skills/frontend/agent.md）：
1. 确认 worktree 工作目录正确（pwd 末段含 .worktrees/）
2. 加载规范：读取 rules/frontend-rule.mdc
3. 按顺序阅读 OpenSpec artifacts（proposal → specs → design → tasks → contracts → TASK 文件）
4. 只在 frontend/ 目录下实现代码，每完成一个功能执行 pnpm build 验证
5. 在 tasks.md 中勾选已完成的前端任务项
6. 检查并处理 inbox/backend/MSG-backend-*.md 中的未读消息
7. 创建 openspec/changes/<name>/frontend-report.md 完成报告
8. 创建 inbox/test/DONE-frontend-%s.md 完成通知（含 change_name 字段）`, fePath, changeName, reqID)

	bePrompt := fmt.Sprintf(`请完成以下后端开发任务。

任务文件: %s
OpenSpec change: %s

工作步骤（参见 skills/backend/agent.md）：
1. 确认 worktree 工作目录正确（pwd 末段含 .worktrees/）
2. 加载规范：读取 rules/backend-rule.mdc
3. 按顺序阅读 OpenSpec artifacts（proposal → specs → design → tasks → contracts → TASK 文件）
4. 只在 backend/ 目录下实现代码，完成后运行 go test ./... 验证
5. 在 tasks.md 中勾选已完成的后端任务项
6. 检查并处理 inbox/frontend/MSG-frontend-*.md 中的未读消息
7. 创建 openspec/changes/<name>/backend-report.md 完成报告
8. 创建 inbox/test/DONE-backend-%s.md 完成通知（含 change_name 字段）`, bePath, changeName, reqID)

	// 用于统计整个并行阶段的总耗时。
	var feResult, beResult *runner.Result
	startTime := time.Now()

	// 使用 errgroup 并发启动前后端 Agent，任意失败则取消另一个。
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		feResult, err = o.runner.Run(gctx, "frontend", fePrompt, runner.RunOptions{
			ReqID: reqID, Title: "前端-REQ-" + reqID, WorkDir: wtPath,
		})
		return err
	})
	g.Go(func() error {
		var err error
		beResult, err = o.runner.Run(gctx, "backend", bePrompt, runner.RunOptions{
			ReqID: reqID, Title: "后端-REQ-" + reqID, WorkDir: wtPath,
		})
		return err
	})

	runErr := g.Wait()
	elapsed := time.Since(startTime).Seconds()

	// 综合判断退出码：errgroup 错误、前端失败、后端失败，任意一个非零即失败。
	exitCode := 0
	if runErr != nil {
		exitCode = 1
	}
	if feResult != nil && feResult.ExitCode != 0 {
		exitCode = feResult.ExitCode
	}
	if beResult != nil && beResult.ExitCode != 0 {
		exitCode = beResult.ExitCode
	}

	// 重新读取状态以防 Agent 运行期间外部修改，更新最终结果。
	s, _ = status.Read(o.Root, reqID)
	if s == nil {
		s = &status.PipelineStatus{ReqID: reqID}
	}
	if exitCode == 0 {
		log("并行开发完成", "OK")
	} else {
		s.PipelineStatus = "failed"
		log(fmt.Sprintf("并行开发失败 (exit=%d)", exitCode), "ERR")
	}
	_ = status.Write(o.Root, reqID, s)

	printResult("develop", reqID, exitCode, elapsed)
	if exitCode != 0 {
		return fmt.Errorf("develop failed (exit=%d)", exitCode)
	}
	return nil
}

// ── Phase 3: 测试验收 ─────────────────────────────────────────────────────────

// RunTest 执行流水线第 3 阶段——Test Agent 对实现进行验收测试。
//
// 测试通过的判定逻辑：
//   - Test Agent 进程 exit_code == 0
//   - Test Agent 在 stdout 中输出了 @@ORCHESTRATOR_RESULT@@ {...,"passed":true}
//
// 两个条件同时满足才视为通过（防止 Agent 崩溃时误判为通过）。
func (o *Orchestrator) RunTest(ctx context.Context, reqID, title string) (*TestResult, error) {
	log(fmt.Sprintf("Phase 3/4: 测试验收 (REQ-%s %s)", reqID, title), "RUN")

	// 读取 changeName 以构建正确的 OpenSpec 文件路径。
	s, _ := status.Read(o.Root, reqID)
	changeName := "req-" + reqID
	if s != nil && s.ChangeName != "" {
		changeName = s.ChangeName
	}

	// 构建测试 Agent 的提示词，要求其在完成后输出标准结果标记。
	prompt := fmt.Sprintf(`请根据 OpenSpec change "%s" 的验收标准，对 REQ-%s 的实现进行测试验收。

change_name: %s
req_id: %s

参考文件：
- openspec/changes/%s/specs/ — 每个 spec 的接受标准（逐条验证）
- openspec/changes/%s/tasks.md — 任务完成情况
- openspec/changes/%s/design.md — 技术设计
- docs/contracts/api-%s.yaml — API 契约

详细流程参见 skills/test/agent.md。

验收完成后，在 stdout 最后单独输出一行（不要放在代码块内）：
%s {"phase":"test","req_id":"%s","passed":true,"exit_code":0}

注意：若测试失败，将上面的 true 改为 false，其余字段不变。passed 只能是 true 或 false 字面量。`,
		changeName, reqID,
		changeName, reqID,
		changeName, changeName, changeName, changeName,
		jsonSummaryMarker, reqID)

	// 在 worktree 中运行（如果存在），与开发阶段保持一致。
	wtPath := o.worktreePathForReq("req-" + reqID)
	startTime := time.Now()
	result, err := o.runner.Run(ctx, "test", prompt, runner.RunOptions{
		ReqID: reqID, Title: "测试-REQ-" + reqID, WorkDir: wtPath,
	})
	elapsed := time.Since(startTime).Seconds()
	if err != nil {
		return nil, err
	}

	// 双重判定：exit_code=0 且 Agent 主动输出了 passed:true。
	passed := result.ExitCode == 0 && extractPassed(result.RawOutput)

	if passed {
		log("测试通过 ✅", "OK")
	} else {
		log(fmt.Sprintf("测试失败 (exit=%d)", result.ExitCode), "WARN")
	}

	printResult("test", reqID, result.ExitCode, elapsed)
	return &TestResult{Passed: passed, ExitCode: result.ExitCode, Elapsed: elapsed}, nil
}

// extractPassed 扫描 Agent 原始输出，找到 @@ORCHESTRATOR_RESULT@@ 标记行，
// 解析其后的 JSON 并返回 "passed" 字段的值。
// 若未找到标记或解析失败，返回 false（保守策略，触发修复循环）。
func extractPassed(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, jsonSummaryMarker) {
			continue
		}
		// 截取标记之后的 JSON 字符串。
		idx := strings.Index(line, jsonSummaryMarker)
		jsonStr := strings.TrimSpace(line[idx+len(jsonSummaryMarker):])
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			continue // JSON 格式错误，继续扫描下一行
		}
		if p, ok := m["passed"].(bool); ok {
			return p
		}
	}
	return false // 未找到标记或字段，保守返回 false
}

// ── Phase 4: 修复循环 ─────────────────────────────────────────────────────────

// RunFix 执行单轮修复——Frontend 与 Backend Agent 并行读取各自 inbox 中的修复请求。
//
// Test Agent 在测试失败后会将 bug 分类写入：
//   - inbox/frontend/FIX-<id>-<seq>.md  — 前端 bug
//   - inbox/backend/FIX-<id>-<seq>.md   — 后端 bug
//
// 本函数分别引导前后端 Agent 读取各自目录并修复，修复完成后将文件状态标记为 "resolved"。
func (o *Orchestrator) RunFix(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("修复中 (REQ-%s %s)", reqID, title), "RUN")

	// 读取 changeName 以构建正确的上下文信息。
	s, _ := status.Read(o.Root, reqID)
	changeName := "req-" + reqID
	if s != nil && s.ChangeName != "" {
		changeName = s.ChangeName
	}

	// 在 worktree 中运行（与开发阶段保持一致）。
	wtPath := o.worktreePathForReq("req-" + reqID)

	// 前端 Agent 只处理 inbox/frontend/ 下 status 为 unread 的修复请求，与后端完全独立。
	fePrompt := fmt.Sprintf(`请处理 inbox/frontend/ 目录下的修复请求文件。

步骤：
1. 列出所有 FIX-*.md 文件：ls inbox/frontend/FIX-*.md
2. 对每个文件，读取其 frontmatter 中的 status 字段
3. 只处理 status 为 "unread" 的文件，跳过 "resolved" 的文件
4. 按 FIX 文件描述修复对应的前端 bug
5. 修复完成后将该文件的 status 字段改为 "resolved"

OpenSpec change: %s，需求 ID: REQ-%s。
详细修复流程参见 skills/frontend/agent.md 的「处理修复请求 场景一」。`, changeName, reqID)

	// 后端 Agent 只处理 inbox/backend/ 下 status 为 unread 的修复请求。
	bePrompt := fmt.Sprintf(`请处理 inbox/backend/ 目录下的修复请求文件。

步骤：
1. 列出所有 FIX-*.md 文件：ls inbox/backend/FIX-*.md
2. 对每个文件，读取其 frontmatter 中的 status 字段
3. 只处理 status 为 "unread" 的文件，跳过 "resolved" 的文件
4. 按 FIX 文件描述修复对应的后端 bug（修复后运行 go test ./... 验证）
5. 修复完成后将该文件的 status 字段改为 "resolved"

OpenSpec change: %s，需求 ID: REQ-%s。
详细修复流程参见 skills/backend/agent.md 的「处理修复请求 场景一」。`, changeName, reqID)

	// 前后端并行修复，互不依赖。
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err := o.runner.Run(gctx, "frontend", fePrompt, runner.RunOptions{
			ReqID: reqID, Title: "修复FE-REQ-" + reqID, WorkDir: wtPath,
		})
		return err
	})
	g.Go(func() error {
		_, err := o.runner.Run(gctx, "backend", bePrompt, runner.RunOptions{
			ReqID: reqID, Title: "修复BE-REQ-" + reqID, WorkDir: wtPath,
		})
		return err
	})
	return g.Wait()
}

// RunFixLoop 循环执行 Fix → Test，直到测试通过或超过最大轮数。
//
// 每轮开始时更新 Phase 为 "fixing-N"，测试时更新为 "testing-N"，
// 便于通过 `chainagent status` 实时追踪修复进度。
// 通过后 Phase 更新为 "completed"；耗尽轮数后更新为 "fix_failed"。
func (o *Orchestrator) RunFixLoop(ctx context.Context, reqID, title string, maxRounds int) error {
	if maxRounds <= 0 {
		maxRounds = DefaultMaxFixRounds
	}

	// updatePhase 是内联辅助函数，读取最新状态后只修改 Phase 字段并写回。
	// 使用闭包避免在每处调用时重复读取状态。
	updatePhase := func(phase string) {
		s, _ := status.Read(o.Root, reqID)
		if s == nil {
			s = &status.PipelineStatus{ReqID: reqID}
		}
		s.Phase = phase
		_ = status.Write(o.Root, reqID, s)
	}

	for round := 1; round <= maxRounds; round++ {
		log(fmt.Sprintf("Fix loop 第 %d/%d 轮", round, maxRounds), "RUN")

		// 标记当前为第 N 轮修复阶段。
		updatePhase(fmt.Sprintf("fixing-%d", round))
		if err := o.RunFix(ctx, reqID, title); err != nil {
			return fmt.Errorf("fix round %d: %w", round, err)
		}

		// 修复完成后立即重新测试。
		updatePhase(fmt.Sprintf("testing-%d", round))
		tr, err := o.RunTest(ctx, reqID, title)
		if err != nil {
			return err
		}
		if tr.Passed {
			log(fmt.Sprintf("✅ 测试通过（第 %d 轮修复后）", round), "OK")
			updatePhase("completed")
			printResult("fix", reqID, 0, 0)
			return nil
		}
		// 本轮测试未通过，继续下一轮（日志已在 RunTest 中打印）。
	}

	// 所有轮次耗尽，标记为修复失败。
	updatePhase("fix_failed")
	printResult("fix", reqID, 1, 0)
	return fmt.Errorf("超过最大修复轮数 (%d)，测试仍未通过", maxRounds)
}

// ── Phase 5: 代码质量优化 ─────────────────────────────────────────────────────

// RunPref 对指定目标（frontend 或 backend）执行代码质量优化。
// 优化内容参考 prompts/pref.md 中的指引（性能、可读性、规范合规性等）。
// 此阶段为可选优化，失败不影响主流水线（RunFull 中以 WARN 级别记录错误）。
func (o *Orchestrator) RunPref(ctx context.Context, reqID, target, title string) error {
	log(fmt.Sprintf("代码质量优化: %s (REQ-%s)", target, reqID), "RUN")

	prompt := fmt.Sprintf(`请对 REQ-%s 的 %s 代码进行质量优化。
参考 prompts/pref.md 中的优化指引。`, reqID, target)

	// 与开发和测试阶段保持一致，在同一个 worktree 中运行，确保能访问到代码。
	wtPath := o.worktreePathForReq("req-" + reqID)

	result, err := o.runner.Run(ctx, target, prompt, runner.RunOptions{
		ReqID: reqID, Title: "Pref-" + target + "-REQ-" + reqID, WorkDir: wtPath,
	})
	if err != nil {
		return err
	}
	printResult("pref", reqID, result.ExitCode, result.Elapsed)
	return nil
}

// ── 针对性 Bug 修复 ───────────────────────────────────────────────────────────

// RunBugfix 针对单个 bug 启动指定 Agent 进行精准修复。
// 适用于 B 流（已上线 bug 的快速修复），与 A 流（新需求修复循环）独立。
//
// 参数：
//   - agentRole:    执行修复的 Agent 角色，"frontend" 或 "backend"
//   - description:  bug 描述，应包含根因、涉及文件和修复方向，信息越详细越好
//   - worktreeName: 预先创建的 worktree 名称，如 "fix-bug-001"；为空则在项目根目录运行
func (o *Orchestrator) RunBugfix(ctx context.Context, agentRole, description, worktreeName string) error {
	log(fmt.Sprintf("Bug 修复: %s — %s", agentRole, description), "RUN")

	// 通过显式传入的 worktreeName 查找 worktree 路径。
	// Manager 在调用 bugfix 之前应先通过 `chainagent worktree setup --name fix-bug-<seq>` 创建好。
	wtPath := ""
	if worktreeName != "" {
		wtPath = o.worktreePathForReq(worktreeName)
	}

	prompt := fmt.Sprintf(`请修复以下 bug：%s`, description)
	result, err := o.runner.Run(ctx, agentRole, prompt, runner.RunOptions{
		Title: "Bugfix-" + agentRole, WorkDir: wtPath,
	})
	if err != nil {
		return err
	}
	printResult("bugfix", "", result.ExitCode, result.Elapsed)
	return nil
}

// ── Demo 页面生成 ─────────────────────────────────────────────────────────────

// RunDemo 驱动 Frontend Agent 根据 OpenSpec artifacts 生成一个纯 HTML Demo 页面。
// Demo 页面保存到 frontend/demo/demo-<reqID>.html，可直接在浏览器中预览。
func (o *Orchestrator) RunDemo(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("前端 HTML Demo (REQ-%s)", reqID), "RUN")

	// 读取 changeName 以定位正确的 OpenSpec artifacts 路径。
	s, _ := status.Read(o.Root, reqID)
	changeName := "req-" + reqID
	if s != nil && s.ChangeName != "" {
		changeName = s.ChangeName
	}

	prompt := fmt.Sprintf(`请根据以下 OpenSpec artifacts 生成一个纯 HTML Demo 页面：

OpenSpec Artifacts 参考:
- 提案: openspec/changes/%s/proposal.md
- 规格: openspec/changes/%s/specs/
- 设计: openspec/changes/%s/design.md

要求:
1. 生成一个纯 HTML + CSS 的 Demo 页面（不依赖框架和构建工具）
2. 将 Demo 页面保存到 frontend/demo/demo-%s.html
3. 页面应展示所有主要功能的 UI 布局和交互流程
4. 使用 inline CSS 或 <style> 标签，确保直接打开 HTML 文件即可预览
5. 不需要实现后端逻辑，用静态数据展示即可`,
		changeName, changeName, changeName, reqID)

	result, err := o.runner.Run(ctx, "frontend", prompt, runner.RunOptions{
		ReqID: reqID, Title: "Demo-REQ-" + reqID,
	})
	if err != nil {
		return err
	}
	printResult("demo", reqID, result.ExitCode, result.Elapsed)
	return nil
}

// ── 全自动流水线 ──────────────────────────────────────────────────────────────

// RunFull 运行完整的自动化流水线：
//
//	plan → develop → test → [fix loop] → pref(frontend) → pref(backend)
//
// 任意必选阶段（plan / develop / test / fix loop）失败则立即终止并返回错误。
// 可选阶段（pref）失败时仅打印 WARN 日志，不影响整体流程完成。
func (o *Orchestrator) RunFull(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("全自动流水线 REQ-%s", reqID), "RUN")

	// Phase 1: 策划
	if err := o.RunPlanning(ctx, reqID, title); err != nil {
		return fmt.Errorf("planning: %w", err)
	}

	// Phase 2: 并行开发
	if err := o.RunDevelop(ctx, reqID, title); err != nil {
		return fmt.Errorf("develop: %w", err)
	}

	// Phase 3: 测试验收
	tr, err := o.RunTest(ctx, reqID, title)
	if err != nil {
		return fmt.Errorf("test: %w", err)
	}

	// Phase 4: 若测试未通过，进入修复循环
	if !tr.Passed {
		if err := o.RunFixLoop(ctx, reqID, title, DefaultMaxFixRounds); err != nil {
			return fmt.Errorf("fix loop: %w", err)
		}
	}

	// Phase 5: 代码质量优化（可选，失败不阻断）
	if err := o.RunPref(ctx, reqID, "frontend", title); err != nil {
		log(fmt.Sprintf("前端代码优化失败（不影响主流程）: %v", err), "WARN")
	}
	if err := o.RunPref(ctx, reqID, "backend", title); err != nil {
		log(fmt.Sprintf("后端代码优化失败（不影响主流程）: %v", err), "WARN")
	}

	log("全自动流水线完成 ✅", "OK")
	printResult("run", reqID, 0, 0)
	return nil
}
