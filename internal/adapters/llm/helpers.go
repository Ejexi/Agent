package llm

import "net/http"

// headerTransport is an http.RoundTripper that adds custom headers to every request.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// newHeaderTransport creates an http.RoundTripper that injects the given headers.
func newHeaderTransport(headers map[string]string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &headerTransport{
		base:    base,
		headers: headers,
	}
}
