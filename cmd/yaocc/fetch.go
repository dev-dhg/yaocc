package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

func runFetch(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: yaocc fetch <url>")
		os.Exit(1)
	}

	url := args[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Try to load config for storage path
	var tempDir string
	cfg, _, _, err := config.LoadConfig("config.json")
	if err == nil && cfg.Storage.TempDir != "" {
		tempDir = cfg.Storage.TempDir
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			fmt.Printf("Warning: could not create temp dir %s: %v\n", tempDir, err)
			tempDir = "" // Fallback to current dir
		}
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: HTTP Status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	contentType := resp.Header.Get("Content-Type")

	var fileType, prefix, ext string
	if strings.HasPrefix(contentType, "image/") {
		fileType = "image"
		prefix = "#IMAGE#:"
		ext = ".png" // Default
		if strings.Contains(contentType, "jpeg") {
			ext = ".jpg"
		} else if strings.Contains(contentType, "gif") {
			ext = ".gif"
		} else if strings.Contains(contentType, "webp") {
			ext = ".webp"
		}
	} else if strings.HasPrefix(contentType, "audio/") {
		fileType = "audio"
		prefix = "#AUDIO#:"
		ext = ".mp3" // Default
		if strings.Contains(contentType, "ogg") {
			ext = ".ogg"
		} else if strings.Contains(contentType, "wav") {
			ext = ".wav"
		} else if strings.Contains(contentType, "mpeg") {
			ext = ".mp3"
		}
	} else if strings.HasPrefix(contentType, "video/") {
		fileType = "video"
		prefix = "#VIDEO#:"
		ext = ".mp4" // Default
		if strings.Contains(contentType, "webm") {
			ext = ".webm"
		} else if strings.Contains(contentType, "avi") {
			ext = ".avi"
		}
	}

	// If it's media, save to a temporary file and tell the Agent to use it
	if fileType != "" {
		filename := fmt.Sprintf("fetched_%s_%d%s", fileType, time.Now().Unix(), ext)
		var filePath string
		if tempDir != "" {
			filePath = filepath.Join(tempDir, filename)
		} else {
			filePath = filename
		}

		file, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			fmt.Printf("Error saving %s: %v\n", fileType, err)
			os.Exit(1)
		}

		absPath, _ := filepath.Abs(filePath)
		fmt.Printf("%s saved to: %s\n", strings.Title(fileType), absPath)
		fmt.Printf("SYSTEM HINT: To display this %s to the user, output exactly:\n%s%s\n", fileType, prefix, absPath)
		return
	}

	// For text/html/json, just print body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(body))
}
