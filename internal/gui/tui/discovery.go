package tui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sahilm/fuzzy"
)

// CommandInfo holds metadata about a discovered system command.
type CommandInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CommandDiscovery handles background indexing and fuzzy searching of system commands.
type CommandDiscovery struct {
	commands []CommandInfo
	mu       sync.RWMutex
	cacheDir string
}

func NewCommandDiscovery() *CommandDiscovery {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".duckops")
	_ = os.MkdirAll(cacheDir, 0700)

	d := &CommandDiscovery{
		cacheDir: cacheDir,
	}
	d.loadCache()
	go d.Refresh() // Background indexing
	return d
}

func (d *CommandDiscovery) Refresh() {
	paths := filepath.SplitList(os.Getenv("PATH"))
	found := make(map[string]string)

	for _, path := range paths {
		files, err := os.ReadDir(path)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			if isExecutable(f, path) {
				// De-duplicate: first one in PATH wins
				if _, ok := found[f.Name()]; !ok {
					found[f.Name()] = filepath.Join(path, f.Name())
				}
			}
		}
	}

	var commands []CommandInfo
	for name, path := range found {
		commands = append(commands, CommandInfo{Name: name, Path: path})
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	d.mu.Lock()
	d.commands = commands
	d.mu.Unlock()
	d.saveCache()
}

func (d *CommandDiscovery) Search(query string) []CommandInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if query == "" {
		return nil
	}

	// Fuzzy search
	names := make([]string, len(d.commands))
	for i, c := range d.commands {
		names[i] = c.Name
	}

	matches := fuzzy.Find(query, names)
	
	result := make([]CommandInfo, 0, len(matches))
	// Limit to top 10 results
	limit := len(matches)
	if limit > 10 {
		limit = 10
	}
	
	for i := 0; i < limit; i++ {
		result = append(result, d.commands[matches[i].Index])
	}
	
	return result
}

func (d *CommandDiscovery) SearchFiles(query string) []CommandInfo {
	cwd, _ := os.Getwd()
	
	// We'll perform a fast recursive walk. 
	// To keep it performant, we limit the depth or the number of items.
	var names []string
	var fullInfos []CommandInfo
	
	// Skip common noise directories
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".duckops":     true,
		"bin":          true,
		"obj":          true,
	}

	maxItems := 1000 // Safety limit for indexing
	count := 0

	filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil || count >= maxItems {
			return nil
		}
		
		name := d.Name()
		if d.IsDir() {
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// We can also include directories in the search
		}

		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}

		// Don't include the current directory itself "."
		if rel == "." {
			return nil
		}

		names = append(names, rel)
		fullInfos = append(fullInfos, CommandInfo{
			Name: rel,
			Path: path,
		})
		count++
		return nil
	})

	if query == "" {
		// Just show top relative items
		limit := len(fullInfos)
		if limit > 10 { limit = 10 }
		return fullInfos[:limit]
	}

	matches := fuzzy.Find(query, names)
	result := make([]CommandInfo, 0, len(matches))
	limit := len(matches)
	if limit > 10 { limit = 10 }
	
	for i := 0; i < limit; i++ {
		result = append(result, fullInfos[matches[i].Index])
	}
	return result
}

func (d *CommandDiscovery) loadCache() {
	cachePath := filepath.Join(d.cacheDir, "commands.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return
	}

	var commands []CommandInfo
	if err := json.Unmarshal(data, &commands); err == nil {
		d.mu.Lock()
		d.commands = commands
		d.mu.Unlock()
	}
}

func (d *CommandDiscovery) saveCache() {
	cachePath := filepath.Join(d.cacheDir, "commands.json")
	d.mu.RLock()
	data, _ := json.Marshal(d.commands)
	d.mu.RUnlock()
	_ = os.WriteFile(cachePath, data, 0600)
}

func isExecutable(f fs.DirEntry, path string) bool {
	if runtime.GOOS == "windows" {
		exts := []string{".exe", ".bat", ".cmd", ".ps1"}
		name := strings.ToLower(f.Name())
		for _, ext := range exts {
			if strings.HasSuffix(name, ext) {
				return true
			}
		}
		return false
	}

	// Unix: check executable bit
	info, err := f.Info()
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}
