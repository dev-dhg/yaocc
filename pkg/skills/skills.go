package skills

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Homepage    string                 `yaml:"homepage"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	Content     string                 `yaml:"-"` // Markdown content
	Path        string                 `yaml:"-"`
}

type Loader struct {
	Paths []string
}

func NewLoader(paths []string) *Loader {
	return &Loader{Paths: paths}
}

func (l *Loader) Load() ([]Skill, error) {
	var skills []Skill

	for _, path := range l.Paths {
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.ToLower(d.Name()) != "skill.md" {
				return nil
			}

			skill, err := parseSkillFile(p)
			if err != nil {
				return fmt.Errorf("failed to parse skill %s: %w", p, err)
			}
			skills = append(skills, *skill)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk path %s: %w", path, err)
		}
	}

	return skills, nil
}

func parseSkillFile(path string) (*Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var frontmatterLines []string
	var contentLines []string
	inFrontmatter := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				continue
			}
			frontmatterLines = append(frontmatterLines, line)
		} else {
			contentLines = append(contentLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var skill Skill
	if len(frontmatterLines) > 0 {
		frontmatterContent := strings.Join(frontmatterLines, "\n")
		if err := yaml.Unmarshal([]byte(frontmatterContent), &skill); err != nil {
			return nil, fmt.Errorf("failed to unmarshal frontmatter: %w", err)
		}
	}

	skill.Content = strings.Join(contentLines, "\n")
	skill.Path = path

	return &skill, nil
}
