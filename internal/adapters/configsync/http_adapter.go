package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
)

type HTTPAdapter struct {
	client     *http.Client
	gatewayURL string
	apiKey     string
}

func NewHTTPAdapter(gatewayURL, apiKey string) *HTTPAdapter {
	return &HTTPAdapter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		gatewayURL: gatewayURL,
		apiKey:     apiKey,
	}
}

func (a *HTTPAdapter) FetchRemoteConfig(ctx context.Context) (*ports.RemoteConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/config/sync", a.gatewayURL), nil)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to create config sync request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to execute config sync request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, types.New(types.ErrCodeInternal, fmt.Sprintf("config sync failed with status: %d", resp.StatusCode))
	}

	var config ports.RemoteConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to decode remote config")
	}

	return &config, nil
}
