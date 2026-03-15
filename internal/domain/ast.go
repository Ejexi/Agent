package domain

import "strings"

// ExecutionAST represents the structured intent parsed from a raw command.
// This is used by the Warden to evaluate Cedar policies against semantic meaning
// rather than raw string matching.
type ExecutionAST struct {
	Action   string            `json:"action"`   // e.g., "delete", "apply", "read", "list"
	Resource string            `json:"resource"` // e.g., "pod/xyz", "main.tf", "/var/log"
	Provider string            `json:"provider"` // e.g., "kubectl", "terraform", "docker"
	Flags    map[string]string `json:"flags"`    // e.g., "--force": "true", "--namespace": "prod"
}

// ParseExecutionAST converts a raw command and its arguments into a structured AST.
func ParseExecutionAST(command string, args []string) ExecutionAST {
	ast := ExecutionAST{
		Provider: command,
		Flags:    make(map[string]string),
	}

	switch command {
	case "kubectl":
		ast = parseKubectl(args, ast)
	case "terraform":
		ast = parseTerraform(args, ast)
	case "docker":
		ast = parseDocker(args, ast)
	case "rm":
		ast.Action = "delete"
		if len(args) > 0 {
			ast.Resource = args[len(args)-1]
		}
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				ast.Flags[a] = "true"
			}
		}
	default:
		ast.Action = "execute"
		if len(args) > 0 {
			ast.Resource = strings.Join(args, " ")
		}
	}

	return ast
}

func parseKubectl(args []string, ast ExecutionAST) ExecutionAST {
	if len(args) == 0 {
		ast.Action = "help"
		return ast
	}
	ast.Action = args[0] // e.g., "delete", "apply", "get"

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				ast.Flags[arg] = args[i+1]
				i++
			} else {
				ast.Flags[arg] = "true"
			}
		} else if ast.Resource == "" {
			ast.Resource = arg
		} else {
			ast.Resource += "/" + arg
		}
	}
	return ast
}

func parseTerraform(args []string, ast ExecutionAST) ExecutionAST {
	if len(args) == 0 {
		ast.Action = "help"
		return ast
	}
	ast.Action = args[0] // e.g., "plan", "apply", "destroy"

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				ast.Flags[arg] = args[i+1]
				i++
			} else {
				ast.Flags[arg] = "true"
			}
		} else if ast.Resource == "" {
			ast.Resource = arg
		}
	}
	return ast
}

func parseDocker(args []string, ast ExecutionAST) ExecutionAST {
	if len(args) == 0 {
		ast.Action = "help"
		return ast
	}
	ast.Action = args[0] // e.g., "run", "rm", "stop", "build"

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				ast.Flags[arg] = args[i+1]
				i++
			} else {
				ast.Flags[arg] = "true"
			}
		} else if ast.Resource == "" {
			ast.Resource = arg
		}
	}
	return ast
}
