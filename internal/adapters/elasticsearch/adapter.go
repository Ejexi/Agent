package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"duckops/internal/ports"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// Adapter implements ports.LogDB using Elasticsearch.
// Optimized for high-volume, append-heavy log ingestion and full-text search.
type Adapter struct {
	client *elasticsearch.Client
	index  string // Base index name (e.g., "duckops-scan-logs")
}

// Config holds connection parameters for the Elasticsearch adapter.
type Config struct {
	Addresses []string // e.g., ["http://localhost:9200"]
	Username  string
	Password  string
	Index     string // Base index name
}

// NewAdapter creates a new Elasticsearch adapter and verifies connectivity.
func NewAdapter(cfg Config) (*Adapter, error) {
	index := cfg.Index
	if index == "" {
		index = "duckops-scan-logs"
	}

	esCfg := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: failed to create client: %w", err)
	}

	// Verify connectivity
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: failed to connect: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch: server error: %s", res.String())
	}

	return &Adapter{
		client: client,
		index:  index,
	}, nil
}

// StoreLogs persists raw scan output logs associated with a scan ID.
// Each log line is stored as a separate document for granular search.
func (a *Adapter) StoreLogs(ctx context.Context, scanID string, logs []string) error {
	for _, line := range logs {
		entry := ports.LogEntry{
			ScanID:    scanID,
			Line:      line,
			Timestamp: time.Now(),
			Level:     detectLogLevel(line),
		}

		body, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("elasticsearch: marshal log entry: %w", err)
		}

		req := esapi.IndexRequest{
			Index:   a.index,
			Body:    bytes.NewReader(body),
			Refresh: "false", // Async refresh for throughput
		}

		res, err := req.Do(ctx, a.client)
		if err != nil {
			return fmt.Errorf("elasticsearch: index log entry: %w", err)
		}
		defer res.Body.Close()

		if res.IsError() {
			return fmt.Errorf("elasticsearch: index error: %s", res.String())
		}
	}

	return nil
}

// GetLogs retrieves all stored logs for a given scan ID.
func (a *Adapter) GetLogs(ctx context.Context, scanID string) ([]ports.LogEntry, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"scan_id": scanID,
			},
		},
		"sort": []map[string]interface{}{
			{"timestamp": map[string]string{"order": "asc"}},
		},
		"size": 10000,
	}

	return a.executeSearch(ctx, query)
}

// SearchLogs performs a full-text search across stored scan logs.
func (a *Adapter) SearchLogs(ctx context.Context, q ports.LogSearchQuery) ([]ports.LogEntry, error) {
	must := []map[string]interface{}{}

	// Full-text search on the log line content
	if q.Text != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{
				"line": q.Text,
			},
		})
	}

	// Filter by scan ID
	if q.ScanID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"scan_id": q.ScanID,
			},
		})
	}

	// Filter by log level
	if q.Level != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"level": q.Level,
			},
		})
	}

	// Time range filter
	if q.From != nil || q.To != nil {
		rangeQuery := map[string]interface{}{}
		if q.From != nil {
			rangeQuery["gte"] = q.From.Format(time.RFC3339)
		}
		if q.To != nil {
			rangeQuery["lte"] = q.To.Format(time.RFC3339)
		}
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": rangeQuery,
			},
		})
	}

	size := q.Limit
	if size <= 0 {
		size = 100
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"sort": []map[string]interface{}{
			{"timestamp": map[string]string{"order": "asc"}},
		},
		"size": size,
		"from": q.Offset,
	}

	return a.executeSearch(ctx, query)
}

// DeleteLogs removes all log entries for a given scan ID.
func (a *Adapter) DeleteLogs(ctx context.Context, scanID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"scan_id": scanID,
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("elasticsearch: marshal delete query: %w", err)
	}

	req := esapi.DeleteByQueryRequest{
		Index: []string{a.index},
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(ctx, a.client)
	if err != nil {
		return fmt.Errorf("elasticsearch: delete by query: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch: delete error: %s", res.String())
	}

	return nil
}

// Close is a no-op for elasticsearch (the HTTP client doesn't need explicit closing).
func (a *Adapter) Close() error {
	return nil
}

// executeSearch is a helper that runs an ES query and deserializes hits into LogEntry slices.
func (a *Adapter) executeSearch(ctx context.Context, query map[string]interface{}) ([]ports.LogEntry, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: marshal query: %w", err)
	}

	res, err := a.client.Search(
		a.client.Search.WithContext(ctx),
		a.client.Search.WithIndex(a.index),
		a.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch: search error: %s", res.String())
	}

	var esResp struct {
		Hits struct {
			Hits []struct {
				Source ports.LogEntry `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("elasticsearch: decode response: %w", err)
	}

	entries := make([]ports.LogEntry, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		entries = append(entries, hit.Source)
	}

	return entries, nil
}

// detectLogLevel is a simple heuristic to extract log level from a log line.
func detectLogLevel(line string) string {
	for _, level := range []string{"ERROR", "WARN", "INFO", "DEBUG"} {
		if len(line) > len(level)+2 {
			for i := 0; i <= len(line)-len(level); i++ {
				if line[i:i+len(level)] == level {
					return level
				}
			}
		}
	}
	return "INFO"
}
