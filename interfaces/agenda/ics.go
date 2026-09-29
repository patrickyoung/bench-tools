package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"
)

func renderICS(out io.Writer, p projection) error {
	quote := func(s string) string {
		return strings.NewReplacer("\\", "\\\\", "\r\n", "\\n", "\n", "\\n", "\r", "\\n", ";", "\\;", ",", "\\,").Replace(s)
	}
	stamp := func(s string) string {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.UTC().Format("20060102T150405Z")
	}
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Bench//Agenda UI " + version + "//EN", "CALSCALE:GREGORIAN", "X-WR-CALNAME:Agenda obligations (snapshot)"}
	for _, r := range p.Obligations {
		if r.DueAt == "" {
			continue
		}
		uid := sha256.Sum256([]byte(r.ID))
		start := r.NotBefore
		if validInstant(start) != nil {
			start = r.DueAt
		}
		lines = append(lines, "BEGIN:VEVENT", "UID:"+hex.EncodeToString(uid[:])+"@agenda", "DTSTAMP:"+stamp(p.AsOf), "DTSTART:"+stamp(start))
		begin, _ := time.Parse(time.RFC3339Nano, start)
		due, _ := time.Parse(time.RFC3339Nano, r.DueAt)
		if due.After(begin) {
			lines = append(lines, "DTEND:"+stamp(r.DueAt))
		}
		description := "Owner: " + r.Owner + "; state: " + r.State + "; acceptance: " + r.Acceptance + "; " + r.Timeliness + "; timezone: " + r.Timezone + "; attention: " + strings.Join(r.Attention, ", ") + "; snapshot: " + p.AsOf + "; coverage: " + p.Coverage
		lines = append(lines, "SUMMARY:"+quote(r.Title), "DESCRIPTION:"+quote(description), "CATEGORIES:"+quote(r.State), "TRANSP:TRANSPARENT")
		if r.State == "cancelled" || r.State == "skipped" {
			lines = append(lines, "STATUS:CANCELLED")
		}
		lines = append(lines, "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")
	for _, line := range lines {
		part := ""
		for _, char := range line {
			if len(part)+len(string(char)) > 75 {
				if _, err := io.WriteString(out, part+"\r\n"); err != nil {
					return err
				}
				part = " "
			}
			part += string(char)
		}
		if _, err := io.WriteString(out, part+"\r\n"); err != nil {
			return err
		}
	}
	return nil
}
