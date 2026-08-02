// Writing into a skill.
//
// brief owns the format: the frontmatter, the specification's limits, and
// the lint that holds a skill to them. hone owns one heading inside the
// body and nothing else, and checks its work by running brief afterwards.
// That is the whole seam, and it is why neither program has a copy of the
// other's rules.
//
// Every write is a delta. A model handed its own accumulated notes and
// asked to produce the next version does not edit them, it rewrites them --
// and rewriting compresses away the specifics that made them worth keeping.
// ACE named that context collapse and measured it; the answer is to append
// what is new and never to regenerate what is there. So hone appends, and
// refining is somebody else's verb.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// lessonsHeading is the one part of a skill body hone claims. A skill can
// say anything it likes above it; below it is a list hone maintains.
const lessonsHeading = "## Lessons"

// mark is the provenance of one lesson, and the reason a bad one can be
// deleted rather than argued with.
//
// Both ids replay. The first names the run the lesson was learned from, the
// second the call that worded it, and `ask replay -check` proves either.
// The security literature's finding was that a poisoned memory survives
// correction -- 100% relapse when teams tried to fix one by telling the
// agent it was wrong -- because there was nothing to point at. This is the
// thing to point at.
//
// It is an HTML comment because a skill is markdown that somebody reads: it
// is invisible when rendered, greppable when not, and it travels with the
// text when the text is copied somewhere else.
func mark(from, by string) string {
	return fmt.Sprintf("<!-- hone %s %s -->", from, by)
}

// total counts the lessons in a document, which is the count of marks in
// it: every lesson hone writes carries one, and nothing else does.
func total(doc string) int { return strings.Count(doc, "<!-- hone ") }

// save replaces a skill in one step, or leaves it exactly as it was.
//
// os.WriteFile truncates and then writes, so a crash between the two ends
// with somebody's curated skill empty. That is a poor trade for a file this
// program's whole thesis says must stay true, and the fix is the one every
// Unix program has used for this: write a sibling, rename over the target.
// rename(2) is atomic within a directory, so a reader sees the old file or
// the new one and never half of either.
func save(path, doc string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".hone.")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // a no-op once the rename has succeeded
	if _, err := f.WriteString(doc); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// CreateTemp makes 0600; a skill is meant to be read.
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// fallbackDescription is used when a model could not be reached to write a
// better one. It is deliberately poor and deliberately honest: brief ranks
// over descriptions, so this skill will be hard to find, and the line says
// what to do about that rather than pretending otherwise.
const fallbackDescription = "Lessons learned from work in this tree, recorded by hone. " +
	"Use when working here. Replace this line with what the skill actually " +
	"covers -- brief finds a skill by its description, so this one will not be found."

// scaffold is the skill hone writes when -into names one that does not
// exist. It passes `brief lint -strict` on the way out, because the first
// file an author sees teaches them the shape.
func scaffold(name, description string) string {
	if strings.TrimSpace(description) == "" {
		description = fallbackDescription
	}
	return fmt.Sprintf(`---
name: %s
description: %s
---

# %s

Each item below was learned from one run that failed, was fixed, and was
then confirmed done by a program. The comment after a lesson names the
session it came from and the call that worded it; `+"`ask replay -check`"+`
proves either.

%s
`, name, description, name, lessonsHeading)
}

// taught reports whether a skill already holds what a run taught.
//
// A run teaches once. Without this, honing the same session twice writes
// the same lesson twice in slightly different words -- the model does not
// word it identically, so no text matcher will catch it -- and a loop run
// over an archive on Tuesday and again on Friday doubles the corpus while
// adding nothing. That is the unbounded-growth failure exactly: an
// add-everything agent reached 2,400 records and 13% accuracy where a
// selective one held 248 and reached 39%.
//
// It is checked before the model is called, so a second pass over an
// archive is free as well as harmless.
func taught(dir, sessionID string) bool {
	body, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(body), "\n") {
		if isMarkFor(line, sessionID) {
			return true
		}
	}
	return false
}

// add folds lessons into a skill and reports how many landed and how many
// the skill then holds. It is append-only within the heading, and it reads
// the file whole because a SKILL.md is small and a partial read is a subtler
// bug than it is worth avoiding.
//
// The total is the pressure gauge. A skill accumulating lessons faster than
// it is used is not a richer skill, it is a procedure missing a step -- and
// the number is free here, because add is already holding the document it
// just wrote. Counting *uses* would be the truer signal and would need brief
// to keep a counter, which is a cache, which brief does not have and should
// not grow. Count the thing that costs nothing.
//
// describe writes the line a new skill is found by. It is called only when
// one is being created, so an existing skill's description is never
// touched: what a human curated stays curated.
func add(dir, name string, lessons []string, from, by string, describe func() string) (int, int, error) {
	path := filepath.Join(dir, "SKILL.md")
	body, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, 0, err
		}
		body = []byte(scaffold(name, describe()))
	case err != nil:
		return 0, 0, err
	}

	doc := string(body)
	if !strings.Contains(doc, lessonsHeading) {
		doc = strings.TrimRight(doc, "\n") + "\n\n" + lessonsHeading + "\n"
	}

	added := 0
	for _, l := range lessons {
		// Against the growing document, not the one read from disk: two
		// lessons in one reply can say the same thing, and the second must
		// see the first.
		if have(doc, l) {
			continue
		}
		doc = insert(doc, l+"\n"+mark(from, by))
		added++
	}
	if added == 0 {
		return 0, 0, nil
	}
	if err := save(path, doc); err != nil {
		return 0, 0, err
	}
	return added, total(doc), nil
}

// insert puts a lesson at the end of the lessons section, which is the end
// of the file unless somebody has written below it. Appending at the end of
// the section rather than the end of the file is what keeps a skill that
// says something after its lessons still saying it.
func insert(doc, item string) string {
	i := strings.Index(doc, lessonsHeading)
	if i < 0 {
		return strings.TrimRight(doc, "\n") + "\n" + item
	}
	rest := doc[i+len(lessonsHeading):]
	// The next heading at the same level or above ends the section.
	end := len(doc)
	for _, line := range lineOffsets(rest) {
		t := rest[line.start:line.end]
		if strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "# ") {
			end = i + len(lessonsHeading) + line.start
			break
		}
	}
	head := strings.TrimRight(doc[:end], "\n")
	tail := doc[end:]
	if strings.TrimSpace(tail) != "" {
		return head + "\n\n" + item + "\n\n" + tail
	}
	return head + "\n\n" + item + "\n"
}

type span struct{ start, end int }

func lineOffsets(s string) []span {
	var out []span
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			out = append(out, span{start, i})
			start = i + 1
		}
	}
	return out
}

// have decides whether a skill already says this. It is deliberately crude:
// exact text, or the same content words in the same order once punctuation
// and case are gone. A near-duplicate that slips through costs a line; a
// clever matcher that drops a real lesson costs the lesson, and nothing
// says it happened.
func have(doc, lesson string) bool {
	want := normalize(lesson)
	if want == "" {
		return true
	}
	for _, line := range strings.Split(doc, "\n") {
		if normalize(line) == want {
			return true
		}
	}
	return strings.Contains(doc, strings.TrimSpace(lesson))
}

func normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "*")
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '\t':
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// forget removes every lesson a run taught, and the marks that named it. It
// takes the session id because that is what a mark records, and because the
// unit of a mistake is a run: a session that taught one wrong thing usually
// taught its neighbours too.
func forget(path, sessionID string) (int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(body), "\n")
	var out []string
	removed := 0
	for i := 0; i < len(lines); i++ {
		if !isMarkFor(lines[i], sessionID) {
			out = append(out, lines[i])
			continue
		}
		// The mark follows its lesson, so walk back over the lesson's lines
		// and drop them with it. A lesson is a list item and its
		// continuations, which ends at the blank line before it.
		for len(out) > 0 {
			last := strings.TrimSpace(out[len(out)-1])
			out = out[:len(out)-1]
			if strings.HasPrefix(last, "-") || strings.HasPrefix(last, "*") {
				break
			}
			if last == "" {
				break
			}
		}
		removed++
	}
	if removed == 0 {
		return 0, nil
	}
	doc := strings.Join(out, "\n")
	for strings.Contains(doc, "\n\n\n") {
		doc = strings.ReplaceAll(doc, "\n\n\n", "\n\n")
	}
	return removed, save(path, doc)
}

// isMarkFor matches a mark by its session field exactly, never as a
// substring. Session ids are usually long and unique, but `ask -f
// pass.jsonl` makes one called "pass", and a substring match would then
// have "pass" matching every mark with the word in it.
func isMarkFor(line, sessionID string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "<!-- hone ") || !strings.HasSuffix(t, "-->") {
		return false
	}
	f := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(t, "<!--"), "-->"))
	if len(f) < 2 || f[0] != "hone" {
		return false
	}
	return sessionID == "" || f[1] == sessionID
}

// parse pulls the lessons out of what a model returned. It accepts the list
// it was asked for and nothing else: a reply that is prose, or that says
// none, yields nothing, and nothing is an ordinary answer.
func parse(reply string, max int) []string {
	reply = strings.TrimSpace(reply)
	if reply == "" || strings.EqualFold(reply, "none") || strings.EqualFold(reply, "none.") {
		return nil
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimRight(cur.String(), " \n"); strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for _, line := range strings.Split(reply, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "):
			flush()
			cur.WriteString("- " + strings.TrimSpace(t[2:]))
		case t == "":
			flush()
		case cur.Len() > 0:
			cur.WriteString("\n  " + t)
		}
	}
	flush()
	if len(out) > max {
		out = out[:max]
	}
	return out
}
