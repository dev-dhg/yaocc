package templates

import (
	"embed"
)

//go:embed *
var Files embed.FS

// updated to force embed refresh
