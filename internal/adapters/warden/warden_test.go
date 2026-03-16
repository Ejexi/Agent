package warden_test

import (
	"context"
	"testing"

	"github.com/SecDuckOps/agent/internal/adapters/warden"
	"github.com/SecDuckOps/agent/internal/domain/security"
)

func TestWarden_EvaluateAllowPolicy(t *testing.T) {
	w := warden.New(true, nil) // default deny

	policies := []security.NetworkPolicy{
		{
			ID:        "pol-1",
			Name:      "Allow GitHub API",
			CedarBody: `ALLOW url_contains "api.github.com"`,
			Priority:  100,
			Enabled:   true,
		},
	}

	ctx := context.Background()
	w.LoadPolicies(ctx, policies)

	decision, err := w.Evaluate(ctx, security.NetworkRequest{
		Method: "GET",
		URL:    "https://api.github.com/repos",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected request to be allowed")
	}
	if decision.PolicyID != "pol-1" {
		t.Errorf("expected policy pol-1, got %s", decision.PolicyID)
	}
}

func TestWarden_EvaluateDenyPolicy(t *testing.T) {
	w := warden.New(false, nil) // default allow

	policies := []security.NetworkPolicy{
		{
			ID:        "pol-2",
			Name:      "Block DELETE",
			CedarBody: `DENY method "DELETE"`,
			Priority:  100,
			Enabled:   true,
		},
	}

	ctx := context.Background()
	w.LoadPolicies(ctx, policies)

	decision, err := w.Evaluate(ctx, security.NetworkRequest{
		Method: "DELETE",
		URL:    "https://api.example.com/resource/123",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if decision.Allowed {
		t.Error("expected DELETE to be blocked")
	}
}

func TestWarden_DefaultDeny(t *testing.T) {
	w := warden.New(true, nil) // default deny

	ctx := context.Background()
	w.LoadPolicies(ctx, nil) // no policies

	decision, err := w.Evaluate(ctx, security.NetworkRequest{
		Method: "GET",
		URL:    "https://unknown-service.com/api",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if decision.Allowed {
		t.Error("expected default deny to block unknown requests")
	}
}

func TestWarden_DefaultAllow(t *testing.T) {
	w := warden.New(false, nil) // default allow

	ctx := context.Background()
	w.LoadPolicies(ctx, nil)

	decision, err := w.Evaluate(ctx, security.NetworkRequest{
		Method: "GET",
		URL:    "https://unknown-service.com/api",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected default allow to pass unknown requests")
	}
}

func TestWarden_MultipleRules(t *testing.T) {
	w := warden.New(true, nil)

	policies := []security.NetworkPolicy{
		{
			ID:   "pol-deny-delete",
			Name: "Block all DELETE",
			CedarBody: `DENY method "DELETE"
DENY method "PUT"`,
			Priority: 200,
			Enabled:  true,
		},
		{
			ID:        "pol-allow-github",
			Name:      "Allow GitHub",
			CedarBody: `ALLOW url_contains "github.com"`,
			Priority:  100,
			Enabled:   true,
		},
	}

	ctx := context.Background()
	w.LoadPolicies(ctx, policies)

	// DELETE to GitHub should be denied (higher priority policy)
	decision, _ := w.Evaluate(ctx, security.NetworkRequest{
		Method: "DELETE",
		URL:    "https://api.github.com/repos/test",
	})
	if decision.Allowed {
		t.Error("DELETE should be blocked by higher priority policy")
	}

	// GET to GitHub should be allowed
	decision, _ = w.Evaluate(ctx, security.NetworkRequest{
		Method: "GET",
		URL:    "https://api.github.com/repos/test",
	})
	if !decision.Allowed {
		t.Error("GET to GitHub should be allowed")
	}
}

func TestWarden_DisabledPolicy(t *testing.T) {
	w := warden.New(true, nil)

	policies := []security.NetworkPolicy{
		{
			ID:        "pol-disabled",
			Name:      "Disabled Allow All",
			CedarBody: `ALLOW all "*"`,
			Priority:  100,
			Enabled:   false, // disabled
		},
	}

	ctx := context.Background()
	w.LoadPolicies(ctx, policies)

	decision, _ := w.Evaluate(ctx, security.NetworkRequest{
		Method: "GET",
		URL:    "https://anything.com",
	})
	if decision.Allowed {
		t.Error("disabled policy should not match")
	}
}

func TestWarden_SourceToolFilter(t *testing.T) {
	w := warden.New(true, nil)

	policies := []security.NetworkPolicy{
		{
			ID:        "pol-scan-only",
			Name:      "Allow scan tool network",
			CedarBody: `ALLOW source_tool "scan_tool"`,
			Priority:  100,
			Enabled:   true,
		},
	}

	ctx := context.Background()
	w.LoadPolicies(ctx, policies)

	// scan_tool should be allowed
	decision, _ := w.Evaluate(ctx, security.NetworkRequest{
		Method:     "GET",
		URL:        "https://api.example.com",
		SourceTool: "scan_tool",
	})
	if !decision.Allowed {
		t.Error("scan_tool should be allowed")
	}

	// other tools should be denied
	decision, _ = w.Evaluate(ctx, security.NetworkRequest{
		Method:     "GET",
		URL:        "https://api.example.com",
		SourceTool: "echo",
	})
	if decision.Allowed {
		t.Error("echo tool should be denied (no matching policy)")
	}
}
