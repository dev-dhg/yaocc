package websearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

type PerplexityProvider struct {
	Name         string
	APIKey       string
	MaxResults   int
	Fallback     string
	AllProviders map[string]config.SearchProvider
	TempDir      string
}

func NewPerplexityProvider(name string, cfg config.SearchProvider, allProviders map[string]config.SearchProvider, tempDir string) (*PerplexityProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("perplexity provider '%s' requires an API key", name)
	}
	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	return &PerplexityProvider{
		Name:         name,
		APIKey:       cfg.APIKey,
		MaxResults:   maxResults,
		Fallback:     cfg.Fallback,
		AllProviders: allProviders,
		TempDir:      tempDir,
	}, nil
}

func (p *PerplexityProvider) Search(query string) ([]SearchResult, error) {
	results, err := p.performSearch(query)
	if err != nil {
		if p.Fallback != "" {
			fmt.Printf("Perplexity search failed: %v. Using fallback provider: %s\n", err, p.Fallback)
			fallbackCfg, ok := p.AllProviders[p.Fallback]
			if !ok {
				return nil, fmt.Errorf("search failed and fallback provider '%s' not found: %w", p.Fallback, err)
			}
			fallbackProvider, err := NewProvider(p.Fallback, fallbackCfg, p.AllProviders, p.TempDir)
			if err != nil {
				return nil, fmt.Errorf("failed to create fallback provider: %w", err)
			}
			return fallbackProvider.Search(query)
		}
		return nil, err
	}
	return results, nil
}

func (p *PerplexityProvider) performSearch(query string) ([]SearchResult, error) {
	url := "https://api.perplexity.ai/search"

	payload := map[string]interface{}{
		"query":       query,
		"max_results": p.MaxResults,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+p.APIKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Perplexity response structure:
	// { "results": [ { "title": "...", "url": "...", "snippet": "..." } ] }
	var results []SearchResult
	if rawResults, ok := data["results"].([]interface{}); ok {
		// MaxResults is already handled by API, but we can double check
		for _, rawItem := range rawResults {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}

			title, _ := item["title"].(string)
			snippet, _ := item["snippet"].(string)
			urlStr, _ := item["url"].(string)

			results = append(results, SearchResult{
				Title:   title,
				Snippet: snippet,
				Link:    urlStr,
			})
		}
	}

	return results, nil
}
