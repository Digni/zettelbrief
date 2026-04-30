package scan

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/cyphant/zettelbrief/internal/models"
)

func ClassifyType(vaultRelativePath string, fm map[string]interface{}) models.NoteType {
	path := filepath.ToSlash(filepath.Clean(vaultRelativePath))
	var typ models.NoteType
	switch {
	case strings.Contains(path, "/1. Daily Work/") || strings.HasPrefix(path, "1. Daily Work/"):
		typ = models.NoteTypeDailyWork
	case strings.HasPrefix(path, "4.Granola/"):
		typ = models.NoteTypeMeeting
	case filepath.Base(path) == "State.md":
		typ = models.NoteTypeProjectState
	default:
		typ = models.NoteTypeKnowledge
	}
	if tags, ok := NormalizeFrontmatterList(fm, "tags"); ok {
		for _, tag := range tags {
			if strings.EqualFold(tag, "state") {
				return models.NoteTypeProjectState
			}
		}
	}
	if typ == models.NoteTypeMeeting {
		return typ
	}
	if _, hasID := fm["granola_id"]; hasID && strings.HasPrefix(path, "4.Granola/") {
		return models.NoteTypeMeeting
	}
	return typ
}

func ResolvePathProject(vaultRelativePath string) (string, bool) {
	path := filepath.ToSlash(filepath.Clean(vaultRelativePath))
	const prefix = "1.Projects/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", false
	}
	return parts[0], true
}

type ProjectMatchResult struct {
	Matched   []string
	Ambiguous []string
	Unmatched []string
}

func MatchGranolaProjects(folders []string, aliases map[string][]string) ProjectMatchResult {
	index := map[string][]string{}
	for project, names := range aliases {
		all := append([]string{project}, names...)
		for _, name := range all {
			key := normalizeProjectToken(name)
			if key == "" {
				continue
			}
			index[key] = append(index[key], project)
		}
	}
	matched := map[string]struct{}{}
	var ambiguous, unmatched []string
	for _, folder := range folders {
		key := normalizeProjectToken(folder)
		projects := uniqueSorted(index[key])
		switch len(projects) {
		case 0:
			unmatched = append(unmatched, folder)
		case 1:
			matched[projects[0]] = struct{}{}
		default:
			ambiguous = append(ambiguous, folder)
		}
	}
	out := ProjectMatchResult{Ambiguous: ambiguous, Unmatched: unmatched}
	for project := range matched {
		out.Matched = append(out.Matched, project)
	}
	sort.Strings(out.Matched)
	return out
}

func normalizeProjectToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r == ' ' || r == '-' || r == '_' || r == '.' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
