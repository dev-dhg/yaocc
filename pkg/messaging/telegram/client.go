package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/utils"
)

type Client struct {
	Token        string
	AllowedUsers []string
	Agent        *agent.Agent
	Offset       int
	HttpClient   *http.Client
}

func NewClient(cfg config.TelegramConfig, agt *agent.Agent) *Client {
	return &Client{
		Token:        cfg.BotToken,
		AllowedUsers: cfg.AllowedUsers,
		Agent:        agt,
		Offset:       0,
		HttpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) StartPolling() {
	log.Println("Starting Telegram polling...")

	me, err := c.GetMe()
	if err != nil {
		log.Printf("Warning: failed to get bot info: %v", err)
	} else {
		log.Printf("Starting Telegram polling for bot @%s", me.Username)
	}

	for {
		updates, err := c.getUpdates()
		if err != nil {
			log.Printf("Error getting updates: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= c.Offset {
				c.Offset = update.UpdateID + 1
			}
			c.handleUpdate(update)
		}

		time.Sleep(1 * time.Second)
	}
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type GetMeResponse struct {
	Ok     bool `json:"ok"`
	Result User `json:"result"`
}

func (c *Client) GetMe() (*User, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", c.Token)
	resp, err := c.HttpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GetMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Ok {
		return nil, fmt.Errorf("telegram api error")
	}

	return &result.Result, nil
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Text string `json:"text"`
}

type GetUpdatesResponse struct {
	Ok     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func (c *Client) getUpdates() ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=10", c.Token, c.Offset)
	resp, err := c.HttpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Ok {
		return nil, fmt.Errorf("telegram api error")
	}

	return result.Result, nil
}

func (c *Client) handleUpdate(update Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	userID := strconv.FormatInt(update.Message.From.ID, 10)
	allowed := false
	for _, allowedUser := range c.AllowedUsers {
		if allowedUser == userID {
			allowed = true
			break
		}
	}

	if !allowed {
		log.Printf("Unauthorized access attempt from user %s", userID)
		return
	}

	chatID := update.Message.Chat.ID
	sessionID := fmt.Sprintf("telegram-%d", chatID)

	log.Printf("Received message from %s: %s", sessionID, update.Message.Text)

	// Start continuous typing action
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Send immediately
		c.SendChatAction(chatID, "typing")

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				c.SendChatAction(chatID, "typing")
			}
		}
	}()
	// Ensure we stop the ticker when this function exits (success or error)
	defer close(done)

	// Process with Agent
	response, err := c.Agent.Run(sessionID, c, strconv.FormatInt(chatID, 10), update.Message.Text)
	if err != nil {
		log.Printf("Agent error: %v", err)
		c.sendMessageInt64(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	// Parse targetID (string) from sessionID?
	// The Agent returns just text. The client needs to reply to the same chatID.
	// But `handleUpdate` has `chatID` (int64) locally available.
	// So we can call `c.sendMessageInt64(chatID, response)` directly.
	// We don't use the generic interface methods inside `handleUpdate` necessarily,
	// but we could if we wanted to be perfectly generic.
	// But `handleUpdate` is specific to Telegram updates.
	// So using internal methods is fine.

	// However, we recently changed `SendMessage` to be generic string ID.
	// So we should call `c.sendMessageInt64(chatID, response)`.
	// Parse targetID (string) from sessionID?
	// The Agent returns just text. The client needs to reply to the same chatID.
	// But `handleUpdate` has `chatID` (int64) locally available.
	// So we can call `c.sendMessageInt64(chatID, response)` directly.
	c.sendMessageInt64(chatID, response)
}

func (c *Client) SendChatAction(chatID int64, action string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", c.Token)
	body := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := c.HttpClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Provider Interface Implementation

func (c *Client) Name() string {
	return "telegram"
}

func (c *Client) Start() {
	go c.StartPolling()
}

func (c *Client) SendMessage(targetID string, message string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID '%s': %v", targetID, err)
	}
	// Call internal/specific method
	return c.sendMessageInt64(chatID, message)
}

func (c *Client) SendImage(targetID string, url string, caption string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID: %v", err)
	}
	return c.sendPhotoInt64(chatID, url, caption)
}

func (c *Client) SendAudio(targetID string, url string, caption string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID: %v", err)
	}
	return c.sendAudioInt64(chatID, url, caption)
}

func (c *Client) SendVideo(targetID string, url string, caption string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID: %v", err)
	}
	return c.sendVideoInt64(chatID, url, caption)
}

func (c *Client) SendDocument(targetID string, url string, caption string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID: %v", err)
	}
	return c.sendDocumentInt64(chatID, url, caption)
}

func (c *Client) SystemPromptInstruction() string {
	return `
## Telegram Context
You are talking to a user via Telegram. Please use Markdown formatting (bold, italic, lists) to make your messages easy to read.
Avoid using characters that conflict with Markdown syntax (like unescaped underscores in variable names) unless you are inside a code block.
`
}

// Internal/Specific Methods (formerly public, now renamed or kept as helpers)

type MediaItem struct {
	Type    string
	Content string
}

func (c *Client) sendMessageInt64(chatID int64, text string) error {
	cleanText, mediaItems := c.parseMedia(text)

	// 1. Send Text if present
	if cleanText != "" {
		// Split text into chunks if too long
		// Telegram limit is 4096. We use 4000 to be safe.
		chunks := c.splitText(cleanText, 4000)

		for i, chunk := range chunks {
			if i > 0 {
				time.Sleep(200 * time.Millisecond) // Small delay between chunks
			}

			url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.Token)
			body := map[string]interface{}{
				"chat_id":    chatID,
				"text":       chunk,
				"parse_mode": "Markdown",
			}

			err := c.postJSON(url, body)
			if err != nil {
				// Check if error is related to markdown parsing
				// Telegram error for bad markdown usually contains "can't parse entities"
				if strings.Contains(err.Error(), "can't parse entities") {
					log.Printf("Markdown parsing failed (%v) for chunk %d/%d, retrying as plain text...", err, i+1, len(chunks))

					// Retry without parse_mode
					delete(body, "parse_mode")
					if retryErr := c.postJSON(url, body); retryErr != nil {
						log.Printf("Error sending text chunk %d: %v", i+1, retryErr)
						// Should we break or continue? Usually continue to try sending the rest.
					}
				} else {
					log.Printf("Error sending text chunk %d: %v", i+1, err)
				}
				// Continue to next chunk
			}
		}
	}

	// 2. Send Media if present
	if len(mediaItems) > 0 {
		return c.sendMediaItems(chatID, mediaItems)
	}

	return nil
}

func (c *Client) splitText(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	totalRunes := len(runes)

	for i := 0; i < totalRunes; i += limit {
		end := i + limit
		if end > totalRunes {
			end = totalRunes
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

func (c *Client) parseMedia(text string) (string, []MediaItem) {
	lines := strings.Split(text, "\n")
	var textLines []string
	var mediaItems []MediaItem

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var item MediaItem
		matched := false

		if strings.HasPrefix(trimmed, "#STICKER#:") {
			item = MediaItem{Type: "STICKER", Content: strings.TrimPrefix(trimmed, "#STICKER#:")}
			matched = true
		} else if strings.HasPrefix(trimmed, "#IMAGE#:") {
			item = MediaItem{Type: "IMAGE", Content: strings.TrimPrefix(trimmed, "#IMAGE#:")}
			matched = true
		} else if strings.HasPrefix(trimmed, "#AUDIO#:") {
			item = MediaItem{Type: "AUDIO", Content: strings.TrimPrefix(trimmed, "#AUDIO#:")}
			matched = true
		} else if strings.HasPrefix(trimmed, "#VIDEO#:") {
			item = MediaItem{Type: "VIDEO", Content: strings.TrimPrefix(trimmed, "#VIDEO#:")}
			matched = true
		} else if strings.HasPrefix(trimmed, "#DOC#:") {
			item = MediaItem{Type: "DOC", Content: strings.TrimPrefix(trimmed, "#DOC#:")}
			matched = true
		} else if strings.HasPrefix(trimmed, "#BASE64_IMAGE#:") {
			item = MediaItem{Type: "BASE64_IMAGE", Content: strings.TrimPrefix(trimmed, "#BASE64_IMAGE#:")}
			matched = true
		}

		if matched {
			mediaItems = append(mediaItems, item)
		} else {
			textLines = append(textLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(textLines, "\n")), mediaItems
}

func (c *Client) sendMediaItems(chatID int64, items []MediaItem) error {
	var lastErr error
	for i, item := range items {
		if i > 0 {
			time.Sleep(500 * time.Millisecond) // Delay to prevent spamming/ordering issues
		}

		var err error
		switch item.Type {
		case "STICKER":
			err = c.sendLink(chatID, "sendSticker", "sticker", item.Content)
		case "IMAGE":
			err = c.sendPhotoInt64(chatID, item.Content, "")
		case "AUDIO":
			err = c.sendAudioInt64(chatID, item.Content, "")
		case "VIDEO":
			err = c.sendVideoInt64(chatID, item.Content, "")
		case "DOC":
			err = c.sendDocumentInt64(chatID, item.Content, "")
		case "BASE64_IMAGE":
			path, saveErr := utils.SaveBase64ToTempFile(item.Content, "telegram-image")
			if saveErr != nil {
				c.sendMessageInt64(chatID, fmt.Sprintf("Error processing base64 image: %v", saveErr))
				lastErr = saveErr
				continue
			}
			err = c.sendPhotoInt64(chatID, path, "")
			os.Remove(path) // Best effort cleanup
		}

		if err != nil {
			log.Printf("Error sending media item %d (%s): %v", i, item.Type, err)
			lastErr = err
		}
	}
	return lastErr
}

func (c *Client) sendPhotoInt64(chatID int64, media string, caption string) error {
	return c.sendMedia(chatID, "sendPhoto", "photo", media, caption)
}

func (c *Client) sendAudioInt64(chatID int64, media string, caption string) error {
	return c.sendMedia(chatID, "sendAudio", "audio", media, caption)
}

func (c *Client) sendVideoInt64(chatID int64, media string, caption string) error {
	return c.sendMedia(chatID, "sendVideo", "video", media, caption)
}

func (c *Client) sendDocumentInt64(chatID int64, media string, caption string) error {
	return c.sendMedia(chatID, "sendDocument", "document", media, caption)
}

// Legacy/Compatibility: StartPolling is kept but Start() is preferred for interface
// StartPolling is what Start() calls.

func (c *Client) sendLink(chatID int64, method, field, value string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.Token, method)
	body := map[string]interface{}{
		"chat_id": chatID,
		field:     value,
	}
	return c.postJSON(url, body)
}

func (c *Client) postJSON(url string, body map[string]interface{}) error {
	jsonBody, _ := json.Marshal(body)
	resp, err := c.HttpClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		// Return the actual error message from Telegram so caller can check for specific errors
		return fmt.Errorf("telegram api error:Status=%d Body=%s", resp.StatusCode, buf.String())
	}
	return nil
}

func (c *Client) sendMedia(chatID int64, method, field, media, caption string) error {
	// If it is a local file, upload it
	if utils.IsLocalFile(media) {
		return c.uploadFile(chatID, method, field, media, caption)
	}

	// Otherwise, send as URL
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.Token, method)
	body := map[string]interface{}{
		"chat_id": chatID,
		field:     media,
	}
	if caption != "" {
		body["caption"] = caption
	}
	return c.postJSON(url, body)
}

func (c *Client) uploadFile(chatID int64, method, field, path, caption string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.Token, method)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		log.Printf("Telegram API Error (Upload): %s", buf.String())
		return fmt.Errorf("telegram api returned status %d", resp.StatusCode)
	}

	return nil
}
