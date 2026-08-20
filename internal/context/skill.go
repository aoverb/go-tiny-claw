package context

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Body        string
}

type SkillLoader struct {
	workDir string
}

func NewSkillLoader(workDir string) *SkillLoader {
	return &SkillLoader{
		workDir: workDir,
	}
}

func (s *SkillLoader) LoadAll() string {
	skillBaseDir := filepath.Join(s.workDir, ".claw", "skills")

	if _, err := os.Stat(skillBaseDir); os.IsNotExist(err) {
		return ""
	}

	var skillBuilder strings.Builder
	skillBuilder.WriteString("\n### 可用专业技能 (Agent Skills)\n")
	skillBuilder.WriteString("以下是你拥有的标准化外挂技能，请在符合 description 描述的场景下严格遵循其正文指令：\n\n")

	err := filepath.WalkDir(skillBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			skill := parseSkillMD(string(content))

			skillBuilder.WriteString(fmt.Sprintf("#### 技能名称: %s\n", skill.Name))
			skillBuilder.WriteString(fmt.Sprintf("**触发条件**: %s\n\n", skill.Description))
			skillBuilder.WriteString("**执行指南**:\n")
			skillBuilder.WriteString(skill.Body)
			skillBuilder.WriteString("\n\n---\n")
		}
		return nil
	})

	if err != nil || skillBuilder.Len() < 100 {
		return ""
	}
	return skillBuilder.String()
}

func parseSkillMD(content string) Skill {
	skill := Skill{
		Name:        "Unknown Skill",
		Description: "No description provided.",
		Body:        content, // 默认将全量内容作为 body
	}

	// 简单解析 YAML Frontmatter (以 --- 包裹)
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) == 3 {
			frontmatter := parts[1]
			skill.Body = strings.TrimSpace(parts[2])

			// 逐行提取 metadata
			lines := strings.Split(frontmatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					skill.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				} else if strings.HasPrefix(line, "description:") {
					skill.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				}
			}
		}
	}

	return skill
}
