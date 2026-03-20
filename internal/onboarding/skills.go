package onboarding

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed skills/*.md
var skillsFS embed.FS

// SkillInfo describes an available skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
}

// ListSkills returns all embedded skill files.
func ListSkills() ([]SkillInfo, error) {
	var skills []SkillInfo

	err := fs.WalkDir(skillsFS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		name := strings.TrimSuffix(filepath.Base(path), ".md")
		description := skillDescription(name)

		skills = append(skills, SkillInfo{
			Name:        name,
			Filename:    filepath.Base(path),
			Description: description,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	return skills, nil
}

// GetSkill returns the markdown content of a skill by name.
func GetSkill(name string) (string, error) {
	filename := name + ".md"
	data, err := skillsFS.ReadFile(filepath.Join("skills", filename))
	if err != nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return string(data), nil
}

// skillDescription returns a short description for a skill by name.
func skillDescription(name string) string {
	descriptions := map[string]string{
		"stigmergy-workflow": "Stigmergy-based workflow for claiming, processing, and completing work items on channels",
		"task-auction":       "Task auction workflow for bidding on and executing tasks in auction channels",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Agent skill"
}
