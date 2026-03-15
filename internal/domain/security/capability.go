package security

// Capability represents a specific permission required by a tool to execute.
type Capability string

const (
	CapReadFS           Capability = "fs:read"
	CapWriteFS          Capability = "fs:write"
	CapExecuteShell     Capability = "exec:shell"
	CapNetOutbound      Capability = "net:outbound"
	CapModifyInfra      Capability = "infra:modify"
	CapAccessKubernetes Capability = "k8s:access"
	CapAgentControl     Capability = "agent:control"
)
