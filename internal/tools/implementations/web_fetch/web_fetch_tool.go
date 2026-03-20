// Package web_fetch provides a tool that fetches the content of a specific URL
// and returns extracted, readable text. Unlike web_search (which queries a
// search engine), web_fetch retrieves a page you already know the address of.
//
// Security guardrails:
//   - Blocks private/internal IP ranges (RFC-1918, loopback, link-local)
//   - Caps response body to 2 MB to prevent OOM
//   - Strips HTML tags, scripts, and styles before returning
//   - 15-second total timeout
package web_fetch

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
	"golang.org/x/net/html"
)

const (
	maxBodySize    = 2 * 1024 * 1024 // 2 MB
	maxOutputChars = 8000
	httpTimeout    = 15 * time.Second
)

// WebFetchParams are the LLM-provided parameters.
type WebFetchParams struct {
	// URL is the fully-qualified URL to fetch (must start with http:// or https://).
	URL string `json:"url"`
	// Prompt is an optional instruction for what to extract from the page.
	// e.g. "summarise the release notes" or "extract all CVE identifiers"
	Prompt string `json:"prompt,omitempty"`
}

// WebFetchTool retrieves the readable text content of a specific URL.
type WebFetchTool struct {
	base.BaseTypedTool[WebFetchParams]
}

func New() *WebFetchTool {
	t := &WebFetchTool{}
	t.Impl = t
	return t
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "web_fetch",
		Description: `Fetch and read the text content of a specific URL.
Use this when you already know the exact URL — for example a CVE detail page,
a library's changelog, or a security advisory. For general queries where you
don't know the URL, use web_search first.
Returns extracted readable text (HTML tags stripped). Max 8000 characters.`,
		Parameters: map[string]string{
			"url":    "string (required): fully-qualified URL starting with http:// or https://",
			"prompt": "string (optional): what to extract or focus on from the page",
		},
	}
}

func (t *WebFetchTool) ParseParams(input map[string]interface{}) (WebFetchParams, error) {
	return base.DefaultParseParams[WebFetchParams](input)
}

func (t *WebFetchTool) Execute(ctx context.Context, params WebFetchParams) (domain.Result, error) {
	if params.URL == "" {
		return domain.Result{
			Success: false,
			Error:   types.New(types.ErrCodeInvalidInput, "web_fetch: url is required").Error(),
		}, nil
	}

	// Validate URL scheme
	parsed, err := url.Parse(params.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return domain.Result{
			Success: false,
			Error:   types.Newf(types.ErrCodeInvalidInput, "web_fetch: url must start with http:// or https://").Error(),
		}, nil
	}

	// Block private/internal addresses (SSRF protection)
	if err := checkSSRF(parsed.Host); err != nil {
		return domain.Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Fetch
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return types.New(types.ErrCodeExecutionFailed, "web_fetch: stopped after 5 redirects")
			}
			// Re-check SSRF on redirect target
			return checkSSRF(req.URL.Host)
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.URL, nil)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   types.Wrap(err, types.ErrCodeInternal, "web_fetch: failed to build request").Error(),
		}, nil
	}
	req.Header.Set("User-Agent", "DuckOps-Agent/1.0 (security research tool)")
	req.Header.Set("Accept", "text/html,text/plain,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   types.Wrap(err, types.ErrCodeExecutionFailed, "web_fetch: request failed").Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return domain.Result{
			Success: false,
			Error:   types.Newf(types.ErrCodeExecutionFailed, "web_fetch: HTTP %d from %s", resp.StatusCode, params.URL).Error(),
		}, nil
	}

	// Read body — capped at maxBodySize
	limitedBody := io.LimitReader(resp.Body, maxBodySize)
	bodyBytes, err := io.ReadAll(limitedBody)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   types.Wrap(err, types.ErrCodeInternal, "web_fetch: failed to read response body").Error(),
		}, nil
	}

	// Extract readable text
	ct := resp.Header.Get("Content-Type")
	var text string
	if strings.Contains(ct, "text/html") || ct == "" {
		text = extractText(string(bodyBytes))
	} else {
		// Plain text or other — return as-is
		text = string(bodyBytes)
	}

	text = strings.TrimSpace(text)
	truncated := false
	if len([]rune(text)) > maxOutputChars {
		runes := []rune(text)
		text = string(runes[:maxOutputChars]) + "\n\n… (truncated — page has more content)"
		truncated = true
	}

	result := domain.Result{
		Success: true,
		Data: map[string]interface{}{
			"url":       params.URL,
			"content":   text,
			"truncated": truncated,
			"chars":     len([]rune(text)),
		},
	}
	if params.Prompt != "" {
		result.Data["prompt"] = params.Prompt
	}
	return result, nil
}

// ── SSRF guard ────────────────────────────────────────────────────────────────

// checkSSRF returns an error if the host resolves to a private/internal address.
func checkSSRF(host string) error {
	// Strip port if present
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// DNS failure — allow (don't block on DNS errors, let HTTP handle it)
		return nil
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivate(ip) {
			return types.Newf(types.ErrCodeSecurityViolation,
				"web_fetch: blocked — %s resolves to a private/internal address (%s)", hostname, addr)
		}
	}
	return nil
}

// isPrivate returns true for RFC-1918, loopback, and link-local addresses.
func isPrivate(ip net.IP) bool {
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"169.254.0.0/16", // link-local
		"fd00::/8",       // ULA
	}
	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// ── HTML text extractor ───────────────────────────────────────────────────────

// skipTags are elements whose subtrees we skip entirely.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true,
	"nav": true, "footer": true, "header": true,
	"iframe": true, "svg": true, "canvas": true,
}

func extractText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Collapse multiple blank lines
	lines := strings.Split(sb.String(), "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
		} else {
			blank = 0
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
