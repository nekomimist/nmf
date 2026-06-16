package shellmenu

import (
	"strings"
	"unicode"
)

type menuDedupeEntry struct {
	position  int
	commandID uintptr
	label     string
	verb      string
	separator bool
	enabled   bool
}

func duplicateMenuPositions(entries []menuDedupeEntry) []int {
	keepByVerb := make(map[string]int)
	keepByLabel := make(map[string]int)
	remove := make(map[int]struct{})

	for i, entry := range entries {
		verb, label := menuDedupeKeys(entry)
		if verb == "" && label == "" {
			continue
		}
		kept, ok := findKeptMenuEntry(keepByVerb, keepByLabel, verb, label)
		if ok {
			keptEntry := entries[kept]
			if !keptEntry.enabled && entry.enabled {
				remove[keptEntry.position] = struct{}{}
				forgetMenuEntry(keepByVerb, keepByLabel, keptEntry, kept)
				rememberMenuEntry(keepByVerb, keepByLabel, verb, label, i)
				continue
			}
			remove[entry.position] = struct{}{}
			continue
		}
		rememberMenuEntry(keepByVerb, keepByLabel, verb, label, i)
	}

	positions := make([]int, 0, len(remove))
	for pos := range remove {
		positions = append(positions, pos)
	}
	sortDescending(positions)
	return positions
}

func menuDedupeKeys(entry menuDedupeEntry) (verb string, label string) {
	if entry.separator {
		return "", ""
	}
	if verb := strings.TrimSpace(strings.ToLower(entry.verb)); verb != "" {
		return verb, normalizeMenuLabel(entry.label)
	}
	return "", normalizeMenuLabel(entry.label)
}

func findKeptMenuEntry(keepByVerb map[string]int, keepByLabel map[string]int, verb string, label string) (int, bool) {
	if verb != "" {
		if kept, ok := keepByVerb[verb]; ok {
			return kept, true
		}
	}
	if label != "" {
		if kept, ok := keepByLabel[label]; ok {
			return kept, true
		}
	}
	return 0, false
}

func rememberMenuEntry(keepByVerb map[string]int, keepByLabel map[string]int, verb string, label string, index int) {
	if verb != "" {
		keepByVerb[verb] = index
	}
	if label != "" {
		keepByLabel[label] = index
	}
}

func forgetMenuEntry(keepByVerb map[string]int, keepByLabel map[string]int, entry menuDedupeEntry, index int) {
	verb, label := menuDedupeKeys(entry)
	if verb != "" && keepByVerb[verb] == index {
		delete(keepByVerb, verb)
	}
	if label != "" && keepByLabel[label] == index {
		delete(keepByLabel, label)
	}
}

func normalizeMenuLabel(label string) string {
	label = stripMenuMnemonics(label)
	label = strings.NewReplacer("...", " ", "\u2026", " ", "\t", " ").Replace(label)
	label = strings.TrimSpace(label)
	label = stripTrailingAccelerator(label)
	fields := strings.FieldsFunc(label, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(strings.Join(fields, " "))
}

func stripMenuMnemonics(label string) string {
	var b strings.Builder
	for i := 0; i < len(label); i++ {
		if label[i] != '&' {
			b.WriteByte(label[i])
			continue
		}
		if i+1 < len(label) && label[i+1] == '&' {
			b.WriteByte('&')
			i++
		}
	}
	return b.String()
}

func stripTrailingAccelerator(label string) string {
	label = strings.TrimSpace(label)
	if len(label) < 3 || !strings.HasSuffix(label, ")") {
		return label
	}
	open := strings.LastIndex(label, "(")
	if open < 0 {
		return label
	}
	accel := strings.TrimSpace(label[open+1 : len(label)-1])
	if len([]rune(accel)) != 1 {
		return label
	}
	return strings.TrimSpace(label[:open])
}

func separatorCleanupPositions(entries []menuDedupeEntry, removed map[int]struct{}) []int {
	positions := make([]int, 0)
	prevSeparator := true
	remaining := 0

	for _, entry := range entries {
		if _, ok := removed[entry.position]; ok {
			continue
		}
		if entry.separator {
			if prevSeparator {
				positions = append(positions, entry.position)
				removed[entry.position] = struct{}{}
				continue
			}
			prevSeparator = true
			remaining++
			continue
		}
		prevSeparator = false
		remaining++
	}

	if remaining > 0 {
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			if _, ok := removed[entry.position]; ok {
				continue
			}
			if !entry.separator {
				break
			}
			positions = append(positions, entry.position)
			removed[entry.position] = struct{}{}
		}
	}

	sortDescending(positions)
	return positions
}

func sortDescending(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] > values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
