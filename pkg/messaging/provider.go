package messaging

// Provider defines the interface that all messaging platforms must implement.
type Provider interface {
	// Name returns the unique name of the provider (e.g., "telegram", "discord").
	Name() string

	// Start initializes and starts the provider (e.g., starts polling or listening).
	Start()

	// SendMessage sends a text message to the specified target.
	// It should handle protocol prefixes (e.g., #IMAGE#:) internally if possible,
	// or specific media methods can be used.
	SendMessage(targetID string, message string) error

	// SendImage sends an image to the target. url van be a web URL or a local file path.
	SendImage(targetID string, url string, caption string) error

	// SendAudio sends an audio file to the target.
	SendAudio(targetID string, url string, caption string) error

	// SendVideo sends a video file to the target.
	SendVideo(targetID string, url string, caption string) error

	// SendDocument sends a general document to the target.
	SendDocument(targetID string, url string, caption string) error

	// SystemPromptInstruction returns a specific instruction to be added to the system prompt
	// for this provider (e.g. "Use Markdown for Telegram").
	SystemPromptInstruction() string
}
