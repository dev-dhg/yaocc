package chatdb

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ChatDB is the SQLite-backed chat history database.
type ChatDB struct {
	db *sql.DB
}

// ChatMessage represents a single message stored in the database.
type ChatMessage struct {
	ID        int64
	SessionID string
	Role      string // user, assistant, system, tool
	Content   string
	Timestamp time.Time
	Metadata  string // JSON blob (tool_call_id, tool_name, etc.)
}

// Open opens (or creates) the SQLite database at the given path and initializes the schema.
func Open(dbPath string) (*ChatDB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open chat database: %w", err)
	}

	// Verify connectivity
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping chat database: %w", err)
	}

	cdb := &ChatDB{db: db}
	if err := cdb.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize chat database schema: %w", err)
	}

	log.Printf("[ChatDB] Opened database at %s", dbPath)
	return cdb, nil
}

// Close closes the database connection.
func (c *ChatDB) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *ChatDB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT DEFAULT '{}'
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session_ts ON messages(session_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id, id);

	-- Summary checkpoint tracking for efficient rolling summaries
	CREATE TABLE IF NOT EXISTS summary_checkpoints (
		session_id TEXT PRIMARY KEY,
		last_message_id INTEGER NOT NULL DEFAULT 0,
		summary TEXT NOT NULL DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := c.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create base schema: %w", err)
	}

	// FTS5 virtual table — created separately because IF NOT EXISTS syntax differs
	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		content,
		content=messages,
		content_rowid=id
	);
	`
	if _, err := c.db.Exec(ftsSchema); err != nil {
		return fmt.Errorf("failed to create FTS5 table: %w", err)
	}

	// Triggers to keep FTS index in sync
	// We use INSERT OR IGNORE pattern — triggers are created only if not exist
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS messages_fts_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS messages_fts_ad AFTER DELETE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, content) VALUES('delete', old.id, old.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS messages_fts_au AFTER UPDATE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, content) VALUES('delete', old.id, old.content);
			INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
		END;`,
	}

	for _, trigger := range triggers {
		if _, err := c.db.Exec(trigger); err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}
	}

	return nil
}

// Append inserts a new message into the database.
func (c *ChatDB) Append(sessionID, role, content, metadata string) error {
	if metadata == "" {
		metadata = "{}"
	}
	_, err := c.db.Exec(
		`INSERT INTO messages (session_id, role, content, timestamp, metadata) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, time.Now().UTC().Format(time.RFC3339), metadata,
	)
	return err
}

// AppendWithTimestamp inserts a message with a specific timestamp (used for migration).
func (c *ChatDB) AppendWithTimestamp(sessionID, role, content, metadata string, ts time.Time) error {
	if metadata == "" {
		metadata = "{}"
	}
	_, err := c.db.Exec(
		`INSERT INTO messages (session_id, role, content, timestamp, metadata) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, ts.UTC().Format(time.RFC3339), metadata,
	)
	return err
}

// LoadHistory retrieves the last N messages for a session. Use limit=-1 for all messages.
func (c *ChatDB) LoadHistory(sessionID string, limit int) ([]ChatMessage, error) {
	var query string
	var args []interface{}

	if limit < 0 {
		// All messages
		query = `SELECT id, session_id, role, content, timestamp, metadata FROM messages WHERE session_id = ? ORDER BY id ASC`
		args = []interface{}{sessionID}
	} else {
		// Last N messages, but return them in chronological order
		query = `SELECT id, session_id, role, content, timestamp, metadata FROM (
			SELECT id, session_id, role, content, timestamp, metadata FROM messages
			WHERE session_id = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`
		args = []interface{}{sessionID, limit}
	}

	return c.queryMessages(query, args...)
}

// Search performs a full-text search within a session's messages.
func (c *ChatDB) Search(sessionID, query string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 10
	}

	sqlQuery := `
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp, m.metadata
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		WHERE m.session_id = ? AND messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`
	return c.queryMessages(sqlQuery, sessionID, query, limit)
}

// SearchByDate retrieves messages within a date range for a session.
func (c *ChatDB) SearchByDate(sessionID string, from, to time.Time, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, session_id, role, content, timestamp, metadata
		FROM messages
		WHERE session_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY id ASC
		LIMIT ?
	`
	return c.queryMessages(query, sessionID, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339), limit)
}

// SearchByDateAndQuery combines date range filtering with FTS5 search.
func (c *ChatDB) SearchByDateAndQuery(sessionID, searchQuery string, from, to time.Time, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp, m.metadata
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		WHERE m.session_id = ? AND messages_fts MATCH ? AND m.timestamp >= ? AND m.timestamp <= ?
		ORDER BY rank
		LIMIT ?
	`
	return c.queryMessages(query, sessionID, searchQuery, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339), limit)
}

// GetUnsummarizedMessages returns messages after the last summary checkpoint.
func (c *ChatDB) GetUnsummarizedMessages(sessionID string) ([]ChatMessage, int64, error) {
	checkpoint, err := c.GetSummaryCheckpoint(sessionID)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, session_id, role, content, timestamp, metadata
		FROM messages
		WHERE session_id = ? AND id > ?
		ORDER BY id ASC
	`
	msgs, err := c.queryMessages(query, sessionID, checkpoint)
	if err != nil {
		return nil, 0, err
	}

	// Find the last message ID for checkpoint update
	var lastID int64
	if len(msgs) > 0 {
		lastID = msgs[len(msgs)-1].ID
	} else {
		lastID = checkpoint
	}

	return msgs, lastID, nil
}

// SaveSummaryCheckpoint saves the summary and the last message ID that was summarized.
func (c *ChatDB) SaveSummaryCheckpoint(sessionID string, lastMessageID int64, summary string) error {
	_, err := c.db.Exec(`
		INSERT INTO summary_checkpoints (session_id, last_message_id, summary, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			last_message_id = excluded.last_message_id,
			summary = excluded.summary,
			updated_at = excluded.updated_at
	`, sessionID, lastMessageID, summary, time.Now().UTC().Format(time.RFC3339))
	return err
}

// GetSummaryCheckpoint returns the last summarized message ID for a session.
func (c *ChatDB) GetSummaryCheckpoint(sessionID string) (int64, error) {
	var lastID int64
	err := c.db.QueryRow(
		`SELECT last_message_id FROM summary_checkpoints WHERE session_id = ?`,
		sessionID,
	).Scan(&lastID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return lastID, err
}

// LoadSummary returns the stored summary for a session.
func (c *ChatDB) LoadSummary(sessionID string) (string, error) {
	var summary string
	err := c.db.QueryRow(
		`SELECT summary FROM summary_checkpoints WHERE session_id = ?`,
		sessionID,
	).Scan(&summary)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return summary, err
}

// FormatMessages formats a slice of ChatMessages into a human-readable string.
func FormatMessages(messages []ChatMessage) string {
	if len(messages) == 0 {
		return "No messages found."
	}

	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			msg.Timestamp.Format("2006-01-02 15:04"),
			strings.Title(msg.Role),
			msg.Content,
		))
		sb.WriteString("---\n")
	}
	return sb.String()
}

// queryMessages is a helper that executes a query and scans results into ChatMessage slices.
func (c *ChatDB) queryMessages(query string, args ...interface{}) ([]ChatMessage, error) {
	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var ts string
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &ts, &msg.Metadata); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		// Parse timestamp
		msg.Timestamp, _ = time.Parse(time.RFC3339, ts)
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
