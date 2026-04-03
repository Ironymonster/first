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
)

const (
	DefaultMaxFixRounds  = 10
	jsonSummaryMarker    = "@@ORCHESTRATOR_RESULT@@"
)

// TestResult holds the outcome of a test run.
type TestResult struct {
	Passed   bool
	ExitCode int
	Elapsed  float64
}

// Orchestrator coordinates the multi-agent pipeline.
type Orchestrator struct {
	Root   string
	runner *runner.Runner
}

// New creates an Orchestrator for the project at root.
func New(root string) (*Orchestrator, error) {
	r, err := runner.New(root)
	if err != nil {
		return nil, err
	}
	return &Orchestrator{Root: root, runner: r}, nil
}

// ── Logging helpers ───────────────────────────────────────────────────────────

func log(msg, tag string) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, tag, msg)
}

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

// ── Git helpers ───────────────────────────────────────────────────────────────

// GitCommit runs git add -A && git commit with the given message.
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
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// ── Phase: Planning ───────────────────────────────────────────────────────────

// RunPlanning runs Phase 1 — OpenSpec planning via Manager Agent.
func (o *Orchestrator) RunPlanning(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("Phase 1/4: OpenSpec 策划 (REQ-%s %s)", reqID, title), "RUN")

	reqFile := filepath.Join(o.Root, "docs", "requirements", fmt.Sprintf("REQ-%s.md", reqID))
	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		return fmt.Errorf("需求文件不存在: %s", reqFile)
	}

	changeName := "req-" + reqID
	s, err := status.Read(o.Root, reqID)
	if err != nil || s == nil {
		s, err = status.CreateInitial(o.Root, reqID, changeName, title)
		if err != nil {
			return err
		}
	}
	s.Phase = "planning"
	s.ManagerStatus = "in_progress"
	s.ChangeName = changeName
	if title != "" {
		s.Title = title
	}
	s.PipelineStatus = "in_progress"
	_ = status.Write(o.Root, reqID, s)

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

	s, _ = status.Read(o.Root, reqID)
	if s == nil {
		s = &status.PipelineStatus{ReqID: reqID, ChangeName: changeName}
	}
	if result.ExitCode == 0 {
		s.ManagerStatus = "completed"
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

// ── Phase: Develop ────────────────────────────────────────────────────────────

// RunDevelop runs Phase 2 — parallel Frontend + Backend development.
func (o *Orchestrator) RunDevelop(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("Phase 2/4: 并行开发 (REQ-%s %s)", reqID, title), "RUN")

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
		changeName = "req-" + reqID
	}

	s.Phase = "development"
	s.PipelineStatus = "in_progress"
	_ = status.Write(o.Root, reqID, s)

	fePath := fmt.Sprintf("inbox/frontend/TASK-%s.md", reqID)
	bePath := fmt.Sprintf("inbox/backend/TASK-%s.md", reqID)

	fePrompt := fmt.Sprintf(`请阅读任务文件 %s 并完成所有前端开发任务。
OpenSpec change: %s`, fePath, changeName)

	bePrompt := fmt.Sprintf(`请阅读任务文件 %s 并完成所有后端开发任务。
OpenSpec change: %s`, bePath, changeName)

	var feResult, beResult *runner.Result
	startTime := time.Now()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		feResult, err = o.runner.Run(gctx, "frontend", fePrompt, runner.RunOptions{
			ReqID: reqID, Title: "前端-REQ-" + reqID,
		})
		return err
	})
	g.Go(func() error {
		var err error
		beResult, err = o.runner.Run(gctx, "backend", bePrompt, runner.RunOptions{
			ReqID: reqID, Title: "后端-REQ-" + reqID,
		})
		return err
	})

	runErr := g.Wait()
	elapsed := time.Since(startTime).Seconds()

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

// ── Phase: Test ───────────────────────────────────────────────────────────────

// RunTest runs Phase 3 — test agent acceptance.
func (o *Orchestrator) RunTest(ctx context.Context, reqID, title string) (*TestResult, error) {
	log(fmt.Sprintf("Phase 3/4: 测试验收 (REQ-%s %s)", reqID, title), "RUN")

	s, _ := status.Read(o.Root, reqID)
	changeName := "req-" + reqID
	if s != nil && s.ChangeName != "" {
		changeName = s.ChangeName
	}

	prompt := fmt.Sprintf(`请根据 OpenSpec change "%s" 的验收标准，对 REQ-%s 的实现进行测试验收。

参考文件：
- openspec/changes/%s/specs/ — 每个 spec 的接受标准
- openspec/changes/%s/tasks.md — 任务完成情况

验收完成后，在 stdout 输出:
%s {"phase":"test","req_id":"%s","passed":<true|false>,"exit_code":0}`,
		changeName, reqID, changeName, changeName,
		jsonSummaryMarker, reqID)

	startTime := time.Now()
	result, err := o.runner.Run(ctx, "test", prompt, runner.RunOptions{
		ReqID: reqID, Title: "测试-REQ-" + reqID,
	})
	elapsed := time.Since(startTime).Seconds()
	if err != nil {
		return nil, err
	}

	passed := result.ExitCode == 0 && extractPassed(result.RawOutput)

	if passed {
		log("测试通过 ✅", "OK")
	} else {
		log(fmt.Sprintf("测试失败 (exit=%d)", result.ExitCode), "WARN")
	}

	printResult("test", reqID, result.ExitCode, elapsed)
	return &TestResult{Passed: passed, ExitCode: result.ExitCode, Elapsed: elapsed}, nil
}

// extractPassed scans raw output for the @@ORCHESTRATOR_RESULT@@ marker and
// checks the "passed" field.
func extractPassed(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, jsonSummaryMarker) {
			continue
		}
		idx := strings.Index(line, jsonSummaryMarker)
		jsonStr := strings.TrimSpace(line[idx+len(jsonSummaryMarker):])
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			continue
		}
		if p, ok := m["passed"].(bool); ok {
			return p
		}
	}
	return false
}

// ── Phase: Fix ────────────────────────────────────────────────────────────────

// RunFix runs one fix round (frontend + backend in parallel).
func (o *Orchestrator) RunFix(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("修复中 (REQ-%s %s)", reqID, title), "RUN")

	s, _ := status.Read(o.Root, reqID)
	changeName := "req-" + reqID
	if s != nil && s.ChangeName != "" {
		changeName = s.ChangeName
	}

	fixFileBase := fmt.Sprintf("reports/fix-request-%s.md", reqID)
	prompt := fmt.Sprintf(`请根据修复请求文件 %s 修复 REQ-%s 中的问题。
OpenSpec change: %s`, fixFileBase, reqID, changeName)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err := o.runner.Run(gctx, "frontend", "前端: "+prompt, runner.RunOptions{
			ReqID: reqID, Title: "修复FE-REQ-" + reqID,
		})
		return err
	})
	g.Go(func() error {
		_, err := o.runner.Run(gctx, "backend", "后端: "+prompt, runner.RunOptions{
			ReqID: reqID, Title: "修复BE-REQ-" + reqID,
		})
		return err
	})
	return g.Wait()
}

// RunFixLoop runs fix → test → repeat until passing or maxRounds exceeded.
func (o *Orchestrator) RunFixLoop(ctx context.Context, reqID, title string, maxRounds int) error {
	if maxRounds <= 0 {
		maxRounds = DefaultMaxFixRounds
	}
	for round := 1; round <= maxRounds; round++ {
		log(fmt.Sprintf("Fix loop 第 %d/%d 轮", round, maxRounds), "RUN")

		if err := o.RunFix(ctx, reqID, title); err != nil {
			return fmt.Errorf("fix round %d: %w", round, err)
		}

		tr, err := o.RunTest(ctx, reqID, title)
		if err != nil {
			return err
		}
		if tr.Passed {
			log(fmt.Sprintf("✅ 测试通过（第 %d 轮修复后）", round), "OK")
			printResult("fix", reqID, 0, 0)
			return nil
		}
	}
	printResult("fix", reqID, 1, 0)
	return fmt.Errorf("超过最大修复轮数 (%d)，测试仍未通过", maxRounds)
}

// ── Phase: Pref ───────────────────────────────────────────────────────────────

// RunPref runs code quality optimization for the given target (frontend|backend).
func (o *Orchestrator) RunPref(ctx context.Context, reqID, target, title string) error {
	log(fmt.Sprintf("代码质量优化: %s (REQ-%s)", target, reqID), "RUN")

	prompt := fmt.Sprintf(`请对 REQ-%s 的 %s 代码进行质量优化。
参考 prompts/pref.md 中的优化指引。`, reqID, target)

	result, err := o.runner.Run(ctx, target, prompt, runner.RunOptions{
		ReqID: reqID, Title: "Pref-" + target + "-REQ-" + reqID,
	})
	if err != nil {
		return err
	}
	printResult("pref", reqID, result.ExitCode, result.Elapsed)
	return nil
}

// ── Phase: Bugfix ─────────────────────────────────────────────────────────────

// RunBugfix runs a targeted bug fix for the given agent role.
func (o *Orchestrator) RunBugfix(ctx context.Context, agentRole, description string) error {
	log(fmt.Sprintf("Bug 修复: %s — %s", agentRole, description), "RUN")

	prompt := fmt.Sprintf(`请修复以下 bug：%s`, description)
	result, err := o.runner.Run(ctx, agentRole, prompt, runner.RunOptions{
		Title: "Bugfix-" + agentRole,
	})
	if err != nil {
		return err
	}
	printResult("bugfix", "", result.ExitCode, result.Elapsed)
	return nil
}

// ── Phase: Demo ───────────────────────────────────────────────────────────────

// RunDemo generates a frontend HTML demo page.
func (o *Orchestrator) RunDemo(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("前端 HTML Demo (REQ-%s)", reqID), "RUN")

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

// ── Phase: Full pipeline ──────────────────────────────────────────────────────

// RunFull runs the full autonomous pipeline:
// plan → develop → test → fix loop → pref.
func (o *Orchestrator) RunFull(ctx context.Context, reqID, title string) error {
	log(fmt.Sprintf("全自动流水线 REQ-%s", reqID), "RUN")

	if err := o.RunPlanning(ctx, reqID, title); err != nil {
		return fmt.Errorf("planning: %w", err)
	}
	if err := o.RunDevelop(ctx, reqID, title); err != nil {
		return fmt.Errorf("develop: %w", err)
	}

	tr, err := o.RunTest(ctx, reqID, title)
	if err != nil {
		return fmt.Errorf("test: %w", err)
	}
	if !tr.Passed {
		if err := o.RunFixLoop(ctx, reqID, title, DefaultMaxFixRounds); err != nil {
			return fmt.Errorf("fix loop: %w", err)
		}
	}

	_ = o.RunPref(ctx, reqID, "frontend", title)
	_ = o.RunPref(ctx, reqID, "backend", title)

	log("全自动流水线完成 ✅", "OK")
	printResult("run", reqID, 0, 0)
	return nil
}
