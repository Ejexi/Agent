package domain

import (
	"time"
)

// ScannerType represents the type of security scanner
type ScannerType string

const (
	ScannerTypeSAST       ScannerType = "SAST"
	ScannerTypeDAST       ScannerType = "DAST"
	ScannerTypeSecrets    ScannerType = "SECRETS"
	ScannerTypeContainer  ScannerType = "CONTAINER"
	ScannerTypeDependency ScannerType = "DEPENDENCY"
	ScannerTypeIaC        ScannerType = "IAC"
)

// ScanStatus represents the current status of a scan
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "PENDING"
	ScanStatusRunning   ScanStatus = "RUNNING"
	ScanStatusCompleted ScanStatus = "COMPLETED"
	ScanStatusFailed    ScanStatus = "FAILED"
)

// Severity represents the severity level of a vulnerability
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

// Vulnerability represents a single security finding
type Vulnerability struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
	Location    string    `json:"location"` // e.g., file path, URL, package name
	Line        int       `json:"line,omitempty"`
	CVE         string    `json:"cve,omitempty"`
	CVSS        float64   `json:"cvss,omitempty"`
	Remediation string    `json:"remediation,omitempty"`
	References  []string  `json:"references,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
}

// ScanRequest is the message published to RabbitMQ to trigger a scan
type ScanRequest struct {
	ID          string            `json:"id"`           // Unique scan job ID (UUID)
	Target      string            `json:"target"`       // e.g., repo URL, image name, file path
	ScannerType ScannerType       `json:"scanner_type"` // Which worker should handle this
	Priority    int               `json:"priority,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"` // Extra context (branch, commit, env)
	RequestedAt time.Time         `json:"requested_at"`
	RequestedBy string            `json:"requested_by,omitempty"` // User or service that triggered the scan
}

// ScanResult is the message published back by a Scanner Worker after completing a scan
type ScanResult struct {
	ScanID          string          `json:"scan_id"` // References ScanRequest.ID
	ScannerType     ScannerType     `json:"scanner_type"`
	Status          ScanStatus      `json:"status"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Logs            []string        `json:"logs"` // Raw scanner output / debug info
	Summary         ScanSummary     `json:"summary"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     time.Time       `json:"completed_at"`
	Error           string          `json:"error,omitempty"` // Populated if Status == FAILED
}

// ScanSummary provides a quick count of findings by severity
type ScanSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

// ComputeSummary populates ScanSummary from the Vulnerabilities slice
func (r *ScanResult) ComputeSummary() {
	r.Summary = ScanSummary{}
	for _, v := range r.Vulnerabilities {
		r.Summary.Total++
		switch v.Severity {
		case SeverityCritical:
			r.Summary.Critical++
		case SeverityHigh:
			r.Summary.High++
		case SeverityMedium:
			r.Summary.Medium++
		case SeverityLow:
			r.Summary.Low++
		case SeverityInfo:
			r.Summary.Info++
		}
	}
}
