package main

import (
	"fmt"
	"strings"
	"unicode"
)

func buildExternalCommandLine(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	if strings.TrimSpace(command) != "" {
		parts = append(parts, quoteExternalCommandWord(command))
	}
	for _, arg := range args {
		parts = append(parts, quoteExternalCommandWord(arg))
	}
	return strings.Join(parts, " ")
}

func quoteExternalCommandWord(word string) string {
	if word == "" {
		return "''"
	}
	needsQuote := false
	for _, r := range word {
		if unicode.IsSpace(r) || strings.ContainsRune(`'"\\`, r) {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func parseExternalCommandLine(line string) (string, []string, error) {
	words, err := splitExternalCommandLine(line)
	if err != nil {
		return "", nil, err
	}
	if len(words) == 0 {
		return "", nil, fmt.Errorf("command line must not be empty")
	}
	return words[0], words[1:], nil
}

func splitExternalCommandLine(line string) ([]string, error) {
	var words []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	wordStarted := false

	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			wordStarted = true
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			wordStarted = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				wordStarted = true
				continue
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
				wordStarted = true
				continue
			}
		}
		if unicode.IsSpace(r) && !inSingle && !inDouble {
			if wordStarted {
				words = append(words, current.String())
				current.Reset()
				wordStarted = false
			}
			continue
		}
		current.WriteRune(r)
		wordStarted = true
	}
	if escaped {
		return nil, fmt.Errorf("command line ends with escape")
	}
	if inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}
	if wordStarted {
		words = append(words, current.String())
	}
	return words, nil
}
