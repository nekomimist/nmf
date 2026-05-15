package search

import "testing"

func TestPlainMatcherIsCaseInsensitiveSubstring(t *testing.T) {
	matcher := NewPlainProvider().Build("ALP")

	if !matcher.Match("alpha.txt") {
		t.Fatal("plain matcher should match case-insensitive substrings")
	}
	if matcher.Match("beta.txt") {
		t.Fatal("plain matcher should not match unrelated text")
	}
}

func TestMigemoMatcherMatchesJapaneseCandidate(t *testing.T) {
	matcher := NewProvider(func(string, ...interface{}) {}).Build("nihongo")

	if !matcher.Match("日本語.txt") {
		t.Fatal("migemo matcher should match romaji query against Japanese text")
	}
}
