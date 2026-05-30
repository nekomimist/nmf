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

func TestPlainMatcherMatchesAllWhitespaceSeparatedTokens(t *testing.T) {
	matcher := NewPlainProvider().Build("beta ALP")

	if !matcher.Match("/tmp/alpha/beta.txt") {
		t.Fatal("plain matcher should match candidates containing all query tokens")
	}
	if matcher.Match("/tmp/alpha/gamma.txt") {
		t.Fatal("plain matcher should reject candidates missing one query token")
	}
}

func TestPlainMatcherIgnoresRepeatedOuterWhitespace(t *testing.T) {
	matcher := NewPlainProvider().Build("  beta   alpha  ")

	if !matcher.Match("/tmp/alpha/beta.txt") {
		t.Fatal("plain matcher should ignore repeated and outer whitespace")
	}
}

func TestMigemoMatcherMatchesJapaneseCandidate(t *testing.T) {
	matcher := NewProvider(func(string, ...interface{}) {}).Build("nihongo")

	if !matcher.Match("日本語.txt") {
		t.Fatal("migemo matcher should match romaji query against Japanese text")
	}
}

func TestMigemoMatcherCombinesTokenMatchesWithAnd(t *testing.T) {
	matcher := NewProvider(func(string, ...interface{}) {}).Build("tmp nihongo")

	if !matcher.Match("/tmp/日本語") {
		t.Fatal("migemo matcher should match Japanese candidate when all query tokens match")
	}
	if matcher.Match("/work/日本語") {
		t.Fatal("migemo matcher should reject candidates missing the plain token")
	}
}
