// Package skill 负责扫描并加载项目中所有技能（skill）的定义。
// 每个技能对应 skills/<角色名>/ 目录，包含 SKILL.md 和 agent.md 两个核心文件。
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillDef 保存从 SKILL.md frontmatter 中解析出的技能元数据。
type SkillDef struct {
	Name        string // 技能名称，如 "manager"、"frontend"
	Model       string // 使用的 Claude 模型，如 "claude-opus-4-5"
	Description string // 技能功能描述（供日志和调试使用）
	AgentFile   string // agent.md 的绝对路径，作为 claude CLI 的 --system-prompt-file 参数
	RulesDir    string // rules/ 目录的绝对路径（可选，部分技能无规则文件）
}

// Loader 扫描 skills 目录，提供按角色名获取技能定义的能力。
type Loader struct {
	skillsDir string              // skills 根目录，如 "<项目根>/skills"
	skills    map[string]SkillDef // 已加载的技能，key 为角色名
}

// skillFrontmatter 对应 SKILL.md 文件头部的 YAML frontmatter 字段。
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Model       string `yaml:"model"`
	Description string `yaml:"description"`
}

// NewLoader 创建一个以 skillsDir 为根目录的 Loader。
// 典型调用: skill.NewLoader(filepath.Join(projectRoot, "skills"))
func NewLoader(skillsDir string) *Loader {
	return &Loader{skillsDir: skillsDir, skills: make(map[string]SkillDef)}
}

// LoadAll 扫描 skillsDir 下所有包含 SKILL.md 的子目录并加载对应的技能定义。
// 必须在调用 Get 之前执行。
func (l *Loader) LoadAll() (map[string]SkillDef, error) {
	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 skills 目录 %q 失败: %w", l.skillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 跳过非目录文件
		}
		role := entry.Name()
		skillFile := filepath.Join(l.skillsDir, role, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue // 没有 SKILL.md 则不是合法的技能目录，跳过
		}

		def, err := l.loadOne(role, skillFile)
		if err != nil {
			return nil, fmt.Errorf("加载技能 %q 失败: %w", role, err)
		}
		l.skills[role] = def
	}

	return l.skills, nil
}

// Get 按角色名返回对应的 SkillDef。
// 必须在 LoadAll 之后调用；若角色名不存在则返回错误。
func (l *Loader) Get(role string) (SkillDef, error) {
	def, ok := l.skills[role]
	if !ok {
		return SkillDef{}, fmt.Errorf("技能 %q 在目录 %q 中未找到", role, l.skillsDir)
	}
	return def, nil
}

// loadOne 解析单个 SKILL.md 并构建对应的 SkillDef。
func (l *Loader) loadOne(role, skillFile string) (SkillDef, error) {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return SkillDef{}, err
	}

	fm, err := parseFrontmatter(string(data))
	if err != nil {
		return SkillDef{}, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	// 若 frontmatter 中未设置 name，则使用目录名作为默认值。
	if fm.Name == "" {
		fm.Name = role
	}

	skillDir := filepath.Join(l.skillsDir, role)

	// agent.md 是必须存在的系统提示文件，用于驱动对应的 Claude Agent。
	agentFile := filepath.Join(skillDir, "agent.md")
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		return SkillDef{}, fmt.Errorf("agent.md 不存在: %q", agentFile)
	}

	// rules/ 目录是可选的，存放代码规范文件（.mdc 格式）。
	rulesDir := filepath.Join(skillDir, "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		rulesDir = "" // 不存在则置空，调用方按需处理
	}

	return SkillDef{
		Name:        fm.Name,
		Model:       fm.Model,
		Description: fm.Description,
		AgentFile:   agentFile,
		RulesDir:    rulesDir,
	}, nil
}

// parseFrontmatter 从 Markdown 内容中提取第一对 "---" 之间的 YAML 块并解析。
// 若文件没有 frontmatter，返回零值结构体（使用默认值）。
func parseFrontmatter(content string) (skillFrontmatter, error) {
	var fm skillFrontmatter

	// 去掉文件头部的空白字符和换行符。
	content = strings.TrimLeft(content, "\r\n ")
	if !strings.HasPrefix(content, "---") {
		return fm, nil // 无 frontmatter，直接使用默认值
	}

	// 查找闭合的 "---" 分隔符。
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return fm, fmt.Errorf("frontmatter 未以 --- 正常关闭")
	}

	yamlBlock := rest[:end]
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, err
	}
	return fm, nil
}
