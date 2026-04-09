package parser

import (
	"strings"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	src := []byte("---\ntitle: Hi\n---\n\n# Body\n")
	yml, body, ok := SplitFrontmatter(src)
	if !ok {
		t.Fatal("expected frontmatter")
	}
	if string(yml) != "title: Hi" {
		t.Fatalf("yaml: %q", yml)
	}
	if !strings.Contains(string(body), "# Body") {
		t.Fatalf("body: %q", body)
	}
}

func TestSplitFrontmatterNone(t *testing.T) {
	src := []byte("# No fm\n")
	_, body, ok := SplitFrontmatter(src)
	if ok {
		t.Fatal("unexpected frontmatter")
	}
	if string(body) != string(src) {
		t.Fatalf("body should be full src")
	}
}

func TestParseFrontmatterYAML_Aliases(t *testing.T) {
	yml := []byte(`
title: Real Title
alias: "[[Another Name]]"
aliases:
  - Foo
  - "[[Bar]]"
tags: [a, b]
public: true
`)
	props, aliases, err := ParseFrontmatterYAML(yml)
	if err != nil {
		t.Fatal(err)
	}
	if props["title"] != "Real Title" {
		t.Fatalf("title: %v", props["title"])
	}
	if props["public"] != "true" {
		t.Fatalf("public: %v", props["public"])
	}
	want := map[string]bool{"Another Name": true, "Foo": true, "Bar": true}
	for _, a := range aliases {
		if !want[a] {
			t.Errorf("unexpected alias %q", a)
		}
		delete(want, a)
	}
	if len(want) != 0 {
		t.Fatalf("missing aliases: %v", want)
	}
}

func TestNormalizeAliasKey(t *testing.T) {
	if NormalizeAliasKey("[[  Hello  ]]") != "hello" {
		t.Fatalf("%q", NormalizeAliasKey("[[  Hello  ]]"))
	}
}
