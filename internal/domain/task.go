package domain

import "github.com/SecDuckOps/agent/internal/domain/security"

type Task struct {
	ID           string                `json:"id"`
	SessionID    string                `json:"session_id"`
	Tool         string                `json:"tool"`
	Args         map[string]interface{} `json:"args,omitempty"`
	RequiredCaps []security.Capability `json:"required_caps,omitempty"`
}
