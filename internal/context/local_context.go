// Package context provides runtime environment discovery for DuckOps sessions.
// It collects OS, shell, git, project markers, and cloud account info and
// formats it as a system-prompt prefix so the LLM knows where it's running.
//
// Mirrors duckops cli/src/utils/local_context.rs + discovery/*.rs
package context

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LocalContext holds everything we know about the user's environment.
type LocalContext struct {
	MachineName      string
	OS               string // "macOS", "Ubuntu 22.04", "Windows", …
	Shell            string // "zsh", "bash", "fish", …
	IsContainer      bool
	WorkingDirectory string
	DateTime         time.Time

	Git     GitInfo
	Project ProjectInfo
}

// GitInfo describes the current git state.
type GitInfo struct {
	IsRepo         bool
	Branch         string
	HasUncommitted bool
	RemoteURL      string
}

// ProjectInfo describes detected languages, frameworks, and CI/CD.
type ProjectInfo struct {
	Languages  []string // ["Go", "Python", …]
	Frameworks []string // ["Docker Compose", "Terraform", …]
	CICD       []string // ["GitHub Actions", …]
}

// Collect builds a LocalContext for the current working directory.
// Non-critical errors are silently ignored — we never block startup.
func Collect() LocalContext {
	cwd, _ := os.Getwd()
	hostname, _ := os.Hostname()

	ctx := LocalContext{
		MachineName:      hostname,
		OS:               detectOS(),
		Shell:            detectShell(),
		IsContainer:      detectContainer(),
		WorkingDirectory: cwd,
		DateTime:         time.Now().UTC(),
		Git:              collectGit(cwd),
		Project:          detectProject(cwd),
	}
	return ctx
}

// FormatSystemPrompt returns a concise markdown block for injection into
// the agent system prompt. Matches duckops format_display() output.
func (c *LocalContext) FormatSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("# System Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Machine:** %s\n", c.MachineName))
	sb.WriteString(fmt.Sprintf("- **OS:** %s\n", c.OS))
	sb.WriteString(fmt.Sprintf("- **Shell:** %s\n", c.Shell))
	sb.WriteString(fmt.Sprintf("- **Date/Time:** %s UTC\n", c.DateTime.Format("2006-01-02 15:04:05")))
	if c.IsContainer {
		sb.WriteString("- **Environment:** container (Docker/Kubernetes/Podman)\n")
	}
	sb.WriteString(fmt.Sprintf("- **Working Directory:** %s\n", c.WorkingDirectory))

	// Git
	if c.Git.IsRepo {
		sb.WriteString("\n## Git\n\n")
		if c.Git.Branch != "" {
			sb.WriteString(fmt.Sprintf("- **Branch:** %s\n", c.Git.Branch))
		}
		if c.Git.HasUncommitted {
			sb.WriteString("- **Uncommitted changes:** yes\n")
		}
		if c.Git.RemoteURL != "" {
			sb.WriteString(fmt.Sprintf("- **Remote:** %s\n", c.Git.RemoteURL))
		}
	}

	// Project markers
	if len(c.Project.Languages) > 0 || len(c.Project.Frameworks) > 0 || len(c.Project.CICD) > 0 {
		sb.WriteString("\n## Project\n\n")
		if len(c.Project.Languages) > 0 {
			sb.WriteString(fmt.Sprintf("- **Languages/Runtimes:** %s\n", strings.Join(c.Project.Languages, ", ")))
		}
		if len(c.Project.Frameworks) > 0 {
			sb.WriteString(fmt.Sprintf("- **Infrastructure:** %s\n", strings.Join(c.Project.Frameworks, ", ")))
		}
		if len(c.Project.CICD) > 0 {
			sb.WriteString(fmt.Sprintf("- **CI/CD:** %s\n", strings.Join(c.Project.CICD, ", ")))
		}
	}

	return sb.String()
}

// ── OS detection ──────────────────────────────────────────────────────────────

func detectOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		// Try /etc/os-release for distro name
		if b, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					name := strings.TrimPrefix(line, "PRETTY_NAME=")
					return strings.Trim(name, `"`)
				}
			}
		}
		return "Linux"
	default:
		return runtime.GOOS
	}
}

// ── Shell detection ───────────────────────────────────────────────────────────

func detectShell() string {
	// SHELL env var is the most reliable on Unix
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	// Windows
	if os.Getenv("PSModulePath") != "" {
		return "PowerShell"
	}
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return filepath.Base(comspec)
	}
	return "unknown"
}

// ── Container detection ───────────────────────────────────────────────────────

func detectContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	for _, v := range []string{"DOCKER_CONTAINER", "KUBERNETES_SERVICE_HOST", "PODMAN_VERSION"} {
		if os.Getenv(v) != "" {
			return true
		}
	}
	if b, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(b)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") || strings.Contains(s, "podman") {
			return true
		}
	}
	return false
}

// ── Git info ──────────────────────────────────────────────────────────────────

func collectGit(dir string) GitInfo {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return GitInfo{}
	}
	info := GitInfo{IsRepo: true}

	if out := gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD"); out != "" && out != "HEAD" {
		info.Branch = out
	}
	if out := gitCmd(dir, "status", "--porcelain"); out != "" {
		info.HasUncommitted = true
	}
	if out := gitCmd(dir, "remote", "get-url", "origin"); out != "" {
		info.RemoteURL = out
	}
	return info
}

func gitCmd(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// ── Project marker detection ──────────────────────────────────────────────────

var languageMarkers = []struct{ file, lang string }{
	{"go.mod", "Go"},
	{"package.json", "Node.js"},
	{"Cargo.toml", "Rust"},
	{"pyproject.toml", "Python"}, {"requirements.txt", "Python"}, {"setup.py", "Python"},
	{"pom.xml", "Java (Maven)"}, {"build.gradle", "Java (Gradle)"},
	{"Gemfile", "Ruby"},
	{"composer.json", "PHP"},
	{"mix.exs", "Elixir"},
	{"pubspec.yaml", "Dart/Flutter"},
}

var infraMarkers = []struct{ file, name string }{
	{"docker-compose.yml", "Docker Compose"}, {"docker-compose.yaml", "Docker Compose"},
	{"compose.yml", "Docker Compose"}, {"compose.yaml", "Docker Compose"},
	{"Dockerfile", "Docker"},
	{"*.tf", "Terraform"}, {"terragrunt.hcl", "Terragrunt"},
	{"Pulumi.yaml", "Pulumi"}, {"cdk.json", "AWS CDK"},
	{"ansible.cfg", "Ansible"},
	{"Chart.yaml", "Helm"}, {"helmfile.yaml", "Helmfile"},
	{"kustomization.yaml", "Kustomize"},
}

var ciMarkers = []struct{ file, name string }{
	{".github/workflows", "GitHub Actions"},
	{".gitlab-ci.yml", "GitLab CI"},
	{"Jenkinsfile", "Jenkins"},
	{".circleci/config.yml", "CircleCI"},
	{"bitbucket-pipelines.yml", "Bitbucket Pipelines"},
	{"azure-pipelines.yml", "Azure Pipelines"},
	{"cloudbuild.yaml", "Cloud Build"},
}

func detectProject(dir string) ProjectInfo {
	var p ProjectInfo
	seen := map[string]bool{}

	add := func(slice *[]string, val string) {
		if !seen[val] {
			seen[val] = true
			*slice = append(*slice, val)
		}
	}

	entries, _ := os.ReadDir(dir)
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}

	for _, m := range languageMarkers {
		if names[m.file] {
			add(&p.Languages, m.lang)
		}
	}
	for _, m := range infraMarkers {
		if strings.HasPrefix(m.file, "*.") {
			ext := strings.TrimPrefix(m.file, "*")
			for name := range names {
				if strings.HasSuffix(name, ext) {
					add(&p.Frameworks, m.name)
					break
				}
			}
		} else if names[m.file] {
			add(&p.Frameworks, m.name)
		} else if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			add(&p.Frameworks, m.name)
		}
	}
	for _, m := range ciMarkers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			add(&p.CICD, m.name)
		}
	}

	return p
}
