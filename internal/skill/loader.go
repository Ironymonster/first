package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillDef holds metadata parsed from a skill's SKILL.md frontmatter.
type SkillDef struct {
	Name        string
	Model       string
	Description string
	AgentFile   string // absolute path to agent.md
	RulesDir    string // absolute path to rules/ (may be empty)
}

// Loader scans a skills directory and provides access to skill definitions.
type Loader struct {
	skillsDir string
	skills    map[string]SkillDef
}

// skillFrontmatter matches the YAML frontmatter in SKILL.md files.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Model       string `yaml:"model"`
	Description string `yaml:"description"`
}

// NewLoader creates a Loader rooted at skillsDir (typically "<project>/skills").
func NewLoader(skillsDir string) *Loader {
	return &Loader{skillsDir: skillsDir, skills: make(map[string]SkillDef)}
}

// LoadAll scans skillsDir for subdirectories containing SKILL.md and loads each.
func (l *Loader) LoadAll() (map[string]SkillDef, error) {
	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("reading skills dir %q: %w", l.skillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		role := entry.Name()
		skillFile := filepath.Join(l.skillsDir, role, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue // not a skill directory
		}

		def, err := l.loadOne(role, skillFile)
		if err != nil {
			return nil, fmt.Errorf("loading skill %q: %w", role, err)
		}
		l.skills[role] = def
	}

	return l.skills, nil
}

// Get returns the SkillDef for the given role name. LoadAll must be called first.
func (l *Loader) Get(role string) (SkillDef, error) {
	def, ok := l.skills[role]
	if !ok {
		return SkillDef{}, fmt.Errorf("skill %q not found in %q", role, l.skillsDir)
	}
	return def, nil
}

// loadOne parses a single SKILL.md and builds the SkillDef.
func (l *Loader) loadOne(role, skillFile string) (SkillDef, error) {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return SkillDef{}, err
	}

	fm, err := parseFrontmatter(string(data))
	if err != nil {
		return SkillDef{}, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// Default name to directory name if not set in frontmatter.
	if fm.Name == "" {
		fm.Name = role
	}

	skillDir := filepath.Join(l.skillsDir, role)
	agentFile := filepath.Join(skillDir, "agent.md")
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		return SkillDef{}, fmt.Errorf("agent.md not found at %q", agentFile)
	}

	rulesDir := filepath.Join(skillDir, "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		rulesDir = "" // optional
	}

	return SkillDef{
		Name:        fm.Name,
		Model:       fm.Model,
		Description: fm.Description,
		AgentFile:   agentFile,
		RulesDir:    rulesDir,
	}, nil
}

// parseFrontmatter extracts YAML between the first pair of "---" delimiters.
func parseFrontmatter(content string) (skillFrontmatter, error) {
	var fm skillFrontmatter

	// Strip leading whitespace/newlines.
	content = strings.TrimLeft(content, "\r\n ")
	if !strings.HasPrefix(content, "---") {
		return fm, nil // no frontmatter — use defaults
	}

	// Find closing "---".
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return fm, fmt.Errorf("frontmatter not closed with ---")
	}

	yamlBlock := rest[:end]
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, err
	}
	return fm, nil
}
