package web_search

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"golang.org/x/net/html"
)

type WebSearchParams struct {
	Query string `json:"query"`
}

type WebSearchTool struct {
	base.BaseTypedTool[WebSearchParams]
}

func NewWebSearchTool() *WebSearchTool {
	t := &WebSearchTool{}
	t.Impl = t
	return t
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "web_search",
		Description: `Search the web for up-to-date information, documentation, CVEs, or technical solutions.
Uses a free search engine (DuckDuckGo HTML) and requires NO API keys.`,
		Parameters: map[string]string{
			"query": "string (required) — the specific question, topic, or CVE to search for on the web",
		},
	}
}

func (t *WebSearchTool) ParseParams(input map[string]interface{}) (WebSearchParams, error) {
	return base.DefaultParseParams[WebSearchParams](input)
}

func (t *WebSearchTool) Execute(ctx context.Context, params WebSearchParams) (domain.Result, error) {
	if params.Query == "" {
		return domain.Result{Success: false, Error: "query cannot be empty"}, nil
	}
	// Security: Prevent massive queries that could be used for data exfiltration or DoS
	if len(params.Query) > 500 {
		params.Query = params.Query[:500]
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		// Optionally restrict redirects for additional safety:
		// CheckRedirect: func(req *http.Request, via []*http.Request) error {
		//     return http.ErrUseLastResponse // do not follow redirects
		// },
	}

	form := url.Values{}
	form.Add("q", params.Query)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return domain.Result{Success: false, Error: "failed to create request: " + err.Error()}, nil
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   "request failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.Result{
			Success: false,
			Error:   "http error",
			Data:    map[string]interface{}{"status_code": resp.StatusCode},
		}, nil
	}

	// Security: Verify content type is HTML to avoid parsing binary data.
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return domain.Result{
			Success: false,
			Error:   "unexpected content type: " + contentType,
		}, nil
	}

	// Security: Prevent Out-Of-Memory (OOM) by capping the response size to 2MB.
	limitReader := io.LimitReader(resp.Body, 2*1024*1024)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil && err != io.EOF {
		return domain.Result{Success: false, Error: "failed to read response: " + err.Error()}, nil
	}

	htmlContent := string(bodyBytes)
	cleaned := extractTextFromHTML(htmlContent)

	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 5000 {
		cleaned = cleaned[:5000] + "\n... (truncated)"
	}

	return domain.Result{
		Success: true,
		Status:  "search completed",
		Data: map[string]interface{}{
			"query":   params.Query,
			"results": cleaned,
		},
	}, nil
}

// extractTextFromHTML uses a proper HTML parser to extract readable text,
// avoiding regex-based parsing which can be vulnerable to ReDoS.
func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		// If parsing fails, return empty string to avoid exposing raw HTML.
		return ""
	}

	var extractText func(*html.Node, *strings.Builder)
	extractText = func(n *html.Node, sb *strings.Builder) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c, sb)
		}
	}

	var sb strings.Builder
	extractText(doc, &sb)
	text := sb.String()

	// Clean up whitespace: replace multiple spaces/tabs with a single space,
	// and collapse multiple newlines.
	text = strings.Join(strings.Fields(text), " ")
	text = strings.ReplaceAll(text, "\n", " ")
	// Restore newlines after sentence boundaries? For simplicity, keep as single line.

	// Security: Strip potentially dangerous control characters.
	var cleaned strings.Builder
	for _, r := range text {
		if r >= 0x20 && r != 0x7F { // printable ASCII range (excluding DEL)
			cleaned.WriteRune(r)
		} else if r == '\n' || r == '\t' {
			// Allow newline and tab to preserve some formatting
			cleaned.WriteRune(r)
		}
		// ignore other control characters
	}

	return cleaned.String()
}