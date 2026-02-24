package templates

import (
	"embed"
)

//go:embed file_skill.md cron_skill.md websearch_skill.md fetch_skill.md prompt_skill.md exec_skill.md skills_skill.md
var Files embed.FS
