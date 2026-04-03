package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Ironymonster/chainAgent/internal/orchestrator"
	"github.com/Ironymonster/chainAgent/internal/status"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// projectRoot returns the directory containing skills/ by walking up from the
// current working directory. Falls back to cwd if not found.
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cwd, _ := os.Getwd()
	return cwd
}

func newOrchestrator() (*orchestrator.Orchestrator, error) {
	root := projectRoot()
	o, err := orchestrator.New(root)
	if err != nil {
		return nil, fmt.Errorf("initializing orchestrator: %w\n\nMake sure you are running chainagent from a project directory that contains skills/", err)
	}
	return o, nil
}

// ── Root command ──────────────────────────────────────────────────────────────

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "chainagent",
		Short: "ChainAgent — 多 Agent 编排框架（Go 版）",
		Long: `ChainAgent 通过 claude CLI 驱动多个专项 Agent 并行工作，
自动完成从需求分析、架构设计到代码实现、测试验收的完整开发流程。

项目主页: https://github.com/Ironymonster/chainAgent`,
	}

	root.AddCommand(
		developCmd(),
		testCmd(),
		fixCmd(),
		planCmd(),
		runCmd(),
		demoCmd(),
		prefCmd(),
		bugfixCmd(),
		statusCmd(),
	)

	return root
}

// ── develop ───────────────────────────────────────────────────────────────────

func developCmd() *cobra.Command {
	var reqID, title string
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "develop",
		Short: "并行启动前端 + 后端开发",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunDevelop(ctx, reqID, title); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("feat: REQ-%s 开发完成", reqID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选，如 001）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── test ──────────────────────────────────────────────────────────────────────

func testCmd() *cobra.Command {
	var reqID, title string
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "test",
		Short: "启动 Test Agent 验收测试",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			tr, err := o.RunTest(ctx, reqID, title)
			if err != nil {
				return err
			}
			if !tr.Passed {
				return fmt.Errorf("测试未通过")
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("test: REQ-%s 验收通过", reqID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "通过后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── fix ───────────────────────────────────────────────────────────────────────

func fixCmd() *cobra.Command {
	var reqID, title string
	var maxRounds int
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "启动自动修复循环（fix → test → 重复）",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunFixLoop(ctx, reqID, title, maxRounds); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("fix: REQ-%s 修复完成", reqID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().IntVar(&maxRounds, "max-rounds", orchestrator.DefaultMaxFixRounds, "最大修复轮数")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── plan ──────────────────────────────────────────────────────────────────────

func planCmd() *cobra.Command {
	var reqID, title string
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "启动 OpenSpec 策划（Manager → Spec Agent）",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunPlanning(ctx, reqID, title); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("plan: REQ-%s 策划完成", reqID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── run ───────────────────────────────────────────────────────────────────────

func runCmd() *cobra.Command {
	var reqID, title string
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "全自动流水线（plan → develop → test → fix → pref）",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunFull(ctx, reqID, title); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("feat: REQ-%s 全流程完成", reqID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── demo ──────────────────────────────────────────────────────────────────────

func demoCmd() *cobra.Command {
	var reqID, title string

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "生成前端 HTML Demo 页面",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			return o.RunDemo(context.Background(), reqID, title)
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

// ── pref ──────────────────────────────────────────────────────────────────────

func prefCmd() *cobra.Command {
	var reqID, target, title string

	cmd := &cobra.Command{
		Use:   "pref",
		Short: "代码质量优化",
		RunE: func(cmd *cobra.Command, args []string) error {
			if target != "frontend" && target != "backend" {
				return fmt.Errorf("--target 必须是 frontend 或 backend，当前: %q", target)
			}
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			return o.RunPref(context.Background(), reqID, target, title)
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&target, "target", "", "优化目标：frontend 或 backend（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	_ = cmd.MarkFlagRequired("req")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// ── bugfix ────────────────────────────────────────────────────────────────────

func bugfixCmd() *cobra.Command {
	var agentRole, description string

	cmd := &cobra.Command{
		Use:   "bugfix",
		Short: "针对性 Bug 修复",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			return o.RunBugfix(context.Background(), agentRole, description)
		},
	}
	cmd.Flags().StringVar(&agentRole, "agent", "", "Agent 角色：frontend 或 backend（必选）")
	cmd.Flags().StringVar(&description, "description", "", "Bug 描述（必选）")
	_ = cmd.MarkFlagRequired("agent")
	_ = cmd.MarkFlagRequired("description")
	return cmd
}

// ── status ────────────────────────────────────────────────────────────────────

func statusCmd() *cobra.Command {
	var reqID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "查看流水线进度",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := projectRoot()

			if reqID != "" {
				s, err := status.Read(root, reqID)
				if err != nil {
					return err
				}
				if s == nil {
					fmt.Printf("REQ-%s: 未找到状态文件\n", reqID)
					return nil
				}
				printStatus(s)
				return nil
			}

			// List all.
			all, err := status.ListAll(root)
			if err != nil {
				return err
			}
			if len(all) == 0 {
				fmt.Println("暂无流水线状态记录")
				return nil
			}
			for _, s := range all {
				printStatus(s)
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（不填则列出所有）")
	return cmd
}

func printStatus(s *status.PipelineStatus) {
	pipeIcon := "🔄"
	switch s.PipelineStatus {
	case "completed":
		pipeIcon = "✅"
	case "failed":
		pipeIcon = "❌"
	}
	fmt.Printf("REQ-%s %s\n", s.ReqID, s.Title)
	fmt.Printf("  change:   %s\n", s.ChangeName)
	fmt.Printf("  phase:    %s\n", s.Phase)
	fmt.Printf("  pipeline: %s %s\n", pipeIcon, s.PipelineStatus)
	fmt.Printf("  manager:  %s\n", s.ManagerStatus)
	fmt.Printf("  updated:  %s\n", s.UpdatedAt)
}
