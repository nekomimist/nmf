package search

import (
	"strings"

	"github.com/koron/gomigemo/embedict"
	"github.com/koron/gomigemo/migemo"
)

// Matcher checks whether a candidate matches a user query.
type Matcher interface {
	Match(candidate string) bool
}

// Provider builds query matchers. It always supports plain case-insensitive
// substring matching, and adds migemo matching when the embedded dictionary
// loads successfully.
type Provider struct {
	dict       migemo.Dict
	debugPrint func(format string, args ...interface{})
}

// NewProvider creates a matcher provider backed by gomigemo's embedded
// dictionary. If dictionary loading fails, returned matchers fall back to plain
// substring matching.
func NewProvider(debugPrint func(format string, args ...interface{})) *Provider {
	provider := &Provider{debugPrint: debugPrint}
	dict, err := embedict.Load()
	if err != nil {
		provider.debug("Search: migemo disabled err=%v", err)
		return provider
	}
	provider.dict = migemo.MultiClauses(dict)
	provider.debug("Search: migemo enabled")
	return provider
}

// NewPlainProvider creates a provider without migemo. It is useful for tests
// and for callers that need explicit legacy matching behavior.
func NewPlainProvider() *Provider {
	return &Provider{}
}

// Build compiles a matcher for one query. Whitespace-separated query tokens
// must all match, while each token keeps the usual plain-or-migemo behavior.
func (p *Provider) Build(query string) Matcher {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return plainMatcher{}
	}
	if len(tokens) == 1 {
		return p.buildToken(tokens[0])
	}

	matchers := make(allMatcher, 0, len(tokens))
	for _, token := range tokens {
		matchers = append(matchers, p.buildToken(token))
	}
	return matchers
}

func (p *Provider) buildToken(query string) Matcher {
	plain := plainMatcher{queryLower: strings.ToLower(query)}
	if query == "" || p == nil || p.dict == nil {
		return plain
	}

	re, err := migemo.Compile(p.dict, query)
	if err != nil {
		p.debug("Search: migemo compile failed query=%q err=%v", query, err)
		return plain
	}
	return combinedMatcher{plain: plain, migemo: re}
}

func (p *Provider) debug(format string, args ...interface{}) {
	if p != nil && p.debugPrint != nil {
		p.debugPrint(format, args...)
	}
}

type plainMatcher struct {
	queryLower string
}

func (m plainMatcher) Match(candidate string) bool {
	if m.queryLower == "" {
		return true
	}
	return strings.Contains(strings.ToLower(candidate), m.queryLower)
}

type combinedMatcher struct {
	plain  plainMatcher
	migemo interface{ MatchString(string) bool }
}

func (m combinedMatcher) Match(candidate string) bool {
	return m.plain.Match(candidate) || m.migemo.MatchString(candidate)
}

type allMatcher []Matcher

func (m allMatcher) Match(candidate string) bool {
	for _, matcher := range m {
		if !matcher.Match(candidate) {
			return false
		}
	}
	return true
}
