package ports

// CommandNormalizer defines the ability to extract the intended command
// from a platform-specific wrapped command (e.g., powershell -Command ...).
type CommandNormalizer interface {
	Normalize(cmd string, args []string) (string, []string)
}
