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
	if err := os.WriteFile(filepath.Join(root, "a", "one.md"), []byte("---\ntags: '#vetz, #backend'\nfolders: VetZ\n---\n# Title\n"), 0o600); err != nil {
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
	if !ok || !reflect.DeepEqual(tags, []string{"vetz", "backend"}) {
		t.Fatalf("tags = %#v ok=%v", tags, ok)
	}
	folders, ok := NormalizeFrontmatterList(fm, "folders")
	if !ok || !reflect.DeepEqual(folders, []string{"VetZ"}) {
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
		"1.Projects/VetZ/1. Daily Work/2026/04/2026-04-24.md": models.NoteTypeDailyWork,
		"4.Granola/2026-04/16/Daily Vetz-2026-04-16.md":       models.NoteTypeMeeting,
		"1.Projects/Flive/State.md":                           models.NoteTypeProjectState,
		"1.Projects/VetZ/Backend/architecture-overview.md":    models.NoteTypeKnowledge,
	}
	for path, want := range cases {
		if got := ClassifyType(path, nil); got != want {
			t.Fatalf("ClassifyType(%q)=%s want %s", path, got, want)
		}
	}
	if got := ClassifyType("foo.md", map[string]interface{}{"tags": "#state, #project/flive"}); got != models.NoteTypeProjectState {
		t.Fatalf("state tag override = %s", got)
	}
	if project, ok := ResolvePathProject("1.Projects/VetZ/State.md"); !ok || project != "VetZ" {
		t.Fatalf("path project = %q ok=%v", project, ok)
	}
	matches := MatchGranolaProjects([]string{"Vetz", "I Reckonu", "Missing"}, map[string][]string{"VetZ": {"Vetz"}, "IReckonu": {"I Reckonu"}})
	if !reflect.DeepEqual(matches.Matched, []string{"IReckonu", "VetZ"}) || !reflect.DeepEqual(matches.Unmatched, []string{"Missing"}) {
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
	meeting := ExtractMeeting(map[string]interface{}{"title": "Daily Vetz", "created": "2026-04-16T08:45:29.243Z", "granola_id": "abc"})
	if meeting.Date != "2026-04-16" || meeting.GranolaID != "abc" || !reflect.DeepEqual(meeting.Tags, []string{"granola"}) {
		t.Fatalf("meeting = %#v", meeting)
	}
	generic := ExtractGeneric("# Heading\nBody", map[string]interface{}{"type": "decision", "tags": []interface{}{"vetz"}}, "x.md")
	if generic.Title != "Heading" || generic.Extra["raw_type"] != "decision" {
		t.Fatalf("generic = %#v", generic)
	}
	if date := ExtractDate("foo/2026-04-24.md", nil, models.NoteTypeKnowledge); date != "2026-04-24" {
		t.Fatalf("date = %s", date)
	}
}
