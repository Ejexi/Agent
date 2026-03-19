package agent

import (
	"testing"
)

func TestParseIntent_ScanAll(t *testing.T) {
	intent := ParseIntent("scan this project")
	if intent.Action != "scan" {
		t.Errorf("expected action=scan, got %s", intent.Action)
	}
	if intent.Target != "." {
		t.Errorf("expected target=., got %s", intent.Target)
	}
	if len(intent.Categories) != 0 {
		t.Errorf("expected no categories (= all), got %v", intent.Categories)
	}
}

func TestParseIntent_SecretsOnly(t *testing.T) {
	intent := ParseIntent("check for hardcoded secrets")
	if intent.Action != "scan" {
		t.Errorf("expected action=scan, got %s", intent.Action)
	}
	found := false
	for _, c := range intent.Categories {
		if c == "secrets" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected secrets in categories, got %v", intent.Categories)
	}
}

func TestParseIntent_TargetPath(t *testing.T) {
	intent := ParseIntent("scan the src folder")
	if intent.Target != "src" && intent.Target != "./src" {
		t.Errorf("expected target=src, got %s", intent.Target)
	}
}

func TestParseIntent_ExplicitPath(t *testing.T) {
	intent := ParseIntent("scan ./backend for vulnerabilities")
	if intent.Target != "backend" && intent.Target != "./backend" {
		t.Errorf("expected target=./backend, got %s", intent.Target)
	}
}

func TestParseIntent_CriticalOnly(t *testing.T) {
	intent := ParseIntent("show me only critical issues")
	if intent.Severity != "critical" {
		t.Errorf("expected severity=critical, got %s", intent.Severity)
	}
}

func TestParseIntent_Status(t *testing.T) {
	intent := ParseIntent("is docker ready?")
	if intent.Action != "status" {
		t.Errorf("expected action=status, got %s", intent.Action)
	}
}

func TestParseIntent_Prefetch(t *testing.T) {
	intent := ParseIntent("download scanner images")
	if intent.Action != "prefetch" {
		t.Errorf("expected action=prefetch, got %s", intent.Action)
	}
}

func TestParseIntent_Help(t *testing.T) {
	intent := ParseIntent("what can you do?")
	if intent.Action != "help" {
		t.Errorf("expected action=help, got %s", intent.Action)
	}
}

func TestParseIntent_Unknown_DefaultsScan(t *testing.T) {
	intent := ParseIntent("what's the weather?")
	if intent.Action != "scan" {
		t.Errorf("expected unknown input to default to scan, got %s", intent.Action)
	}
}

func TestParseIntent_JSONFormat(t *testing.T) {
	intent := ParseIntent("scan this project and export as json")
	if intent.OutputFmt != "json" {
		t.Errorf("expected outputFmt=json, got %s", intent.OutputFmt)
	}
}

func TestParseIntent_IaC(t *testing.T) {
	intent := ParseIntent("check terraform for misconfigurations")
	found := false
	for _, c := range intent.Categories {
		if c == "iac" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iac in categories, got %v", intent.Categories)
	}
}
