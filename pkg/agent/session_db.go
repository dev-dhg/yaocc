package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-dhg/yaocc/pkg/chatdb"
	"github.com/dev-dhg/yaocc/pkg/llm"
)

// DBSessionStore implements SessionStore using SQLite.
type DBSessionStore struct {
	BaseDir string
	db      *chatdb.ChatDB
}

// NewDBSessionStore creates a new SQLite-backed session store.
func NewDBSessionStore(baseDir string) (*DBSessionStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	dbPath := filepath.Join(baseDir, "sessions.db")
	db, err := chatdb.Open(dbPath)
	if err != nil {
		return nil, err
	}

	return &DBSessionStore{
		BaseDir: baseDir,
		db:      db,
	}, nil
}

func (s *DBSessionStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *DBSessionStore) LoadHistory(sessionID string, limit int) ([]llm.Message, error) {
	dbMsgs, err := s.db.LoadHistory(sessionID, limit)
	if err != nil {
		return nil, err
	}

	var messages []llm.Message
	for _, msg := range dbMsgs {
		llmMsg := llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if msg.Metadata != "" && msg.Metadata != "{}" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Metadata), &meta); err == nil {
				if id, ok := meta["tool_call_id"].(string); ok {
					llmMsg.ToolCallID = id
				}
				if name, ok := meta["name"].(string); ok {
					llmMsg.Name = name
				}
				// Reconstruct tool calls if present (simplified assumption for now, usually role=assistant with tool_calls)
				// Full reconstruction of complex tool call trees might require deeper DB schema
			}
		}

		messages = append(messages, llmMsg)
	}

	return messages, nil
}

func (s *DBSessionStore) Append(sessionID string, role, content string) error {
	return s.db.Append(sessionID, role, content, "")
}

// AppendWithMetadata allows storing extra fields like tool_call_id.
// It's not part of the base SessionStore interface yet, but DBSessionStore implements it.
func (s *DBSessionStore) AppendWithMetadata(sessionID string, role, content string, metadata map[string]interface{}) error {
	metaStr := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err == nil {
			metaStr = string(b)
		}
	}
	return s.db.Append(sessionID, role, content, metaStr)
}

func (s *DBSessionStore) LoadSummary(sessionID string) (string, error) {
	return s.db.LoadSummary(sessionID)
}

func (s *DBSessionStore) SaveSummary(sessionID, content string) error {
	// For generic save (not rolling) we just use a 0 checkpoint or get the latest manually
	msgs, err := s.db.LoadHistory(sessionID, 1)
	var latestID int64
	if err == nil && len(msgs) > 0 {
		latestID = msgs[0].ID
	}
	return s.db.SaveSummaryCheckpoint(sessionID, latestID, content)
}

func (s *DBSessionStore) SaveSummaryCheckpoint(sessionID string, lastMessageID int64, content string) error {
	return s.db.SaveSummaryCheckpoint(sessionID, lastMessageID, content)
}

func (s *DBSessionStore) GetUnsummarizedMessages(sessionID string) ([]llm.Message, int64, error) {
	dbMsgs, lastID, err := s.db.GetUnsummarizedMessages(sessionID)
	if err != nil {
		return nil, 0, err
	}

	var messages []llm.Message
	for _, msg := range dbMsgs {
		messages = append(messages, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages, lastID, nil
}

// Locks

func (s *DBSessionStore) GetLockFile(sessionID string) string {
	safeID := filepath.Base(filepath.Clean(sessionID))
	if safeID == "." || safeID == "/" {
		safeID = "general"
	}
	return filepath.Join(s.BaseDir, safeID+".lock")
}

func (s *DBSessionStore) AcquireLock(sessionID string) (func(), error) {
	lockPath := s.GetLockFile(sessionID)
	if _, err := os.Stat(lockPath); err == nil {
		return nil, fmt.Errorf("session locked")
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	f.Close()

	return func() {
		os.Remove(lockPath)
	}, nil
}

func (s *DBSessionStore) WaitForLock(sessionID string, timeout time.Duration) error {
	lockPath := s.GetLockFile(sessionID)
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

// Helper getter
func (s *DBSessionStore) DB() *chatdb.ChatDB {
	return s.db
}
