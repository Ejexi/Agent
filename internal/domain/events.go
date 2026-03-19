package domain

import (
	"github.com/SecDuckOps/shared/events"
)

// All scan domain types are now canonical in shared/events.
// These aliases keep existing agent code compiling without changes.

// ScannerType is an alias to the canonical shared type.
type ScannerType = events.ScannerType

// ScanStatus is an alias to the canonical shared type.
type ScanStatus = events.ScanStatus

// Severity is an alias to the canonical shared type.
type Severity = events.Severity

// Vulnerability is an alias to the canonical shared type.
type Vulnerability = events.Vulnerability

// ScanRequest is an alias to the canonical shared ScanRequestEvent.
type ScanRequest = events.ScanRequestEvent

// ScanResult is an alias to the canonical shared ScanResultEvent.
type ScanResult = events.ScanResultEvent

// ScanSummary is an alias to the canonical shared type.
type ScanSummary = events.ScanSummary

// Re-export ScannerType constants.
const (
	ScannerTypeSAST       = events.ScannerTypeSAST
	ScannerTypeDAST       = events.ScannerTypeDAST
	ScannerTypeSecrets    = events.ScannerTypeSecrets
	ScannerTypeContainer  = events.ScannerTypeContainer
	ScannerTypeDependency = events.ScannerTypeDependency
	ScannerTypeIaC        = events.ScannerTypeIaC
)

// Re-export ScanStatus constants.
const (
	ScanStatusPending   = events.ScanStatusPending
	ScanStatusRunning   = events.ScanStatusRunning
	ScanStatusCompleted = events.ScanStatusCompleted
	ScanStatusFailed    = events.ScanStatusFailed
)

// Re-export Severity constants.
const (
	SeverityCritical = events.SeverityCritical
	SeverityHigh     = events.SeverityHigh
	SeverityMedium   = events.SeverityMedium
	SeverityLow      = events.SeverityLow
	SeverityInfo     = events.SeverityInfo
)
