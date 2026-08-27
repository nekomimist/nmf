package search

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/koron/gomigemo/migemo"
)

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

func TestProviderLoadsMigemoOnFirstNonEmptyQueryOnly(t *testing.T) {
	loadCount := 0
	provider := newProvider(nil, func() (migemo.Dict, error) {
		loadCount++
		return nil, errors.New("test load failure")
	})

	if loadCount != 0 {
		t.Fatalf("load count after construction = %d, want 0", loadCount)
	}
	provider.Build("   ")
	if loadCount != 0 {
		t.Fatalf("load count after whitespace query = %d, want 0", loadCount)
	}
	provider.Build("alpha")
	provider.Build("beta")
	if loadCount != 1 {
		t.Fatalf("load count after non-empty queries = %d, want 1", loadCount)
	}
}

func TestProviderLoadsMigemoOnceAcrossConcurrentQueries(t *testing.T) {
	var loadCount atomic.Int32
	provider := newProvider(nil, func() (migemo.Dict, error) {
		loadCount.Add(1)
		return nil, errors.New("test load failure")
	})

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			provider.Build("alpha")
		}()
	}
	wg.Wait()

	if got := loadCount.Load(); got != 1 {
		t.Fatalf("load count = %d, want 1", got)
	}
}
