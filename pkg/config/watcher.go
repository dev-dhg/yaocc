package config

import (
	"log"
	"os"
	"time"
)

// WatchConfig polls the config file for changes and calls onChange when a change is detected.
// This is a simple polling implementation to avoid external dependencies (fsnotify) for now.
func WatchConfig(path string, onChange func(*Config)) {
	var lastModTime time.Time

	// Initial check
	info, err := os.Stat(path)
	if err == nil {
		lastModTime = info.ModTime()
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(path)
		if err != nil {
			log.Printf("Error watching config file: %v", err)
			continue
		}

		if info.ModTime().After(lastModTime) {
			// Check for lock file
			if IsConfigLocked() {
				log.Println("Config change detected but ignored due to lock file (self-initiated update).")
				// Update lastModTime so we don't trigger on the next tick if the file hasn't changed *again*
				lastModTime = info.ModTime()
				continue
			}

			lastModTime = info.ModTime()
			log.Println("Config change detected, reloading...")

			newCfg, _, _, err := LoadConfig(path)
			if err != nil {
				log.Printf("Error reloading config: %v", err)
				continue
			}

			onChange(newCfg)
		}
	}
}
