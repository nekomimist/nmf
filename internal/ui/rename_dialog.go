package ui

const renameDisplayedNameMax = 56

func middleEllipsizeFileName(name string, maxRunes int) string {
	runes := []rune(name)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return name
	}
	const marker = "..."
	markerLen := len([]rune(marker))
	if maxRunes <= markerLen {
		return string(runes[:maxRunes])
	}

	available := maxRunes - markerLen
	prefixLen := available / 3
	if prefixLen < 1 {
		prefixLen = 1
	}
	suffixLen := available - prefixLen
	return string(runes[:prefixLen]) + marker + string(runes[len(runes)-suffixLen:])
}
