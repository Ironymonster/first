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

	"github.com/chainagent-oss/chainagent/internal/skill"
	"github.com/chainagent-oss/chainagent/internal/status"
)

// ── ANSI colors ───────────────────────────────────────────────────────────────

var agentColors = map[string]string{
	"manager":  "\033[36m", // Cyan
	"spec":     "\033[34m", // Blue
	"frontend": "\033[33m", // Yellow
	"backend":  "\033[35m", // Magenta
	"test":     "\033[32m", // Green
}

const colorReset = "\033[0m"
const colorDim = "\033[2m"
const colorBold = "\033[1m"

func useColor() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func colorFor(role string) string {
	if !useColor() {
		return ""
	}
	if c, ok := agentColors[role]; ok {
		return c
	}
	return "\033[37m" // White fallback
}

func reset() string {
	if !useColor() {
		return ""
	}
	return colorReset
}

// ── Tool icons ────────────────────────────────────────────────────────────────

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

func iconFor(tool string) string {
	if ic, ok := toolIcons[tool]; ok {
		return ic
	}
	return "🔧"
}

// ── Result ────────────────────────────────────────────────────────────────────

// Usage holds token/cost statistics from the result event.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Result is returned by Runner.Run after the agent process exits.
type Result struct {
	ExitCode    int
	TextOutput  string  // accumulated assistant text blocks
	Elapsed     float64 // seconds
	Usage       Usage
	RawOutput   string // full stdout (for @@ORCHESTRATOR_RESULT@@ parsing)
}

// ── RunOptions ────────────────────────────────────────────────────────────────

// RunOptions configures a single agent execution.
type RunOptions struct {
	ReqID    string
	Title    string
	TaskType string // "req" | "bugfix" | "pref" etc.
	Timeout  time.Duration
}

func (o RunOptions) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return 10 * time.Minute
}

// ── Runner ────────────────────────────────────────────────────────────────────

// Runner executes skills via the claude CLI subprocess.
type Runner struct {
	Root      string        // project root (where skills/ lives)
	SkillsDir string        // typically Root/skills
	LogDir    string        // typically Root/.chainagent/logs
	loader    *skill.Loader
	mu        sync.Mutex    // guards stdout when parallel agents run
}

// New creates a Runner. Call loader.LoadAll() before first use.
func New(root string) (*Runner, error) {
	skillsDir := filepath.Join(root, "skills")
	logDir := filepath.Join(root, ".chainagent", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	loader := skill.NewLoader(skillsDir)
	if _, err := loader.LoadAll(); err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}

	return &Runner{
		Root:      root,
		SkillsDir: skillsDir,
		LogDir:    logDir,
		loader:    loader,
	}, nil
}

// GetModel returns the model configured for a skill (empty string if unknown).
func (r *Runner) GetModel(role string) string {
	def, err := r.loader.Get(role)
	if err != nil {
		return ""
	}
	return def.Model
}

// Run executes the named skill with the given prompt and streams output to
// stdout. It returns a Result after the subprocess exits.
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
	cmd.Dir = r.Root

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
		rawBuf     strings.Builder
		textBuf    strings.Builder
		usageHolder Usage
		stepCount  atomic.Int64
		lastAct    atomic.Int64 // unix nano
	)
	lastAct.Store(time.Now().UnixNano())

	// ── Heartbeat goroutine ────────────────────────────────────────────────
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
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

	// ── stderr goroutine ───────────────────────────────────────────────────
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

	// ── stdout / stream-json goroutine ─────────────────────────────────────
	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		parseStream(ctx, stdout, role, &rawBuf, &textBuf, &usageHolder,
			&stepCount, &lastAct, logFile,
			opts.ReqID, r.Root, &r.mu)
	}()

	// ── Wait ───────────────────────────────────────────────────────────────
	<-stdoutDone
	<-stderrDone
	cancel() // stop heartbeat ticker
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

// ── stream-json parser ────────────────────────────────────────────────────────

const maxCmdDisplay  = 100
const maxTextDisplay = 200

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

// formatToolDetail extracts a human-readable summary from tool input.
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

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
