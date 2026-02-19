package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

const braveUsageFile = "brave_usage.json"
const braveFreeTierLimit = 2000

type BraveProvider struct {
	Name         string
	APIKey       string
	FreeTier     config.FreeTierConfig
	AllProviders map[string]config.SearchProvider
	TempDir      string
	MaxResults   int
}

type BraveUsage struct {
	LastRequest time.Time `json:"lastRequest"`
	Count       int       `json:"count"`
}

func NewBraveProvider(name string, cfg config.SearchProvider, allProviders map[string]config.SearchProvider, tempDir string) (*BraveProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("brave provider '%s' requires an API key", name)
	}
	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	return &BraveProvider{
		Name:         name,
		APIKey:       cfg.APIKey,
		FreeTier:     cfg.FreeTier,
		AllProviders: allProviders,
		TempDir:      tempDir,
		MaxResults:   maxResults,
	}, nil
}

func (p *BraveProvider) Search(query string) ([]SearchResult, error) {
	if p.FreeTier.Enabled {
		allowed, err := p.checkMonthlyQuota()
		if err != nil {
			return nil, fmt.Errorf("failed to check rate limit: %w", err)
		}

		if !allowed {
			if p.FreeTier.Fallback != "" {
				fallbackCfg, ok := p.AllProviders[p.FreeTier.Fallback]
				if !ok {
					return nil, fmt.Errorf("fallback provider '%s' not found", p.FreeTier.Fallback)
				}
				// create fallback provider
				fallbackProvider, err := NewProvider(p.FreeTier.Fallback, fallbackCfg, p.AllProviders, p.TempDir)
				if err != nil {
					return nil, fmt.Errorf("failed to create fallback provider: %w", err)
				}
				return fallbackProvider.Search(query)
			}
			return nil, fmt.Errorf("brave free tier limit reached and no fallback configured")
		}

		// Enforce 1 RPS limit (Wait if needed)
		if err := p.enforceOneRequestPerSecond(); err != nil {
			// Log error but proceed? Or fail?
			// Proceeding seems safer as file read error shouldn't block search if API might work.
			// But let's log it.
			fmt.Printf("Warning: failed to enforce rate limit delay: %v\n", err)
		}
	}

	results, err := p.performSearch(query)
	if err != nil {
		return nil, err
	}

	if p.FreeTier.Enabled {
		if err := p.incrementUsage(); err != nil {
			// Log error but don't fail search? For now print to stdout/stderr or ignore
			fmt.Printf("Warning: failed to update usage: %v\n", err)
		}
	}

	return results, nil
}

func (p *BraveProvider) performSearch(query string) ([]SearchResult, error) {
	endpoint := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d", url.QueryEscape(query), p.MaxResults)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Subscription-Token", p.APIKey)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	// Go's http client handles gzip transparently if we don't set the Accept-Encoding header manually.

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Brave API response structure
	// We want data.web.results
	webData, ok := data["web"].(map[string]interface{})
	if !ok {
		// handle case where no web results
		return []SearchResult{}, nil
	}

	var results []SearchResult
	if rawResults, ok := webData["results"].([]interface{}); ok {
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
			// Brave might have different field for snippet. "description"?
			// Example says "description".
			content, _ := item["description"].(string)
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

func (p *BraveProvider) getUsageFilePath() string {
	// If we want one file per provider instance, we could include name.
	// But user said "brave.json", and monthly limit is usually per token/account.
	// Let's assume one global "brave_usage.json" for now, or use name if multiple braves?
	// The requirement said "brave.json or something".
	return filepath.Join(p.TempDir, braveUsageFile)
}

func (p *BraveProvider) checkMonthlyQuota() (bool, error) {
	usagePath := p.getUsageFilePath()
	if _, err := os.Stat(usagePath); os.IsNotExist(err) {
		return true, nil
	}

	data, err := os.ReadFile(usagePath)
	if err != nil {
		return false, err
	}

	var usage BraveUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		// if corrupted, maybe reset? or fail.
		return false, err
	}

	now := time.Now()
	// Check if new month
	if now.Month() != usage.LastRequest.Month() || now.Year() != usage.LastRequest.Year() {
		return true, nil // Should reset on next update
	}

	if usage.Count >= braveFreeTierLimit {
		return false, nil
	}

	return true, nil
}

func (p *BraveProvider) incrementUsage() error {
	usagePath := p.getUsageFilePath()
	now := time.Now()

	var usage BraveUsage

	if _, err := os.Stat(usagePath); err == nil {
		data, err := os.ReadFile(usagePath)
		if err == nil {
			_ = json.Unmarshal(data, &usage)
		}
	}

	// Check reset
	if now.Month() != usage.LastRequest.Month() || now.Year() != usage.LastRequest.Year() {
		usage.Count = 0
	}

	usage.Count++
	usage.LastRequest = now

	// Write back
	data, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		return err
	}

	// Ensure temp dir exists
	if err := os.MkdirAll(p.TempDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(usagePath, data, 0644)
}

func (p *BraveProvider) enforceOneRequestPerSecond() error {
	usagePath := p.getUsageFilePath()
	if _, err := os.Stat(usagePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(usagePath)
	if err != nil {
		return err
	}

	var usage BraveUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return err
	}

	const requiredWait = 1100 * time.Millisecond
	elapsed := time.Since(usage.LastRequest)
	if elapsed < requiredWait {
		sleepDuration := requiredWait - elapsed
		time.Sleep(sleepDuration)
	}

	return nil
}
