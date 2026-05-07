package store

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSearchQueryRemovesStopwordsAndDetectsIdentifiers(t *testing.T) {
	terms := NormalizeSearchQuery("fix the One.Backend billableService snake_case kebab-case path/toThing update persistence")
	var got []QueryTerm
	for _, term := range terms {
		got = append(got, term)
	}
	wantTokens := []string{"one", "backend", "billable", "service", "snake", "case", "kebab", "path", "to", "thing", "persistence"}
	if tokens := TokenizeSearchQuery("fix the One.Backend billableService snake_case kebab-case path/toThing update persistence"); !reflect.DeepEqual(tokens, wantTokens) {
		t.Fatalf("tokens=%#v want %#v; terms=%#v", tokens, wantTokens, got)
	}
	identifierByRaw := map[string]bool{}
	for _, term := range terms {
		identifierByRaw[term.Raw] = term.Identifier
	}
	for _, raw := range []string{"One.Backend", "billableService", "snake_case", "kebab-case", "path/toThing"} {
		if !identifierByRaw[raw] {
			t.Fatalf("%s was not classified as identifier: %#v", raw, terms)
		}
	}
}

func TestBuildFTSQueryORsIdentifierComponentsAndQuotesMetacharacters(t *testing.T) {
	terms := NormalizeSearchQuery(`One.Backend persistence "quoted"`)
	match, err := BuildFTSQuery(terms)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(match, `("one" OR "backend")`) || !strings.Contains(match, `"persistence"`) || !strings.Contains(match, `"quoted"`) {
		t.Fatalf("match expression = %s", match)
	}
}

func TestNormalizeSearchQueryStopwordOnlyIsEmpty(t *testing.T) {
	if terms := NormalizeSearchQuery("fix add update the"); len(terms) != 0 {
		t.Fatalf("terms=%#v", terms)
	}
	if _, err := BuildFTSQuery(nil); err == nil || !strings.Contains(err.Error(), "no searchable terms") {
		t.Fatalf("err=%v", err)
	}
}
