package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DocReader struct {
	KnownDocPaths []string
}

func NewDocReader() *DocReader {
	return &DocReader{
		KnownDocPaths: []string{
			"/opt/homebrew",
			"/usr/local",
			"/usr/share",
			"/usr/lib",
		},
	}
}

func (d *DocReader) SearchDocs(name string) ([]string, error) {
	var results []string
	seen := make(map[string]bool)

	for _, base := range d.KnownDocPaths {
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				if err == nil && entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(entry.Name(), "._") {
				return nil
			}
			if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(name)) {
				if !seen[path] {
					seen[path] = true
					results = append(results, path)
				}
			}
			return nil
		})
		if err != nil {
			continue
		}
	}

	return results, nil
}

func (d *DocReader) FindDocPaths(language string) ([]string, error) {
	langDirs := map[string][]string{
		"zig":        {"/opt/homebrew/opt/zig"},
		"go":         {"/opt/homebrew/opt/go"},
		"rust":       {"/opt/homebrew/opt/rust"},
		"typescript": {"/opt/homebrew/opt/node"},
		"javascript": {"/opt/homebrew/opt/node"},
		"python":     {"/opt/homebrew/opt/python"},
	}

	lang := strings.ToLower(language)
	dirs, ok := langDirs[lang]
	if !ok {
		return nil, fmt.Errorf("no known doc path for %s", language)
	}

	var results []string
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			results = append(results, dir)
		}
	}
	return results, nil
}

func (d *DocReader) GetVersion(language string) string {
	paths := map[string]string{
		"zig":    "/opt/homebrew/opt/zig/bin/zig",
		"go":     "/opt/homebrew/opt/go/bin/go",
		"rust":   "/opt/homebrew/opt/rust/bin/rustc",
		"node":   "/opt/homebrew/opt/node/bin/node",
		"python": "/opt/homebrew/opt/python/bin/python3",
	}
	bin, ok := paths[strings.ToLower(language)]
	if !ok {
		return ""
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		return "(not found)"
	}
	return fmt.Sprintf("%s (%d bytes)", bin, len(data))
}

func (d *DocReader) FindInstallPrefix(language string) string {
	prefixes := map[string]string{
		"zig":    "/opt/homebrew/opt/zig",
		"go":     "/opt/homebrew/opt/go",
		"rust":   "/opt/homebrew/opt/rust",
		"node":   "/opt/homebrew/opt/node",
		"python": "/opt/homebrew/opt/python",
	}
	if p, ok := prefixes[strings.ToLower(language)]; ok {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}

type DocFile struct {
	Path    string
	Content string
	Size    int64
}

func (d *DocReader) ReadDoc(path string, maxSize int64) (*DocFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("doc file too large: %d bytes", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &DocFile{
		Path:    path,
		Content: string(data),
		Size:    info.Size(),
	}, nil
}

func FindDocsInPrefix(prefix string, extensions []string, max int) ([]string, error) {
	var results []string
	err := filepath.WalkDir(prefix, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(entry.Name(), "._") {
			return nil
		}
		if len(results) >= max {
			return filepath.SkipAll
		}
		ext := filepath.Ext(entry.Name())
		for _, wanted := range extensions {
			if strings.EqualFold(ext, wanted) {
				results = append(results, path)
				break
			}
		}
		return nil
	})
	return results, err
}
