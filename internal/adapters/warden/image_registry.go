package warden

import "fmt"

// DefaultImageRegistry keeps track of supported scanners and their verified Docker images
var DefaultImageRegistry = map[string]string{
	"trivy":      "aquasec/trivy:latest",
	"semgrep":    "returntocorp/semgrep:latest",
	"gitleaks":   "zricethezav/gitleaks:latest",
	"bandit":     "cytopia/bandit:latest",
	"gosec":      "securego/gosec:latest",
	"trufflehog": "trufflesecurity/trufflehog:latest",
	"tfsec":           "aquasec/tfsec:latest",
	"terrascan":       "tenable/terrascan:latest",
	"zap":             "zaproxy/zap-stable:latest",
	"dependencycheck": "owasp/dependency-check:latest",
	"brakeman":        "presidentbeef/brakeman:latest",
	"kics":            "checkmarx/kics:latest",
	"grype":           "anchore/grype:latest",
	"nuclei":          "projectdiscovery/nuclei:latest",
	"osvscanner":      "ghcr.io/google/osv-scanner:latest",
	"tflint":          "ghcr.io/terraform-linters/tflint:latest",
	"detectsecrets":   "yelp/detect-secrets:latest",
}

// GetImageForScanner returns the default docker image for a given scanner name
func GetImageForScanner(scannerName string) (string, error) {
	if image, ok := DefaultImageRegistry[scannerName]; ok {
		return image, nil
	}
	return "", fmt.Errorf("no default image found for scanner: %s", scannerName)
}
