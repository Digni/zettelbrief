package scan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cyphant/zettelbrief/internal/models"
)

type DailyWorkSection = models.DailyWorkSection

var datePattern = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)

func SplitDailyWorkSections(content string) []DailyWorkSection {
	lines := strings.Split(content, "\n")
	var sections []DailyWorkSection
	current := -1
	var body strings.Builder
	flush := func() {
		if current >= 0 {
			sections[current].Content = strings.TrimSpace(body.String())
			body.Reset()
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			flush()
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			idx := len(sections) + 1
			sections = append(sections, DailyWorkSection{Index: idx, Heading: heading, SectionID: fmt.Sprintf("%03d-%s", idx, slugify(heading))})
			current = len(sections) - 1
			continue
		}
		if current >= 0 {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func ExtractDailyWork(section DailyWorkSection) (models.NoteMetadata, bool) {
	fields := map[string]string{}
	for _, line := range strings.Split(section.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(trimmed, "- "), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		fields[key] = value
	}
	repo := fields["repo"]
	if repo == "" {
		return models.NoteMetadata{}, false
	}
	return models.NoteMetadata{
		SectionID:    section.SectionID,
		Repo:         repo,
		Branch:       fields["branch"],
		Summary:      fields["summary"],
		Verification: fields["verification"],
		NotesText:    fields["notes"],
		CommitHash:   fields["commit"],
		Ticket:       fields["ticket"],
		Title:        section.Heading,
	}, true
}

func ExtractMeeting(fm map[string]interface{}) models.NoteMetadata {
	created := stringValue(fm["created"])
	updated := stringValue(fm["updated"])
	title := stringValue(fm["title"])
	folders, _ := NormalizeFrontmatterList(fm, "folders")
	extra := map[string]interface{}{}
	if rawType := stringValue(fm["type"]); rawType != "" {
		extra["raw_type"] = rawType
	}
	if len(folders) > 0 {
		extra["folders"] = folders
	}
	return models.NoteMetadata{
		Title:     title,
		Date:      dateFromValue(created),
		UpdatedAt: updated,
		GranolaID: stringValue(fm["granola_id"]),
		Tags:      []string{"granola"},
		Extra:     extra,
	}
}

func ExtractGeneric(content string, fm map[string]interface{}, path string) models.NoteMetadata {
	title := firstHeading(content)
	if title == "" {
		base := filepath.Base(filepath.ToSlash(path))
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}
	tags, _ := NormalizeFrontmatterList(fm, "tags")
	extra := map[string]interface{}{}
	if rawType := stringValue(fm["type"]); rawType != "" {
		extra["raw_type"] = rawType
	}
	return models.NoteMetadata{Title: title, Tags: tags, UpdatedAt: stringValue(fm["updated"]), Extra: extra}
}

func ExtractDate(path string, fm map[string]interface{}, noteType models.NoteType) string {
	if match := datePattern.FindStringSubmatch(path); len(match) == 2 {
		return match[1]
	}
	for _, key := range []string{"created", "date"} {
		if date := dateFromValue(stringValue(fm[key])); date != "" {
			return date
		}
	}
	return ""
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func dateFromValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if match := datePattern.FindStringSubmatch(value); len(match) == 2 {
		return match[1]
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
