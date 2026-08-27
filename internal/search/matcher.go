package search

import (
	"strings"
	"sync"
	"time"

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
	dictOnce   sync.Once
	dict       migemo.Dict
	loadDict   func() (migemo.Dict, error)
	debugPrint func(format string, args ...interface{})
}

// NewProvider creates a matcher provider that loads gomigemo's embedded
// dictionary on the first non-empty query. If dictionary loading fails,
// returned matchers fall back to plain substring matching.
func NewProvider(debugPrint func(format string, args ...interface{})) *Provider {
	return newProvider(debugPrint, loadEmbeddedDict)
}

// NewPlainProvider creates a provider without migemo. It is useful for tests
// and for callers that need explicit legacy matching behavior.
func NewPlainProvider() *Provider {
	return &Provider{}
}

func newProvider(debugPrint func(format string, args ...interface{}), loadDict func() (migemo.Dict, error)) *Provider {
	return &Provider{
		loadDict:   loadDict,
		debugPrint: debugPrint,
	}
}

func loadEmbeddedDict() (migemo.Dict, error) {
	dict, err := embedict.Load()
	if err != nil {
		return nil, err
	}
	return migemo.MultiClauses(dict), nil
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
	if query == "" {
		return plain
	}
	dict := p.dictionary()
	if dict == nil {
		return plain
	}

	re, err := migemo.Compile(dict, query)
	if err != nil {
		p.debug("Search: migemo compile failed query=%q err=%v", query, err)
		return plain
	}
	return combinedMatcher{plain: plain, migemo: re}
}

func (p *Provider) dictionary() migemo.Dict {
	if p == nil || p.loadDict == nil {
		return nil
	}
	p.dictOnce.Do(func() {
		started := time.Now()
		dict, err := p.loadDict()
		if err != nil {
			p.debug("Search: migemo disabled elapsed=%s err=%v", time.Since(started), err)
			return
		}
		p.dict = dict
		p.debug("Search: migemo enabled elapsed=%s", time.Since(started))
	})
	return p.dict
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
