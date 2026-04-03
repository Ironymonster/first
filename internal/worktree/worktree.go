// Package worktree 负责管理 git worktree 隔离工作区。
//
// 设计原则：每个需求（REQ）或 Bug 修复任务都在独立的 git worktree 中运行，
// 目录位于 .worktrees/<任务名>/，并检出独立分支，从而允许多个任务并行执行
// 而互不干扰，也不会污染主工作目录的状态。
package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// worktreeDir 是所有任务 worktree 存放的子目录名。
const worktreeDir = ".worktrees"

// Worktree 表示一个为特定任务创建的 git 隔离工作区。
type Worktree struct {
	Name        string // 任务短标识，如 "req-001" 或 "fix-bug-001"
	Branch      string // 该 worktree 检出的 git 分支名
	Path        string // worktree 目录的绝对路径
	ProjectRoot string // 主仓库的根目录路径
}

// Setup 创建或复用指定名称的 worktree。
//
// 若目录已存在（例如上次运行中断），则直接复用，不重新创建，
// 从而保留已有的开发进度，支持断点续跑。
//
// 创建完成后，会将 skills/ 和 prompts/ 从项目根目录同步到 worktree，
// 确保在 worktree 内运行的 Agent 能找到技能文件和提示词。
func Setup(projectRoot, name, branch string) (*Worktree, error) {
	wtPath := filepath.Join(projectRoot, worktreeDir, name)

	wt := &Worktree{
		Name:        name,
		Branch:      branch,
		Path:        wtPath,
		ProjectRoot: projectRoot,
	}

	if _, err := os.Stat(wtPath); err == nil {
		// worktree 目录已存在，直接复用并同步配置文件。
		fmt.Printf("[worktree] 复用已有 worktree: %s\n", wtPath)
		if err := syncDirs(projectRoot, wtPath); err != nil {
			return nil, err
		}
		return wt, nil
	}

	// 在新分支上创建 worktree，基于当前 HEAD。
	fmt.Printf("[worktree] 创建 worktree %s，分支: %s\n", wtPath, branch)
	if err := git(projectRoot, "worktree", "add", "-b", branch, wtPath); err != nil {
		// 分支可能已存在（如上次崩溃后未清理），尝试不带 -b 直接检出。
		if err2 := git(projectRoot, "worktree", "add", wtPath, branch); err2 != nil {
			return nil, fmt.Errorf("git worktree add 失败: %w", err)
		}
	}

	// 同步 skills/ 和 prompts/ 到新 worktree。
	if err := syncDirs(projectRoot, wtPath); err != nil {
		return nil, err
	}

	return wt, nil
}

// Remove 删除指定名称的 worktree 目录并清理 git 内部注册表。
// 即使 worktree 不存在也安全调用（幂等操作）。
func Remove(projectRoot, name string) error {
	wtPath := filepath.Join(projectRoot, worktreeDir, name)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil // 已不存在，无需操作
	}
	fmt.Printf("[worktree] 删除 worktree: %s\n", wtPath)
	if err := git(projectRoot, "worktree", "remove", "--force", wtPath); err != nil {
		// git worktree remove 失败时，退而手动删除目录。
		if err2 := os.RemoveAll(wtPath); err2 != nil {
			return fmt.Errorf("删除 worktree 目录失败: %w", err2)
		}
	}
	// 清理 .git/worktrees/ 中的过期条目，保持 git 注册表整洁。
	_ = git(projectRoot, "worktree", "prune")
	return nil
}

// List 返回当前所有注册 worktree 的短名称列表（不含主 worktree）。
//
// 优先通过 `git worktree list --porcelain` 获取权威列表，确保与 git
// 内部注册表保持一致（即使目录被手动删除或 git 异常中断也能正确反映）。
// 若 git 命令失败，则降级为扫描 .worktrees/ 文件系统目录。
func List(projectRoot string) ([]string, error) {
	names, err := listFromGit(projectRoot)
	if err == nil {
		return names, nil
	}
	// 降级：直接扫描文件系统目录。
	return listFromFS(projectRoot)
}

// listFromGit 解析 `git worktree list --porcelain` 的输出，
// 返回路径位于 .worktrees/ 目录下的所有 worktree 的短名称。
//
// porcelain 输出格式（每个 worktree 之间以空行分隔）：
//
//	worktree /absolute/path
//	HEAD abc123...
//	branch refs/heads/feat/req-001
func listFromGit(projectRoot string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git worktree list 失败: %w", err)
	}

	base := filepath.Join(projectRoot, worktreeDir)
	var names []string
	// 每个 worktree 块之间以 "\n\n" 分隔。
	for _, block := range strings.Split(string(out), "\n\n") {
		for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
			if !strings.HasPrefix(line, "worktree ") {
				continue
			}
			wtPath := strings.TrimPrefix(line, "worktree ")

			// 跳过主 worktree（路径与项目根目录相同）。
			if filepath.Clean(wtPath) == filepath.Clean(projectRoot) {
				break
			}

			// 只保留 .worktrees/ 下的条目；路径在此目录之外的一律忽略。
			rel, err := filepath.Rel(base, wtPath)
			if err != nil || strings.HasPrefix(rel, "..") {
				break
			}

			// rel 即为短名称，如 "req-001" 或 "fix-bug-001"。
			names = append(names, rel)
			break // 每个 block 只取一个 worktree 路径
		}
	}
	return names, nil
}

// listFromFS 通过扫描 .worktrees/ 文件系统目录获取 worktree 名称列表。
// 作为 listFromGit 的降级备用方案。
func listFromFS(projectRoot string) ([]string, error) {
	base := filepath.Join(projectRoot, worktreeDir)
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil // 目录不存在说明还没有创建过任何 worktree
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// BranchName 根据任务名称生成符合约定的 git 分支名。
//
//	req-001  → feat/req-001
//	fix-xxx  → fix/fix-xxx
func BranchName(name string) string {
	if strings.HasPrefix(name, "fix-") {
		return "fix/" + name
	}
	return "feat/" + name
}

// ── 内部辅助函数 ──────────────────────────────────────────────────────────────

// git 在指定目录下执行 git 命令，stdout/stderr 直接透传到终端。
func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// syncDirs 将 src 中的 skills/、prompts/ 和 rules/ 目录递归复制到 dst，
// 确保在 worktree 内运行的 Agent 能访问到最新的技能文件、提示词和开发规范。
// 已有文件会被覆盖，保持与项目根目录同步。
func syncDirs(src, dst string) error {
	for _, dir := range []string{"skills", "prompts", "rules"} {
		srcDir := filepath.Join(src, dir)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue // 源目录不存在则跳过（如项目尚未创建 prompts/）
		}
		dstDir := filepath.Join(dst, dir)
		if err := copyDir(srcDir, dstDir); err != nil {
			return fmt.Errorf("同步 %s 目录失败: %w", dir, err)
		}
	}
	return nil
}

// copyDir 递归地将 src 目录下的所有文件复制到 dst 目录。
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 计算相对路径，用于在目标目录中重建相同的结构。
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

// copyFile 将单个文件从 src 复制到 dst，保留原始权限位。
func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// 确保目标目录存在。
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
