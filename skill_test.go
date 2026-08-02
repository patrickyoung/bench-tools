package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noDescription() string { return "" }

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAddCreatesASkillAndAppends(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house")
	path := filepath.Join(dir, "SKILL.md")

	n, _, err := add(dir, "house", []string{"- first thing"}, "sess-1", "by-1", noDescription)
	if err != nil || n != 1 {
		t.Fatalf("add = %d, %v", n, err)
	}
	doc := read(t, path)
	for _, want := range []string{"name: house", lessonsHeading, "- first thing", "<!-- hone sess-1 by-1 -->"} {
		if !strings.Contains(doc, want) {
			t.Errorf("new skill is missing %q:\n%s", want, doc)
		}
	}

	if n, _, err = add(dir, "house", []string{"- second thing"}, "sess-2", "by-2", noDescription); err != nil || n != 1 {
		t.Fatalf("second add = %d, %v", n, err)
	}
	doc = read(t, path)
	if !strings.Contains(doc, "- first thing") || !strings.Contains(doc, "- second thing") {
		t.Errorf("appending lost a lesson:\n%s", doc)
	}
	if i, j := strings.Index(doc, "first thing"), strings.Index(doc, "second thing"); i > j {
		t.Error("lessons are not in the order they were learned")
	}
}

// A run teaches once. Without this, a loop over an archive run twice
// doubles the corpus and adds nothing, which is the unbounded-growth
// failure exactly.
func TestARunTeachesOnce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house")
	if _, _, err := add(dir, "house", []string{"- a thing"}, "sess-1", "by-1", noDescription); err != nil {
		t.Fatal(err)
	}
	if !taught(dir, "sess-1") {
		t.Error("taught() does not see what was just written")
	}
	if taught(dir, "sess-2") {
		t.Error("taught() claims a run it never saw")
	}
	// And a short id is matched as a field, never as a substring: `ask -f
	// pass.jsonl` makes a session called "pass".
	if taught(dir, "s") {
		t.Error(`"s" matched "sess-1" as a substring`)
	}
}

func TestAddDoesNotRepeatItself(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house")
	if _, _, err := add(dir, "house", []string{"- Use tabs, not spaces."}, "s1", "b1", noDescription); err != nil {
		t.Fatal(err)
	}
	// Same words, different punctuation and case, from another run.
	n, _, err := add(dir, "house", []string{"- use tabs not spaces"}, "s2", "b2", noDescription)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("added %d near-duplicates, want 0:\n%s", n, read(t, filepath.Join(dir, "SKILL.md")))
	}
	// Two lessons in one reply that say the same thing: the second must see
	// the first.
	dir2 := filepath.Join(t.TempDir(), "h2")
	n, _, err = add(dir2, "h2", []string{"- Use tabs.", "- use tabs"}, "s1", "b1", noDescription)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("added %d, want 1: a reply repeated itself and both landed", n)
	}
}

// A skill can say things after its lessons. Appending must not land past
// them, or the section a human wrote stops being where they put it.
func TestAddInsertsInsideTheSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte(`---
name: x
description: d
---

`+lessonsHeading+`

- old lesson
<!-- hone s0 b0 -->

## Notes

Written by a person, and it stays at the bottom.
`), 0o644)

	if _, _, err := add(dir, "x", []string{"- new lesson"}, "s1", "b1", noDescription); err != nil {
		t.Fatal(err)
	}
	doc := read(t, path)
	newAt, notesAt := strings.Index(doc, "new lesson"), strings.Index(doc, "## Notes")
	if newAt < 0 || notesAt < 0 {
		t.Fatalf("lost something:\n%s", doc)
	}
	if newAt > notesAt {
		t.Errorf("the lesson landed after a section it does not belong to:\n%s", doc)
	}
	if !strings.Contains(doc, "Written by a person") {
		t.Errorf("a human's section was damaged:\n%s", doc)
	}
}

// forget is the answer to a lesson that turned out to be wrong. The unit is
// a run, because a session that taught one wrong thing usually taught its
// neighbours too.
func TestForgetRemovesALessonAndItsMark(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house")
	path := filepath.Join(dir, "SKILL.md")
	if _, _, err := add(dir, "house", []string{"- keep me"}, "good", "b1", noDescription); err != nil {
		t.Fatal(err)
	}
	if _, _, err := add(dir, "house", []string{"- wrong thing", "- also wrong"}, "bad", "b2", noDescription); err != nil {
		t.Fatal(err)
	}

	n, err := forget(path, "bad")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("forgot %d, want 2", n)
	}
	doc := read(t, path)
	if !strings.Contains(doc, "keep me") {
		t.Errorf("forget took a lesson from another run:\n%s", doc)
	}
	for _, gone := range []string{"wrong thing", "also wrong", "<!-- hone bad"} {
		if strings.Contains(doc, gone) {
			t.Errorf("%q survived forget:\n%s", gone, doc)
		}
	}
	if n, _ := forget(path, "bad"); n != 0 {
		t.Errorf("forgetting twice removed %d more", n)
	}
}

func TestParseLessons(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"none is a real answer", "none", nil},
		{"none with a full stop", "None.", nil},
		{"empty", "   \n  ", nil},
		{"one item", "- a thing", []string{"- a thing"}},
		{"a star is a bullet", "* a thing", []string{"- a thing"}},
		{"two items", "- one\n- two", []string{"- one", "- two"}},
		{
			name: "a wrapped item stays one item",
			in:   "- a long thing\n  that wrapped",
			want: []string{"- a long thing\n  that wrapped"},
		},
		{"prose is not a lesson", "I could not find anything useful here.", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parse(tc.in, 3)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// -n is a cap, and a cap that is not enforced is a comment.
func TestParseHonoursTheCap(t *testing.T) {
	got := parse("- one\n- two\n- three\n- four\n- five", 2)
	if len(got) != 2 {
		t.Fatalf("got %d lessons past a cap of 2", len(got))
	}
}

func TestScaffoldSaysWhenTheDescriptionIsPoor(t *testing.T) {
	// A skill nothing can find is worse than useless: the lessons were paid
	// for and will never be read. If a description could not be written, the
	// file has to say so where somebody will see it.
	doc := scaffold("house", "")
	if !strings.Contains(doc, "will not be found") {
		t.Errorf("the fallback description does not admit the problem:\n%s", doc)
	}
	doc = scaffold("house", "Go build conventions here; use when adding Go files.")
	if strings.Contains(doc, "will not be found") {
		t.Errorf("a good description was overwritten with the apology:\n%s", doc)
	}
}

// TestTheTotalIsThePressureGauge: DESIGN.md specified `1 lesson added (4
// total)` and the code printed only the first half, so the gauge AGENTS.md
// describes -- a skill accumulating lessons faster than it is used is a
// procedure missing a step -- was described and never measured. The total
// is the count of marks, because every lesson carries one and nothing else
// does.
func TestTheTotalIsThePressureGauge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house")
	for i, want := range []int{1, 2, 3} {
		n, held, err := add(dir, "house",
			[]string{"- lesson number " + string(rune('a'+i))},
			"sess-"+string(rune('1'+i)), "by-1", noDescription)
		if err != nil || n != 1 {
			t.Fatalf("add = %d, %v", n, err)
		}
		if held != want {
			t.Errorf("total = %d, want %d after %d writes", held, want, i+1)
		}
	}
	// A lesson a human wrote by hand carries no mark and is not hone's to
	// count. The gauge measures what hone put there.
	path := filepath.Join(dir, "SKILL.md")
	write(t, path, read(t, path)+"\n- a human wrote this one\n", 0o644)
	_, held, err := add(dir, "house", []string{"- a fourth"}, "sess-9", "by-1", noDescription)
	if err != nil {
		t.Fatal(err)
	}
	if held != 4 {
		t.Errorf("total = %d, want 4: an unmarked line was counted", held)
	}
}

// TestASkillIsReplacedAtomically: os.WriteFile truncates and then writes, so
// a crash between the two leaves a curated skill empty. contrib/edit in the
// sibling repository already does this properly; the corpus this program
// exists to keep true deserves at least as much care.
func TestASkillIsReplacedAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	write(t, path, "original\n", 0o644)

	// A destination that cannot be replaced must leave the original alone
	// rather than truncating it first and failing afterwards.
	if err := save(filepath.Join(dir, "nosuchdir", "SKILL.md"), "new"); err == nil {
		t.Error("save into a missing directory should fail")
	}
	if got := read(t, path); got != "original\n" {
		t.Errorf("the original changed: %q", got)
	}

	if err := save(path, "replaced\n"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "replaced\n" {
		t.Errorf("read back %q", got)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644: a skill is meant to be read", fi.Mode().Perm())
	}
	// The temp file must not survive as a sibling: a directory of .hone.*
	// leftovers is litter in somebody's skills tree.
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".hone.") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
}
