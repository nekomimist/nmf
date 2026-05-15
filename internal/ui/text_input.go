package ui

import "unicode/utf8"

func trimLastRune(text string) string {
	if text == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(text)
	if size <= 0 {
		return ""
	}
	return text[:len(text)-size]
}
