package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const DefaultMaxNoteBytes int64 = 10 * 1024 * 1024

func NormalizeVaultRelative(vaultRoot, path string) (string, error) {
	if vaultRoot == "" {
		return "", errors.New("vault root is required")
	}
	vaultAbs, err := filepath.Abs(vaultRoot)
	if err != nil {
		return "", err
	}
	var abs string
	if filepath.IsAbs(path) {
		abs = path
	} else {
		abs = filepath.Join(vaultAbs, filepath.FromSlash(path))
	}
	resolvedVault, err := filepath.EvalSymlinks(vaultAbs)
	if err != nil {
		return "", err
	}
	resolvedAbs, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedVault, resolvedAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside vault", path)
	}
	return filepath.ToSlash(filepath.Clean(rel)), nil
}

func Walk(root string) ([]string, error) {
	return walk(root, root)
}

func WalkVault(vaultRoot, relRoot string) ([]string, error) {
	if filepath.IsAbs(relRoot) {
		return nil, errors.New("walk root must be vault-relative")
	}
	return walk(vaultRoot, filepath.Join(vaultRoot, filepath.FromSlash(relRoot)))
}

func walk(vaultRoot, start string) ([]string, error) {
	info, err := os.Lstat(start)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(start)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(vaultRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, errors.New("symlink escapes vault")
		}
		start = resolved
	}
	var files []string
	err = filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			rel, err := NormalizeVaultRelative(vaultRoot, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return DedupePaths(files), nil
}

func DedupePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		clean := filepath.ToSlash(filepath.Clean(p))
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func ReadFile(path string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxNoteBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("file exceeds maximum size of %d bytes", maxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("file is not valid UTF-8")
	}
	return string(data), nil
}

func ParseFrontmatter(content string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return out, nil
	}
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			raw := strings.Join(lines[1:i], "\n")
			if strings.TrimSpace(raw) == "" {
				return out, nil
			}
			if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
				return nil, err
			}
			if out == nil {
				out = map[string]interface{}{}
			}
			return out, nil
		}
	}
	return nil, errors.New("frontmatter opening delimiter without closing delimiter")
}

func NormalizeFrontmatterList(fm map[string]interface{}, key string) ([]string, bool) {
	if fm == nil {
		return nil, true
	}
	value, ok := fm[key]
	if !ok || value == nil {
		return nil, true
	}
	var raw []string
	switch v := value.(type) {
	case string:
		raw = splitListString(v)
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			raw = append(raw, splitListString(s)...)
		}
	case []string:
		for _, item := range v {
			raw = append(raw, splitListString(item)...)
		}
	default:
		return nil, false
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		norm := strings.TrimSpace(strings.TrimPrefix(item, "#"))
		if norm == "" {
			continue
		}
		if _, exists := seen[norm]; exists {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out, true
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func splitListString(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}
