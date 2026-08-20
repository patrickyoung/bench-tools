package main

import (
	"strings"
	"unicode"
)

func semanticText(e event) (string, error) {
	var pieces []string
	switch e.Type {
	case "user":
		var d userData
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		if d.Source != "" {
			pieces = append(pieces, d.Source)
		}
		if len(d.Blocks) == 0 {
			pieces = appendText(pieces, d.Text)
		} else {
			pieces = appendBlocks(pieces, d.Blocks)
		}
	case "assistant":
		var d turn
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		pieces = appendBlocks(pieces, d.Blocks)
	case "request":
		var d requestData
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		pieces = appendText(pieces, d.Model)
		pieces = appendText(pieces, d.Effort)
	case "note":
		var d noteData
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		pieces = appendText(pieces, d.Source)
		pieces = appendText(pieces, d.Text)
	case "retry":
		var d retryData
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		pieces = appendText(pieces, d.Error)
	case "done":
		var d doneData
		if err := decodeEvent(e, &d); err != nil {
			return "", err
		}
		pieces = appendText(pieces, d.Reason)
		pieces = appendText(pieces, d.Error)
	case "session", "abort":
		// Structural. The archive summary carries header metadata; find does not
		// make every session match a shared system prompt.
	default:
		// Unknown events remain visible through show/window. Guessing which of
		// their fields are semantic would make a plugin's opaque state searchable.
	}
	return strings.Join(pieces, "\n"), nil
}

func appendBlocks(dst []string, blocks []block) []string {
	for _, b := range blocks {
		switch b.Type {
		case "text", "reasoning":
			dst = appendText(dst, b.Text)
		case "media":
			dst = appendText(dst, b.Name)
			dst = appendText(dst, b.MediaType)
		case "opaque":
			// Provider-native state and signatures are replay data, not text.
		}
	}
	return dst
}

func appendText(dst []string, text string) []string {
	if text != "" {
		return append(dst, text)
	}
	return dst
}

// foldForMatch makes formatting differences uninteresting without changing
// the text a match record prints. unicode.IsSpace includes newlines and the
// non-ASCII spaces a copied document can carry; ToLower is Unicode-aware.
func foldForMatch(s string) string {
	var b strings.Builder
	space := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
