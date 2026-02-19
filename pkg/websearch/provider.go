package websearch

import (
	"fmt"

	"github.com/dev-dhg/yaocc/pkg/config"
)

type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
}

type Provider interface {
	Search(query string) ([]SearchResult, error)
}

func NewProvider(name string, cfg config.SearchProvider, allProviders map[string]config.SearchProvider, tempDir string) (Provider, error) {
	switch cfg.Type {
	case "searxng":
		return NewSearxNGProvider(cfg), nil
	case "brave":
		return NewBraveProvider(name, cfg, allProviders, tempDir)
	case "perplexity":
		return NewPerplexityProvider(name, cfg, allProviders, tempDir)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}
