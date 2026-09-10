package main

import (
	"bytes"
	"fmt"
)

func checkCandidate(candidate []byte, evidence map[string]sourceRef) []string {
	if len(bytes.TrimSpace(candidate)) == 0 {
		return []string{"candidate is empty"}
	}
	validCount := 0
	var problems []string
	for from := 0; from < len(candidate); {
		i := bytes.Index(candidate[from:], []byte("ctx:"))
		if i < 0 {
			break
		}
		at := from + i
		ref := refAt(candidate, at)
		item, ok := evidence[ref]
		expected := []byte("[" + ref + "](" + item.url + ")")
		if ok && item.url != "" && at > 0 && bytes.HasPrefix(candidate[at-1:], expected) {
			validCount++
			from = at - 1 + len(expected)
			continue
		}
		switch {
		case ref == "":
			problems = append(problems, "malformed ctx ref")
		case !ok:
			problems = append(problems, fmt.Sprintf("unknown ref %q", ref))
		case item.url == "":
			problems = append(problems,
				fmt.Sprintf("ref %q has no citation.url in evidence", ref))
		default:
			problems = append(problems, fmt.Sprintf(
				"ref %q must appear exactly as [%s](%s)", ref, ref, item.url))
		}
		from = at + len("ctx:")
	}
	if validCount == 0 {
		problems = append(problems, "candidate has no valid citations")
	}
	return unique(problems)
}

func refAt(candidate []byte, start int) string {
	end := start
	for end < len(candidate) && refByte(candidate[end]) {
		end++
	}
	ref := string(candidate[start:end])
	if !validRef(ref) {
		return ""
	}
	return ref
}

func refByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' || b == ':' || b == '.' || b == '_' || b == '-'
}

func unique(items []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
