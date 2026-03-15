package engine

// ModelTier represents the cost/capability tier of an LLM model.
type ModelTier string

const (
	ModelTierLocal    ModelTier = "local"    // Local SLM (e.g., Llama 3 via Ollama) — free, fast
	ModelTierStandard ModelTier = "standard" // Standard cloud API (e.g., GPT-4o-mini)
	ModelTierFrontier ModelTier = "frontier" // Frontier model (e.g., GPT-4o, Claude 3.5 Sonnet)
)

// RoutingPolicy determines which LLM tier to use based on task characteristics.
type RoutingPolicy struct{}

// NewRoutingPolicy creates a new routing policy.
func NewRoutingPolicy() *RoutingPolicy {
	return &RoutingPolicy{}
}

// SelectModel determines the optimal model tier based on the route decision.
func (p *RoutingPolicy) SelectModel(decision RouteDecision) ModelTier {
	// Terminal commands never need LLM
	if decision.Intent == "terminal" {
		return ModelTierLocal
	}

	// High-confidence chat can use standard models
	if decision.Intent == "chat" && decision.Confidence >= ConfidenceMedium {
		return ModelTierStandard
	}

	// Orchestration and complex queries use frontier models
	if decision.Intent == "orchestration" {
		return ModelTierFrontier
	}

	// Ambiguous queries default to frontier for accuracy
	return ModelTierFrontier
}
