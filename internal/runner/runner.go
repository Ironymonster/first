// Package runner 负责驱动 claude CLI 子进程执行各 Agent 技能。
//
// 核心流程：
//  1. 从 Loader 获取技能定义（model、agent.md 路径）
//  2. 使用 exec.CommandContext 启动 claude CLI（stream-json 模式）
//  3. 通过 parseStream 实时解析 JSON 事件流，打印工具调用日志
//  4. 等待进程退出，返回 Result（包含退出码、文本输出、token 用量等）
//
// 并发安全：Runner.mu 互斥锁保护多个 Agent 并行运行时的 stdout 输出不交叉。
package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ironymonster/chainAgent/internal/skill"
	"github.com/Ironymonster/chainAgent/internal/status"
)

// ── ANSI 颜色 ─────────────────────────────────────────────────────────────────

// agentColors 为每种 Agent 角色分配不同的终端颜色，便于区分并行输出。
var agentColors = map[string]string{
	"manager":  "\033[36m", // 青色
	"spec":     "\033[34m", // 蓝色
	"frontend": "\033[33m", // 黄色
	"backend":  "\033[35m", // 洋红色
	"test":     "\033[32m", // 绿色
}

const colorReset = "\033[0m" // 重置颜色
const colorDim = "\033[2m"   // 暗色（用于心跳提示）
const colorBold = "\033[1m"  // 粗体（保留，暂未使用）

// useColor 检测 stdout 是否为终端，非终端环境（如 CI 日志）自动禁用颜色。
func useColor() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// colorFor 返回指定角色对应的 ANSI 颜色前缀。非终端环境返回空字符串。
func colorFor(role string) string {
	if !useColor() {
		return ""
	}
	if c, ok := agentColors[role]; ok {
		return c
	}
	return "\033[37m" // 默认白色（未知角色）
}

// reset 返回 ANSI 颜色重置序列。非终端环境返回空字符串。
func reset() string {
	if !useColor() {
		return ""
	}
	return colorReset
}

// ── 工具图标 ──────────────────────────────────────────────────────────────────

// toolIcons 将 claude CLI 工具名映射到对应的 emoji 图标，用于终端日志展示。
var toolIcons = map[string]string{
	"bash":                "⚡",
	"edit":                "📝",
	"write":               "📝",
	"read":                "📖",
	"grep":                "🔍",
	"glob":                "🔍",
	"lsp_diagnostics":     "🔬",
	"lsp_goto_definition": "🔬",
	"lsp_find_references": "🔬",
	"lsp_symbols":         "🔬",
	"lsp_rename":          "🔬",
	"ast_grep_search":     "🌳",
	"ast_grep_replace":    "🌳",
	"webfetch":            "🌐",
	"question":            "❓",
	"todowrite":           "📋",
	"todoread":            "📋",
	"task":                "🚀",
}

// iconFor 返回工具名对应的图标，未知工具返回通用扳手图标。
func iconFor(tool string) string {
	if ic, ok := toolIcons[tool]; ok {
		return ic
	}
	return "🔧" // 通用工具图标
}

// ── 结果结构体 ────────────────────────────────────────────────────────────────

// Usage 保存 claude CLI result 事件中的 token 用量和费用统计。
type Usage struct {
	InputTokens  int     `json:"input_tokens"`  // 输入 token 数
	OutputTokens int     `json:"output_tokens"` // 输出 token 数
	CostUSD      float64 `json:"cost_usd"`      // 本次调用费用（美元）
}

// Result 是 Runner.Run 在 Agent 进程退出后返回的完整执行结果。
type Result struct {
	ExitCode   int     // Agent 进程退出码，0 表示成功
	TextOutput string  // Agent 输出的所有文本块（assistant text 事件汇总）
	Elapsed    float64 // 总耗时（秒）
	Usage      Usage   // token 用量统计
	RawOutput  string  // stdout 原始输出（用于解析 @@ORCHESTRATOR_RESULT@@ 标记）
}

// ── 执行选项 ──────────────────────────────────────────────────────────────────

// RunOptions 配置单次 Agent 执行的参数。
type RunOptions struct {
	ReqID    string        // 关联的需求 ID（用于写入实时状态文件）
	Title    string        // 任务标题（用于日志展示）
	TaskType string        // 任务类型："req" | "bugfix" | "pref" 等（供扩展使用）
	Timeout  time.Duration // 超时时长，0 表示使用默认值（10 分钟）
	// WorkDir 指定 claude 子进程的工作目录。
	// 设置为 git worktree 路径时，Agent 在隔离目录中运行；留空则使用项目根目录。
	WorkDir string
}

// timeout 返回实际使用的超时时长，若未设置则使用 10 分钟默认值。
func (o RunOptions) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return 10 * time.Minute
}

// ── Runner ────────────────────────────────────────────────────────────────────

// Runner 通过 claude CLI 子进程执行技能。
// 并发安全：多个 Agent 并行运行时，mu 互斥锁保护 stdout 输出不交叉混乱。
type Runner struct {
	Root      string        // 项目根目录（skills/ 所在位置）
	SkillsDir string        // skills 目录的绝对路径，通常为 Root/skills
	LogDir    string        // 日志目录的绝对路径，通常为 Root/.chainagent/logs
	loader    *skill.Loader // 技能加载器
	mu        sync.Mutex    // 保护并发 Agent 向 stdout 输出时不产生交叉
}

// New 创建一个针对指定项目根目录的 Runner，并立即加载所有技能定义。
// 若 skills/ 目录不存在或任意技能加载失败，返回错误。
func New(root string) (*Runner, error) {
	skillsDir := filepath.Join(root, "skills")
	logDir := filepath.Join(root, ".chainagent", "logs")
	// 确保日志目录存在，避免后续创建日志文件时失败。
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	loader := skill.NewLoader(skillsDir)
	if _, err := loader.LoadAll(); err != nil {
		return nil, fmt.Errorf("加载技能失败: %w", err)
	}

	return &Runner{
		Root:      root,
		SkillsDir: skillsDir,
		LogDir:    logDir,
		loader:    loader,
	}, nil
}

// GetModel 返回指定角色技能所配置的模型名称。
// 若角色不存在则返回空字符串（调用方可按需处理）。
func (r *Runner) GetModel(role string) string {
	def, err := r.loader.Get(role)
	if err != nil {
		return ""
	}
	return def.Model
}

// Run 启动指定角色的 claude CLI 子进程，流式输出执行过程，并在进程退出后返回结果。
//
// 执行流程：
//  1. 从 Loader 获取技能定义（model、agent.md 路径）
//  2. 构建 claude CLI 命令（--system-prompt-file、--output-format stream-json 等）
//  3. 启动三个 goroutine：心跳检测、stderr 收集、stdout 流式解析
//  4. 等待 stdout/stderr 读取完成，调用 cmd.Wait() 获取退出码
//  5. 返回 Result
func (r *Runner) Run(ctx context.Context, role, prompt string, opts RunOptions) (*Result, error) {
	def, err := r.loader.Get(role)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout())
	defer cancel()

	// ── Build command ──────────────────────────────────────────────────────
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
	}

	args := []string{
		"-p", prompt,
		"--system-prompt-file", def.AgentFile,
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	}
	if def.Model != "" {
		args = append(args, "--model", def.Model)
	}

	cmd := exec.CommandContext(ctx, claudePath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	} else {
		cmd.Dir = r.Root
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// ── Log file ───────────────────────────────────────────────────────────
	ts := time.Now().Format("20060102_150405")
	logPath := filepath.Join(r.LogDir, fmt.Sprintf("%s_%s.log", role, ts))
	logFile, err := os.Create(logPath)
	if err != nil {
		logFile = nil // non-fatal
	} else {
		defer logFile.Close()
		fmt.Fprintf(logFile, "=== agent: %s | model: %s | %s ===\nPROMPT:\n%s\n---\n",
			role, def.Model, ts, prompt)
	}

	// ── Print header ───────────────────────────────────────────────────────
	col := colorFor(role)
	rst := reset()
	r.mu.Lock()
	fmt.Printf("\n%s%s\n  %s started | model: %s\n%s%s\n\n",
		col, strings.Repeat("─", 55),
		role, def.Model,
		strings.Repeat("─", 55), rst)
	r.mu.Unlock()

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	// ── Shared state for goroutines ────────────────────────────────────────
	var (
		rawBuf      strings.Builder
		textBuf     strings.Builder
		usageHolder Usage
		stepCount   atomic.Int64
		lastAct     atomic.Int64 // unix nano
	)
	lastAct.Store(time.Now().UnixNano())

	// ── 心跳检测 goroutine ────────────────────────────────────────────────
	// 每 5 秒检查一次，若 30 秒内没有新的工具调用事件，打印"仍在运行"提示。
	// 防止长时间无输出时用户误以为进程已卡死。
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return // context 已取消（超时或外部 cancel），退出心跳
			case <-ticker.C:
				last := time.Unix(0, lastAct.Load())
				idle := time.Since(last)
				if idle >= 30*time.Second {
					elapsed := time.Since(startTime).Round(time.Second)
					r.mu.Lock()
					fmt.Printf("%s⏳ %s 仍在运行... (elapsed: %s)%s\n",
						colorDim, role, elapsed, colorReset)
					r.mu.Unlock()
				}
			}
		}
	}()

	// ── stderr 收集 goroutine ─────────────────────────────────────────────
	// 将 claude CLI 的 stderr 输出写入日志文件（不打印到终端，避免干扰正常输出）。
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			if logFile != nil {
				fmt.Fprintln(logFile, "[stderr] "+line)
			}
		}
	}()

	// ── stdout 流式解析 goroutine ─────────────────────────────────────────
	// parseStream 实时解析 claude CLI 的 stream-json 格式输出，
	// 打印工具调用日志并收集文本输出和 token 用量。
	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		parseStream(ctx, stdout, role, &rawBuf, &textBuf, &usageHolder,
			&stepCount, &lastAct, logFile,
			opts.ReqID, r.Root, &r.mu)
	}()

	// ── 等待所有 goroutine 完成 ───────────────────────────────────────────
	// 必须先等 stdout/stderr 读完，再 cancel()，最后等心跳退出。
	// 顺序不能颠倒，否则可能丢失最后的输出行。
	<-stdoutDone
	<-stderrDone
	cancel() // 通知心跳 goroutine 退出
	<-heartbeatDone

	exitErr := cmd.Wait()
	exitCode := 0
	if exitErr != nil {
		if ee, ok := exitErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}

	elapsed := time.Since(startTime).Seconds()

	// ── Footer ─────────────────────────────────────────────────────────────
	r.mu.Lock()
	fmt.Printf("%s%s\n  %s done | exit: %d | %.1fs | tokens: %d in / %d out\n%s%s\n\n",
		col, strings.Repeat("─", 55),
		role, exitCode, elapsed,
		usageHolder.InputTokens, usageHolder.OutputTokens,
		strings.Repeat("─", 55), rst)
	r.mu.Unlock()

	return &Result{
		ExitCode:   exitCode,
		TextOutput: textBuf.String(),
		Elapsed:    elapsed,
		Usage:      usageHolder,
		RawOutput:  rawBuf.String(),
	}, nil
}

// ── stream-json 解析器 ────────────────────────────────────────────────────────

// maxCmdDisplay 是命令详情在终端日志中的最大显示字符数，超出部分截断为 "..."
const maxCmdDisplay = 100

// maxTextDisplay 是 Agent 文本输出在终端日志中的最大显示字符数。
const maxTextDisplay = 200

// parseStream 逐行解析 claude CLI 的 stream-json 格式输出。
//
// claude CLI 以 --output-format stream-json 模式运行时，每行输出一个 JSON 事件，
// 事件类型（type 字段）包括：
//   - "assistant"：Agent 的回复，包含 text 和 tool_use 两种 content block
//   - "result"：会话结束事件，包含 token 用量和费用统计
//
// 本函数：
//  1. 将每行原始输出追加到 rawBuf（供调用方解析 @@ORCHESTRATOR_RESULT@@ 标记）
//  2. 解析 tool_use 事件，打印工具调用日志并更新实时状态文件
//  3. 解析 text 事件，打印 Agent 文本输出摘要
//  4. 解析 result 事件，更新 token 用量统计
//  5. 在每行处理前检查 ctx.Done()，支持 context 取消快速退出
func parseStream(
	ctx context.Context,
	r io.Reader,
	role string,
	rawBuf, textBuf *strings.Builder,
	usage *Usage,
	stepCount *atomic.Int64,
	lastAct *atomic.Int64,
	logFile *os.File,
	reqID, root string,
	mu *sync.Mutex,
) {
	col := colorFor(role)
	rst := reset()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)

	liveState := &status.LiveState{Model: ""}
	step := 0

	for sc.Scan() {
		// Respect context cancellation: if the parent context is already done,
		// stop processing further output even if the pipe is still open.
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := sc.Text()
		rawBuf.WriteString(line + "\n")
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		lastAct.Store(time.Now().UnixNano())

		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		evtType, _ := evt["type"].(string)

		switch evtType {
		case "assistant":
			msg, _ := evt["message"].(map[string]any)
			if msg == nil {
				continue
			}
			contentArr, _ := msg["content"].([]any)
			for _, item := range contentArr {
				block, _ := item.(map[string]any)
				if block == nil {
					continue
				}
				btype, _ := block["type"].(string)
				switch btype {
				case "text":
					text, _ := block["text"].(string)
					if text == "" {
						continue
					}
					textBuf.WriteString(text)
					summary := truncate(text, maxTextDisplay)
					mu.Lock()
					fmt.Printf("%s💬 %s: %s%s\n", col, role, summary, rst)
					mu.Unlock()

				case "tool_use":
					toolName, _ := block["name"].(string)
					input, _ := block["input"].(map[string]any)
					icon := iconFor(toolName)
					detail := formatToolDetail(toolName, input)

					step++
					stepCount.Store(int64(step))
					liveState.CurrentTool = toolName
					liveState.StepCount = step

					mu.Lock()
					fmt.Printf("%s%s %s: %s%s\n", col, icon, toolName, detail, rst)
					mu.Unlock()

					if reqID != "" {
						_ = status.WriteLive(root, reqID, role, liveState)
					}
				}
			}

		case "result":
			// Extract usage from result event.
			usageRaw, _ := evt["usage"].(map[string]any)
			if usageRaw != nil {
				usage.InputTokens = toInt(usageRaw["input_tokens"])
				usage.OutputTokens = toInt(usageRaw["output_tokens"])
			}
			if cost, ok := evt["cost_usd"].(float64); ok {
				usage.CostUSD = cost
			}
		}
	}
}

// formatToolDetail 从工具调用的 input 参数中提取可读的摘要字符串，用于终端日志展示。
// 针对常见工具（bash、read、write、grep 等）做了专门处理，提取最关键的参数。
// 未识别的工具取 input 中的第一个字符串值作为摘要。
func formatToolDetail(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}
	switch toolName {
	case "bash":
		cmd, _ := input["command"].(string)
		return truncate(strings.ReplaceAll(cmd, "\n", " "), maxCmdDisplay)
	case "read":
		path, _ := input["file_path"].(string)
		return path
	case "write", "edit":
		path, _ := input["file_path"].(string)
		return path
	case "grep":
		pattern, _ := input["pattern"].(string)
		path, _ := input["path"].(string)
		return fmt.Sprintf("%q in %s", pattern, path)
	case "glob":
		pattern, _ := input["pattern"].(string)
		return pattern
	case "webfetch":
		url, _ := input["url"].(string)
		return truncate(url, 80)
	case "task":
		subtype, _ := input["subagent_type"].(string)
		return "subagent_type=" + subtype
	}
	// Generic: show first string value.
	for _, v := range input {
		if s, ok := v.(string); ok && s != "" {
			return truncate(s, maxCmdDisplay)
		}
	}
	return ""
}

// truncate 将字符串截断到指定最大长度，并将换行符替换为空格，便于单行展示。
// 超出部分以 "..." 结尾。
func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// toInt 将 JSON 数值（float64）或 int 安全转换为 int。
// json.Unmarshal 默认将数字解析为 float64，需要显式转换。
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
