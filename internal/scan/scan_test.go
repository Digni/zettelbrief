package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cyphant/zettelbrief/internal/models"
)

func TestWalkParseNormalizeAndHash(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "one.md"), []byte("---\ntags: '#acme, #backend'\nfolders: Acme\n---\n# Title\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "two.txt"), []byte("skip"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"a/one.md"}) {
		t.Fatalf("files = %#v", files)
	}
	content, err := ReadFile(filepath.Join(root, "a", "one.md"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	fm, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	tags, ok := NormalizeFrontmatterList(fm, "tags")
	if !ok || !reflect.DeepEqual(tags, []string{"acme", "backend"}) {
		t.Fatalf("tags = %#v ok=%v", tags, ok)
	}
	folders, ok := NormalizeFrontmatterList(fm, "folders")
	if !ok || !reflect.DeepEqual(folders, []string{"Acme"}) {
		t.Fatalf("folders = %#v ok=%v", folders, ok)
	}
	if HashContent(content) != HashContent(content) || HashContent(content) == HashContent(content+"x") {
		t.Fatalf("hash is not stable/distinct")
	}
	if _, err := ReadFile(filepath.Join(root, "a", "one.md"), 1); err == nil {
		t.Fatalf("oversized read should fail")
	}
	if _, err := ParseFrontmatter("---\ntags: [\n---\n"); err == nil {
		t.Fatalf("invalid frontmatter should fail")
	}
}

func TestClassificationProjectMatchingAndExtraction(t *testing.T) {
	cases := map[string]models.NoteType{
		"1.Projects/Acme/1. Daily Work/2026/04/2026-04-24.md": models.NoteTypeDailyWork,
		"4.Granola/2026-04/16/Daily Acme-2026-04-16.md":       models.NoteTypeMeeting,
		"1.Projects/Flive/State.md":                           models.NoteTypeProjectState,
		"1.Projects/Acme/Backend/architecture-overview.md":    models.NoteTypeKnowledge,
	}
	for path, want := range cases {
		if got := ClassifyType(path, nil); got != want {
			t.Fatalf("ClassifyType(%q)=%s want %s", path, got, want)
		}
	}
	if got := ClassifyType("foo.md", map[string]interface{}{"tags": "#state, #project/flive"}); got != models.NoteTypeProjectState {
		t.Fatalf("state tag override = %s", got)
	}
	if project, ok := ResolvePathProject("1.Projects/Acme/State.md"); !ok || project != "Acme" {
		t.Fatalf("path project = %q ok=%v", project, ok)
	}
	matches := MatchGranolaProjects([]string{"Acme", "I Reckonu", "Missing"}, map[string][]string{"Acme": {"Acme"}, "IReckonu": {"I Reckonu"}})
	if !reflect.DeepEqual(matches.Matched, []string{"Acme", "IReckonu"}) || !reflect.DeepEqual(matches.Unmatched, []string{"Missing"}) {
		t.Fatalf("matches = %#v", matches)
	}
	sections := SplitDailyWorkSections("## One.Backend\n- Repo: One.Backend\n- Branch: main\n- Summary: Done\n\n### Follow-up\nText\n\n## Missing\n- Summary: skip")
	if len(sections) != 2 || sections[0].SectionID != "001-one-backend" {
		t.Fatalf("sections = %#v", sections)
	}
	meta, ok := ExtractDailyWork(sections[0])
	if !ok || meta.Repo != "One.Backend" || meta.Branch != "main" || meta.Summary != "Done" || !strings.Contains(sections[0].Content, "Follow-up") {
		t.Fatalf("daily meta = %#v ok=%v", meta, ok)
	}
	if _, ok := ExtractDailyWork(sections[1]); ok {
		t.Fatalf("section without repo should be skipped")
	}
	meeting := ExtractMeeting(map[string]interface{}{"title": "Daily Acme", "created": "2026-04-16T08:45:29.243Z", "granola_id": "abc"})
	if meeting.Date != "2026-04-16" || meeting.GranolaID != "abc" || !reflect.DeepEqual(meeting.Tags, []string{"granola"}) {
		t.Fatalf("meeting = %#v", meeting)
	}
	generic := ExtractGeneric("# Heading\nBody", map[string]interface{}{"type": "decision", "tags": []interface{}{"acme"}}, "x.md")
	if generic.Title != "Heading" || generic.Extra["raw_type"] != "decision" {
		t.Fatalf("generic = %#v", generic)
	}
	if date := ExtractDate("foo/2026-04-24.md", nil, models.NoteTypeKnowledge); date != "2026-04-24" {
		t.Fatalf("date = %s", date)
	}
}
