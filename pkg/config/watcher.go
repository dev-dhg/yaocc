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

// WatchFile polls a generic file for changes and calls onChange when a change is detected.
// Unlike WatchConfig, this does not check for a lock file since the separate files
// (cron.json, skills_register.json) are designed to be independently updated.
func WatchFile(path string, onChange func()) {
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
			// File might not exist yet, that's ok
			if !os.IsNotExist(err) {
				log.Printf("Error watching file %s: %v", path, err)
			}
			continue
		}

		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
			log.Printf("File change detected: %s, triggering reload...", path)
			onChange()
		}
	}
}
