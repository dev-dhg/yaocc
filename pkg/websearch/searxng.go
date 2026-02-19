package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

type SearxNGProvider struct {
	Endpoint   string
	MaxResults int
}

func NewSearxNGProvider(cfg config.SearchProvider) *SearxNGProvider {
	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	return &SearxNGProvider{
		Endpoint:   cfg.Endpoint,
		MaxResults: maxResults,
	}
}

func (p *SearxNGProvider) Search(query string) ([]SearchResult, error) {
	baseURL := strings.TrimSuffix(p.Endpoint, "/")
	endpoint := fmt.Sprintf("%s/search?format=json&q=%s", baseURL, url.QueryEscape(query))

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if errMsg, ok := data["error"]; ok {
		return nil, fmt.Errorf("API error: %v", errMsg)
	}

	var results []SearchResult
	if rawResults, ok := data["results"].([]interface{}); ok {
		limit := p.MaxResults
		if len(rawResults) < limit {
			limit = len(rawResults)
		}

		for i := 0; i < limit; i++ {
			item, ok := rawResults[i].(map[string]interface{})
			if !ok {
				continue
			}

			title, _ := item["title"].(string)
			content, _ := item["content"].(string)
			urlStr, _ := item["url"].(string)

			results = append(results, SearchResult{
				Title:   title,
				Snippet: content,
				Link:    urlStr,
			})
		}
	}

	return results, nil
}
