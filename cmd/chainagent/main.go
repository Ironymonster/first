// Package main 是 chainagent 命令行工具的入口。
//
// chainagent 通过 cobra 提供以下子命令：
//
//	plan     — 启动 OpenSpec 策划（Phase 1）
//	develop  — 并行启动前端 + 后端开发（Phase 2）
//	test     — 启动 Test Agent 验收测试（Phase 3）
//	fix      — 启动自动修复循环（Phase 4）
//	pref     — 代码质量优化（Phase 5）
//	run      — 一键运行全自动流水线（Phase 1-5）
//	demo     — 生成前端 HTML Demo 页面
//	bugfix   — 针对性 Bug 修复（B 流）
//	status   — 查看流水线进度
//	worktree — 管理 git worktree 隔离工作区
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

// projectRoot 从当前工作目录向上逐级查找包含 skills/ 目录的路径，作为项目根目录。
// 若一直找到文件系统根目录仍未找到，则回退使用当前工作目录。
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
		worktreeCmd(),
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
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "生成前端 HTML Demo 页面",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunDemo(ctx, reqID, title); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("demo: REQ-%s Demo 页面生成", reqID))
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

// ── pref ──────────────────────────────────────────────────────────────────────

func prefCmd() *cobra.Command {
	var reqID, target, title string
	var gitCommit bool

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
			ctx := context.Background()
			if err := o.RunPref(ctx, reqID, target, title); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("pref: REQ-%s %s 代码优化完成", reqID, target))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "req", "", "需求 ID（必选）")
	cmd.Flags().StringVar(&target, "target", "", "优化目标：frontend 或 backend（必选）")
	cmd.Flags().StringVar(&title, "title", "", "需求标题（可选）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
	_ = cmd.MarkFlagRequired("req")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// ── bugfix ────────────────────────────────────────────────────────────────────

func bugfixCmd() *cobra.Command {
	var agentRole, description, worktreeName string
	var gitCommit bool

	cmd := &cobra.Command{
		Use:   "bugfix",
		Short: "针对性 Bug 修复",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if err := o.RunBugfix(ctx, agentRole, description, worktreeName); err != nil {
				return err
			}
			if gitCommit {
				return o.GitCommit(fmt.Sprintf("fix(%s): %s", agentRole, description))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentRole, "agent", "", "Agent 角色：frontend 或 backend（必选）")
	cmd.Flags().StringVar(&description, "description", "", "Bug 描述（必选）")
	cmd.Flags().StringVar(&worktreeName, "worktree", "", "Worktree 名称，如 fix-bug-001（可选，默认在项目根目录运行）")
	cmd.Flags().BoolVar(&gitCommit, "git-commit", false, "完成后自动 git commit")
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

// ── worktree ──────────────────────────────────────────────────────────────────

func worktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "管理 git worktree 隔离工作区",
	}
	cmd.AddCommand(worktreeSetupCmd(), worktreeRemoveCmd(), worktreeListCmd())
	return cmd
}

func worktreeSetupCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "为任务创建隔离 worktree（如 req-001、fix-login）",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			path, err := o.SetupWorktree(name)
			if err != nil {
				return err
			}
			fmt.Printf("✅ worktree 已就绪: %s\n", path)
			fmt.Printf("   切换目录后可直接运行开发命令，无需额外参数\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "任务名称，如 req-001 或 fix-login（必选）")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func worktreeRemoveCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "删除任务 worktree（MR 合并后清理）",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			if err := o.RemoveWorktree(name); err != nil {
				return err
			}
			fmt.Printf("✅ worktree 已删除: %s\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "任务名称（必选）")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func worktreeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有活跃的 worktree",
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := newOrchestrator()
			if err != nil {
				return err
			}
			names, err := o.ListWorktrees()
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Println("暂无活跃的 worktree")
				return nil
			}
			fmt.Println("活跃的 worktree：")
			for _, n := range names {
				fmt.Printf("  • %s  →  .worktrees/%s\n", n, n)
			}
			return nil
		},
	}
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
