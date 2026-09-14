package event

import (
	"path/filepath"
	"testing"
)

func TestIncrementalSealsMatchCanonicalPrefixAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefix.jsonl")
	l, err := CreateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for phase := 0; phase < 3; phase++ {
		if phase > 0 {
			l, _, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := l.Append(Note, NoteData{Source: "fixture", Text: "unsealed text <>&\n"}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 8; i++ {
			if _, err := l.AppendSealed(Note, NoteData{Source: "fixture", Kind: "bytes/v1", Body: []byte(`{"value":9007199254740993,"bytes":"AP8="}`)}); err != nil {
				t.Fatal(err)
			}
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		es, err := ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range es {
			if e.Type == Seal {
				s, err := As[SealData](e)
				if err != nil {
					t.Fatal(err)
				}
				want, err := PrefixDigest(es[:i])
				if err != nil {
					t.Fatal(err)
				}
				if s.SHA256 != want {
					t.Fatalf("seal %d changed format: got %s want %s", i, s.SHA256, want)
				}
			}
		}
		if err := Check(es); err != nil {
			t.Fatal(err)
		}
	}
}
