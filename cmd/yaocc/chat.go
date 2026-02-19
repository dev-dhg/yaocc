package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/utils"
)

type ChatRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Provider  string `json:"provider,omitempty"`
	ChatID    string `json:"chatId,omitempty"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func runChat(args []string) {
	chatCmd := flag.NewFlagSet("chat", flag.ExitOnError)
	sessionID := chatCmd.String("session", "", "Session ID for the conversation (optional if provider/chat-id set)")
	provider := chatCmd.String("provider", "", "Provider context (e.g., telegram, local)")
	chatID := chatCmd.String("chat-id", "", "Chat ID for the provider context")

	if err := chatCmd.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		return
	}

	if chatCmd.NArg() < 1 {
		fmt.Println("Usage: yaocc chat [--session <id>] [--provider <name>] [--chat-id <id>] <message>")
		return
	}

	message := chatCmd.Arg(0)
	serverURL := "http://localhost:8080/chat" // TODO: Read from config or flags

	reqBody, _ := json.Marshal(ChatRequest{
		SessionID: *sessionID,
		Provider:  *provider,
		ChatID:    *chatID,
		Message:   message,
	})
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Server returned error: %s\n", string(body))
		return
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		fmt.Printf("Error decoding response: %v\n", err)
		return
	}

	if chatResp.Error != "" {
		fmt.Printf("Agent Error: %s\n", chatResp.Error)
	} else {
		response := chatResp.Response
		if strings.HasPrefix(response, "#BASE64_IMAGE#:") {
			data := strings.TrimPrefix(response, "#BASE64_IMAGE#:")
			path, err := utils.SaveBase64ToTempFile(data, "yaocc-image")
			if err != nil {
				fmt.Printf("Error processing image: %v\n", err)
			} else {
				fmt.Printf("Agent sent an image. Saved to: %s\n", path)
			}
		} else if strings.HasPrefix(response, "#IMAGE#:") {
			content := strings.TrimPrefix(response, "#IMAGE#:")
			fmt.Printf("Agent sent an image: %s\n", content)
		} else if strings.HasPrefix(response, "#AUDIO#:") {
			content := strings.TrimPrefix(response, "#AUDIO#:")
			fmt.Printf("Agent sent audio: %s\n", content)
		} else if strings.HasPrefix(response, "#VIDEO#:") {
			content := strings.TrimPrefix(response, "#VIDEO#:")
			fmt.Printf("Agent sent video: %s\n", content)
		} else if strings.HasPrefix(response, "#DOC#:") {
			content := strings.TrimPrefix(response, "#DOC#:")
			fmt.Printf("Agent sent document: %s\n", content)
		} else {
			fmt.Printf("Agent: %s\n", chatResp.Response)
		}
	}
}
