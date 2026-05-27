package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/chatdb"
	"github.com/dev-dhg/yaocc/pkg/config"
)

func runMigrateSessions(args []string) {
	migrateCmd := flag.NewFlagSet("migrate-sessions", flag.ExitOnError)
	configPath := migrateCmd.String("config", "config.json", "Path to config file")
	dryRun := migrateCmd.Bool("dry-run", false, "Preview migration without writing to database")

	if err := migrateCmd.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		return
	}

	_, configDir, _, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	sessionsDir := filepath.Join(configDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		fmt.Println("No sessions directory found.")
		return
	}

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		fmt.Printf("Error reading sessions directory: %v\n", err)
		return
	}

	var db *chatdb.ChatDB
	if !*dryRun {
		dbPath := filepath.Join(sessionsDir, "sessions.db")
		db, err = chatdb.Open(dbPath)
		if err != nil {
			fmt.Printf("Error opening chat database: %v\n", err)
			return
		}
		defer db.Close()
	}

	sm := agent.NewSessionManager(sessionsDir)
	sessionCount := 0
	messageCount := 0

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") || strings.HasSuffix(file.Name(), "-summary.md") {
			continue
		}

		sessionID := strings.TrimSuffix(file.Name(), ".md")
		messages, err := sm.LoadHistory(sessionID, -1) // Temporary override the new limit parameter for migration logic
		if err != nil {
			log.Printf("Failed to load history for %s: %v", sessionID, err)
			continue
		}

		if len(messages) == 0 {
			continue
		}

		sessionCount++
		messageCount += len(messages)

		if *dryRun {
			fmt.Printf("[Dry Run] Would migrate session %s (%d messages)\n", sessionID, len(messages))
			continue
		}

		fmt.Printf("Migrating session %s (%d messages)... ", sessionID, len(messages))

		// Process messages. Since we don't have accurate timestamps in the parsed markdown easily
		// (parseMarkdownHistory doesn't extract the timestamp from the header), we'll insert them
		// spaced out by seconds so order is strictly preserved.
		baseTime := time.Now().Add(-time.Duration(len(messages)) * time.Second)

		var lastID int64
		for i, msg := range messages {
			ts := baseTime.Add(time.Duration(i) * time.Second)

			// Simple check if there was a tool call by checking metadata
			metaStr := "{}"
			if msg.ToolCallID != "" || msg.Name != "" || len(msg.ToolCalls) > 0 {
				meta := map[string]interface{}{}
				if msg.ToolCallID != "" {
					meta["tool_call_id"] = msg.ToolCallID
				}
				if msg.Name != "" {
					meta["name"] = msg.Name
				}
				// ignoring complex tool_calls array for migration simplicity
				// b, _ := json.Marshal(meta)
				// metaStr = string(b)
				// Wait, let's just do a simple string format for simplicity.
				metaStr = fmt.Sprintf(`{"name":"%s","tool_call_id":"%s"}`, msg.Name, msg.ToolCallID)
			}

			contentStr := ""
			if str, ok := msg.Content.(string); ok {
				contentStr = str
			}
			if err := db.AppendWithTimestamp(sessionID, msg.Role, contentStr, metaStr, ts); err != nil {
				log.Printf("\nError inserting message: %v", err)
			}
		}

		// Hacky way to get the last inserted ID
		if msgs, err := db.LoadHistory(sessionID, 1); err == nil && len(msgs) > 0 {
			lastID = msgs[0].ID
		}

		fmt.Println("Done")

		// Migrate summary if exists
		summaryContent, err := sm.LoadSummary(sessionID)
		if err == nil && summaryContent != "" {
			if err := db.SaveSummaryCheckpoint(sessionID, lastID, summaryContent); err != nil {
				log.Printf("Error saving summary checkpoint for %s: %v", sessionID, err)
			} else {
				fmt.Printf("  -> Migrated summary checkpoint for %s\n", sessionID)
			}
		}
	}

	if *dryRun {
		fmt.Printf("\nDry run complete. Would migrate %d sessions (%d messages).\n", sessionCount, messageCount)
	} else {
		fmt.Printf("\nMigration complete! Migrated %d sessions (%d messages) to SQLite DB.\n", sessionCount, messageCount)
		fmt.Println("You can now set \"historyMode\": \"db\" in your config.json if you haven't already.")
	}
}
