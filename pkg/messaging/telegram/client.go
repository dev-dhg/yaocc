package telegram

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/llm"
	"github.com/dev-dhg/yaocc/pkg/utils"
)

type Client struct {
	Token        string
	AllowedUsers []string
	Agent        *agent.Agent
	Offset       int
	HttpClient   *http.Client
	BotUsername  string
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
		c.BotUsername = me.Username
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

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", "channel"
}

type PhotoSize struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size"`
}

type Voice struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
}

type Audio struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
}

type MessageEntity struct {
	Type   string `json:"type"` // "mention", "text_mention", "bot_command"
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID       int             `json:"message_id"`
	Chat            Chat            `json:"chat"`
	From            User            `json:"from"`
	Text            string          `json:"text"`
	Caption         string          `json:"caption"`
	Photo           []PhotoSize     `json:"photo"`
	Voice           *Voice          `json:"voice"`
	Audio           *Audio          `json:"audio"`
	Entities        []MessageEntity `json:"entities"`
	CaptionEntities []MessageEntity `json:"caption_entities"`
	ReplyToMessage  *Message        `json:"reply_to_message"`
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
	if update.Message == nil {
		return
	}

	msg := update.Message

	// Extract prompt text
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	userID := strconv.FormatInt(msg.From.ID, 10)
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

	// Command parsing
	if text != "" {
		trimmedText := strings.TrimSpace(text)
		if strings.HasPrefix(trimmedText, "/") {
			// Extract command name
			fields := strings.Fields(trimmedText)
			cmdName := fields[0]
			// Strip bot username suffix if present (e.g. /history-clear@botname -> /history-clear)
			if strings.Contains(cmdName, "@") {
				parts := strings.SplitN(cmdName, "@", 2)
				cmdName = parts[0]
			}

			chatID := msg.Chat.ID
			sessionID := fmt.Sprintf("telegram-%d", chatID)

			switch cmdName {
			case "/history-clear":
				log.Printf("Clearing chat history for session %s via /history-clear command", sessionID)
				err := c.Agent.Sessions.Clear(sessionID)
				if err != nil {
					log.Printf("Error clearing chat history: %v", err)
					c.sendMessageInt64(chatID, "❌ <b>Error:</b> Failed to clear history.")
				} else {
					c.sendMessageInt64(chatID, "🧹 <b>Chat history cleared!</b> Your session is now fresh and clean.")
				}
				return

			case "/memory-clear":
				log.Printf("Clearing long-term memory for agent via /memory-clear command")
				err := c.Agent.ClearMemory()
				if err != nil {
					log.Printf("Error clearing memory: %v", err)
					c.sendMessageInt64(chatID, "❌ <b>Error:</b> Failed to clear memory.")
				} else {
					c.sendMessageInt64(chatID, "🧠 <b>Long-term memory cleared!</b>")
				}
				return

			case "/memory-clear-all":
				log.Printf("Clearing all memory files for agent via /memory-clear-all command")
				err := c.Agent.ClearMemoryAll()
				if err != nil {
					log.Printf("Error clearing all memories: %v", err)
					c.sendMessageInt64(chatID, "❌ <b>Error:</b> Failed to clear all memories.")
				} else {
					c.sendMessageInt64(chatID, "🧠💥 <b>All long-term and daily memory contexts cleared!</b>")
				}
				return

			case "/list", "/help":
				log.Printf("Listing all available commands via %s command", cmdName)
				helpText := "<b>Available Commands:</b>\n" +
					"• <code>/history-clear</code> — Wipes all chat history and summaries for this session.\n" +
					"• <code>/memory-clear</code> — Clears long-term memory (MEMORY.md).\n" +
					"• <code>/memory-clear-all</code> — Clears long-term memory and all daily files inside memory/ folder.\n" +
					"• <code>/list</code> — Lists all available commands."
				c.sendMessageInt64(chatID, helpText)
				return
			}
		}
	}


	isPrivate := msg.Chat.Type == "private" || msg.Chat.Type == ""
	isMentioned := c.isMentioned(msg)
	isReplyToBot := c.isReplyToBot(msg)

	// In groups/supergroups, only respond if mentioned or replied to
	if !isPrivate && !isMentioned && !isReplyToBot {
		return
	}

	// Remove bot mention from prompt text if in a group
	if !isPrivate {
		text = c.removeMention(text)
	}

	// Fetch active model capabilities to verify if vision/audio is supported
	m := c.Agent.GetCurrentModel()
	hasVision := m != nil && m.SupportsVision()
	hasAudio := m != nil && m.SupportsAudio()

	var attachments []llm.Attachment

	// 1. Check for Photo Attachments
	if len(msg.Photo) > 0 {
		if !hasVision {
			text = "[Image message skipped: active model does not support vision]\n" + text
			log.Printf("Skipping photo: active model %s does not support vision", c.Agent.Config.Models.Selected)
		} else {
			// Get largest photo (last element)
			photo := msg.Photo[len(msg.Photo)-1]
			log.Printf("Downloading photo %s...", photo.FileID)
			base64Data, err := c.DownloadFileBase64(photo.FileID)
			if err != nil {
				log.Printf("Error downloading photo: %v", err)
				text = "[Error downloading image from Telegram]\n" + text
			} else {
				attachments = append(attachments, llm.Attachment{
					Type: "image",
					Data: base64Data,
					MIME: "image/jpeg",
				})
			}
		}
	}

	// 2. Check for Voice messages
	if msg.Voice != nil {
		if !hasAudio {
			text = "[Voice message skipped: active model does not support audio]\n" + text
			log.Printf("Skipping voice: active model %s does not support audio", c.Agent.Config.Models.Selected)
		} else {
			log.Printf("Downloading voice message %s...", msg.Voice.FileID)
			base64Data, err := c.DownloadFileBase64(msg.Voice.FileID)
			if err != nil {
				log.Printf("Error downloading voice: %v", err)
				text = "[Error downloading voice message from Telegram]\n" + text
			} else {
				mime := msg.Voice.MimeType
				if mime == "" {
					mime = "audio/ogg"
				}
				attachments = append(attachments, llm.Attachment{
					Type:   "audio",
					Data:   base64Data,
					MIME:   mime,
					Format: "ogg",
				})
			}
		}
	}

	// 3. Check for Audio files
	if msg.Audio != nil {
		if !hasAudio {
			text = "[Audio file skipped: active model does not support audio]\n" + text
			log.Printf("Skipping audio file: active model %s does not support audio", c.Agent.Config.Models.Selected)
		} else {
			log.Printf("Downloading audio file %s...", msg.Audio.FileID)
			base64Data, err := c.DownloadFileBase64(msg.Audio.FileID)
			if err != nil {
				log.Printf("Error downloading audio: %v", err)
				text = "[Error downloading audio file from Telegram]\n" + text
			} else {
				mime := msg.Audio.MimeType
				if mime == "" {
					mime = "audio/mpeg"
				}
				attachments = append(attachments, llm.Attachment{
					Type:   "audio",
					Data:   base64Data,
					MIME:   mime,
					Format: "mp3",
				})
			}
		}
	}

	// Set default prompt if media was sent with no text
	if text == "" && len(attachments) > 0 {
		if attachments[0].Type == "image" {
			text = "Describe this image."
		} else if attachments[0].Type == "audio" {
			text = "Listen to this audio and respond."
		}
	}

	// If no text prompt and no attachments, nothing to do
	if text == "" && len(attachments) == 0 {
		return
	}

	// Handle Reply context - include replied message content
	if msg.ReplyToMessage != nil {
		replied := msg.ReplyToMessage
		repliedText := replied.Text
		if repliedText == "" {
			repliedText = replied.Caption
		}
		if repliedText == "" {
			if len(replied.Photo) > 0 {
				repliedText = "(media: photo)"
			} else if replied.Voice != nil {
				repliedText = "(media: voice)"
			} else if replied.Audio != nil {
				repliedText = "(media: audio)"
			} else {
				repliedText = "(media)"
			}
		}
		text = fmt.Sprintf("[Replying to %s: \"%s\"]\n%s", replied.From.Username, repliedText, text)
	}

	chatID := msg.Chat.ID
	sessionID := fmt.Sprintf("telegram-%d", chatID)

	log.Printf("Received message from %s: %s (attachments: %d)", sessionID, text, len(attachments))

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

	// Process with Agent, passing any base64 attachments
	response, err := c.Agent.Run(sessionID, c, strconv.FormatInt(chatID, 10), text, attachments...)
	if err != nil {
		log.Printf("Agent error: %v", err)
		c.sendMessageInt64(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	c.sendMessageInt64(chatID, response)
}

func (c *Client) isMentioned(msg *Message) bool {
	if c.BotUsername == "" {
		return false
	}
	botMention := "@" + strings.ToLower(c.BotUsername)

	// Check text entities
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			end := entity.Offset + entity.Length
			if end <= len(msg.Text) {
				mention := strings.ToLower(msg.Text[entity.Offset:end])
				if mention == botMention {
					return true
				}
			}
		}
	}

	// Check caption entities
	for _, entity := range msg.CaptionEntities {
		if entity.Type == "mention" {
			end := entity.Offset + entity.Length
			if end <= len(msg.Caption) {
				mention := strings.ToLower(msg.Caption[entity.Offset:end])
				if mention == botMention {
					return true
				}
			}
		}
	}

	// Fallback substring checks
	if strings.Contains(strings.ToLower(msg.Text), botMention) ||
		strings.Contains(strings.ToLower(msg.Caption), botMention) {
		return true
	}

	return false
}

func (c *Client) isReplyToBot(msg *Message) bool {
	if msg.ReplyToMessage == nil || c.BotUsername == "" {
		return false
	}
	return strings.ToLower(msg.ReplyToMessage.From.Username) == strings.ToLower(c.BotUsername)
}

func (c *Client) removeMention(text string) string {
	if c.BotUsername == "" {
		return text
	}
	botMention := "@" + c.BotUsername
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(botMention))
	text = re.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func (c *Client) DownloadFileBase64(fileID string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", c.Token, fileID)
	resp, err := c.HttpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Ok     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Ok || result.Result.FilePath == "" {
		return "", fmt.Errorf("failed to get file path from telegram")
	}

	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.Token, result.Result.FilePath)
	fileResp, err := c.HttpClient.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer fileResp.Body.Close()

	if fileResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file, status: %d", fileResp.StatusCode)
	}

	fileBytes, err := io.ReadAll(fileResp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(fileBytes), nil
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
You are talking to a user via Telegram. 
IMPORTANT: You MUST use HTML formatting exclusively. DO NOT use Markdown syntax (like # headers or [text](url)).

### Supported HTML Tags:
- <b>bold</b>, <strong>bold</strong>
- <i>italic</i>, <em>italic</em>
- <u>underline</u>, <ins>underline</ins>
- <s>strikethrough</s>, <strike>strikethrough</strike>, <del>strikethrough</del>
- <tg-spoiler>spoiler</tg-spoiler>, <span class="tg-spoiler">spoiler</span>
- <a href="http://www.example.com/">inline URL</a>
- <a href="tg://user?id=123456789">inline mention of a user</a>
- <tg-emoji emoji-id="5368324170671202286">👍</tg-emoji> (requires a valid emoji-id)
- <tg-time unix="1647531900" format="wDT">22:45 tomorrow</tg-time> (formats: r, w, d, D, t, T)
- <code>inline fixed-width code</code>
- <pre>pre-formatted fixed-width code block</pre>
- <pre><code class="language-python">code block with language</code></pre>
- <blockquote>Block quotation</blockquote>
- <blockquote expandable>Expandable block quotation</blockquote>

### Rules for Rich Formatting:
1. **Nesting**: You can nest tags (e.g., <b>bold <i>italic</i></b>).
2. **Headlines**: Use <b>BOLD TEXT</b> for headlines and section titles. Do NOT use '#' headers.
3. **Tables**: Telegram does NOT support tables. Represent tables using ASCII formatting (using | and -) wrapped inside a <pre> or <blockquote> block to ensure fixed-width alignment.
4. **Escaping**: You MUST escape all <, >, and & characters that are NOT part of a tag. Use &lt;, &gt;, and &amp; respectively.
5. **Horizontal Rules**: Use ——— (long dashes) or * * * to separate sections.
6. **Lists**: Use standard bullet points (• or -).
`
}

func (c *Client) sanitizeTelegramHTML(value string) string {
	// Remove comments
	reComment := regexp.MustCompile(`(?s)<!--.*?-->`)
	value = reComment.ReplaceAllString(value, "")

	allowedTags := map[string]bool{
		"a": true, "b": true, "strong": true, "i": true, "em": true, "u": true,
		"ins": true, "s": true, "strike": true, "del": true, "code": true,
		"pre": true, "blockquote": true, "tg-spoiler": true, "tg-emoji": true,
	}

	newlineTags := map[string]bool{
		"br": true, "p": true, "div": true, "li": true,
	}

	// Match HTML tags
	reTag := regexp.MustCompile(`(?i)</?([a-z0-9-]+)\b[^>]*\/?>`)
	value = reTag.ReplaceAllStringFunc(value, func(tag string) string {
		match := reTag.FindStringSubmatch(tag)
		if len(match) > 1 {
			name := strings.ToLower(match[1])
			if newlineTags[name] {
				return "\n"
			}
			if allowedTags[name] {
				return tag
			}
			return ""
		}
		return ""
	})

	value = strings.ReplaceAll(value, "\r\n", "\n")
	reLines := regexp.MustCompile(`\n{3,}`)
	value = reLines.ReplaceAllString(value, "\n\n")

	return value
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

			// Sanitize chunk to strip unsupported tags before sending
			sanitizedChunk := c.sanitizeTelegramHTML(chunk)

			url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.Token)
			body := map[string]interface{}{
				"chat_id":    chatID,
				"text":       sanitizedChunk,
				"parse_mode": "HTML",
			}

			err := c.postJSON(url, body)
			if err != nil {
				// Check if error is related to HTML tag parsing
				// Telegram error for bad HTML usually contains "can't parse entities" or "Bad Request: can't parse HTML"
				if strings.Contains(err.Error(), "can't parse entities") || strings.Contains(err.Error(), "can't parse HTML") {
					log.Printf("HTML parsing failed (%v) for chunk %d/%d, retrying as raw plain text...", err, i+1, len(chunks))

					// Retry with RAW text and NO parse_mode
					body["text"] = sanitizedChunk
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
		body["parse_mode"] = "HTML"
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
		writer.WriteField("parse_mode", "HTML")
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
