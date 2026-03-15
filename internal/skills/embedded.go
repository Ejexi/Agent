package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed data
var embeddedSkills embed.FS

// Skill represents a standalone capability guide that the agent can read.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// Registry provides an interface to access static Agent skills.
type Registry interface {
	GetSkill(name string) (*Skill, error)
	ListSkills() []Skill
}

type embeddedRegistry struct {
	skills map[string]Skill
}

// NewEmbeddedRegistry reads all the Markdown files from the data folder recursively.
func NewEmbeddedRegistry() (Registry, error) {
	reg := &embeddedRegistry{
		skills: make(map[string]Skill),
	}

	err := fs.WalkDir(embeddedSkills, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		contentBytes, err := embeddedSkills.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read skill file %s: %w", path, err)
		}

		content := string(contentBytes)
		
		// Use relative path from 'data/' as the skill name (e.g., "vulnerabilities/xss")
		relPath, _ := filepath.Rel("data", path)
		name := strings.TrimSuffix(relPath, ".md")
		name = strings.ReplaceAll(name, "\\", "/") // Ensure consistent forward slashes
		
		description := "A skill module focusing on " + name

		// Extract description (same logic as before)
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if len(line) > 5 && !strings.HasPrefix(line, "#") {
				description = strings.TrimSpace(line)
				if len(description) > 100 {
					description = description[:97] + "..."
				}
				break
			}
		}

		reg.skills[name] = Skill{
			Name:        name,
			Description: description,
			Content:     content,
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded skills: %w", err)
	}

	return reg, nil
}

func (r *embeddedRegistry) GetSkill(name string) (*Skill, error) {
	if skill, ok := r.skills[name]; ok {
		return &skill, nil
	}
	return nil, fmt.Errorf("skill '%s' not found", name)
}

func (r *embeddedRegistry) ListSkills() []Skill {
	var list []Skill
	for _, skill := range r.skills {
		// we omit the content in listing to save space
		list = append(list, Skill{Name: skill.Name, Description: skill.Description})
	}
	return list
}
