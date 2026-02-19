package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/llm"
)

type SessionManager struct {
	BaseDir string
}

func NewSessionManager(baseDir string) *SessionManager {
	return &SessionManager{BaseDir: baseDir}
}

func (sm *SessionManager) GetSessionFile(sessionID string) string {
	// Sanitize session ID to prevent path traversal
	safeID := filepath.Base(filepath.Clean(sessionID))
	if safeID == "." || safeID == "/" {
		safeID = "general"
	}
	return filepath.Join(sm.BaseDir, safeID+".md")
}

func (sm *SessionManager) GetSummaryFile(sessionID string) string {
	safeID := filepath.Base(filepath.Clean(sessionID))
	if safeID == "." || safeID == "/" {
		safeID = "general"
	}
	return filepath.Join(sm.BaseDir, safeID+"-summary.md")
}

func (sm *SessionManager) GetLockFile(sessionID string) string {
	safeID := filepath.Base(filepath.Clean(sessionID))
	if safeID == "." || safeID == "/" {
		safeID = "general"
	}
	return filepath.Join(sm.BaseDir, safeID+".lock")
}

func (sm *SessionManager) LoadHistory(sessionID string) ([]llm.Message, error) {
	path := sm.GetSessionFile(sessionID)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []llm.Message{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parseMarkdownHistory(string(content)), nil
}

func (sm *SessionManager) LoadSummary(sessionID string) (string, error) {
	path := sm.GetSummaryFile(sessionID)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (sm *SessionManager) Append(sessionID string, role, content string) error {
	path := sm.GetSessionFile(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("\n### %s (%s)\n\n%s\n", strings.Title(role), timestamp, content)
	if _, err := f.WriteString(entry); err != nil {
		return err
	}
	return nil
}

func (sm *SessionManager) SaveSummary(sessionID, content string) error {
	path := sm.GetSummaryFile(sessionID)
	return os.WriteFile(path, []byte(content), 0644)
}

// AcquireLock attempts to create a .lock file.
// It returns a release function and an error if it failed.
// It does NOT wait. The caller should implement waiting if needed.
func (sm *SessionManager) AcquireLock(sessionID string) (func(), error) {
	lockPath := sm.GetLockFile(sessionID)
	// Check if exists
	if _, err := os.Stat(lockPath); err == nil {
		return nil, fmt.Errorf("session locked")
	}

	// Create lock file
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	f.Close()

	return func() {
		os.Remove(lockPath)
	}, nil
}

// WaitForLock waits for the lock to be released, with a timeout.
func (sm *SessionManager) WaitForLock(sessionID string, timeout time.Duration) error {
	lockPath := sm.GetLockFile(sessionID)
	start := time.Now()
	for {
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for lock")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func parseMarkdownHistory(content string) []llm.Message {
	// simplistic parser: looks for ### Role headers
	// This is a placeholder. Real implementation needs robust parsing.
	// For now, we might just treat the whole file as previous context?
	// Or just return empty if we rely on LLM to read the file context directly?
	// Let's implement a simple line-based parser.

	var messages []llm.Message
	lines := strings.Split(content, "\n")
	var currentRole string
	var currentContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "### User") {
			if currentRole != "" {
				messages = append(messages, llm.Message{Role: strings.ToLower(currentRole), Content: strings.TrimSpace(currentContent.String())})
				currentContent.Reset()
			}
			currentRole = "user"
		} else if strings.HasPrefix(line, "### Assistant") || strings.HasPrefix(line, "### Model") {
			if currentRole != "" {
				messages = append(messages, llm.Message{Role: strings.ToLower(currentRole), Content: strings.TrimSpace(currentContent.String())})
				currentContent.Reset()
			}
			currentRole = "assistant"
		} else if strings.HasPrefix(line, "### System") {
			if currentRole != "" {
				messages = append(messages, llm.Message{Role: strings.ToLower(currentRole), Content: strings.TrimSpace(currentContent.String())})
				currentContent.Reset()
			}
			currentRole = "system"
		} else {
			currentContent.WriteString(line + "\n")
		}
	}
	if currentRole != "" {
		messages = append(messages, llm.Message{Role: strings.ToLower(currentRole), Content: strings.TrimSpace(currentContent.String())})
	}

	return messages
}
