package bitmap_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var root, helper, fake, author string

func TestMain(m *testing.M) {
	root = os.Getenv("BITMAP_TEST_ROOT")
	if root == "" {
		panic("set BITMAP_TEST_ROOT to physical authoring root")
	}
	helper = filepath.Join(root, "install/bin/bitmap")
	fake = filepath.Join(root, "install/fixtures/generator")
	author = filepath.Join(root, "install/fixtures")
	for _, p := range []string{helper, fake, filepath.Join(author, "agent")} {
		if _, e := os.Stat(p); e != nil {
			panic(e)
		}
	}
	os.Exit(m.Run())
}
func get(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func put(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func j(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func command(t *testing.T, dir string, env []string, program string, args ...string) (int, []byte) {
	t.Helper()
	c := exec.Command(program, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	b, e := c.CombinedOutput()
	if e == nil {
		return 0, b
	}
	if x, ok := e.(*exec.ExitError); ok {
		return x.ExitCode(), b
	}
	t.Fatal(e)
	return -1, b
}
func accept(t *testing.T, dir string, env []string, p string, args ...string) []byte {
	t.Helper()
	code, b := command(t, dir, env, p, args...)
	if code != 0 {
		t.Fatalf("%s %v: exit %d\n%s", p, args, code, b)
	}
	return b
}
func reject(t *testing.T, dir string, env []string, p string, args ...string) {
	t.Helper()
	code, b := command(t, dir, env, p, args...)
	if code == 0 {
		t.Fatalf("unexpected acceptance: %s %v\n%s", p, args, b)
	}
}
func rect(x, y, w, h int, label string) map[string]any {
	return map[string]any{"x": x, "y": y, "width": w, "height": h, "label": label}
}

type fixture struct {
	w, c, log string
	env       []string
	brief     map[string]any
}

func setup(t *testing.T, style string, regions []map[string]any) fixture {
	t.Helper()
	base := t.TempDir()
	if evidence := os.Getenv("BITMAP_TEST_EVIDENCE"); evidence != "" {
		var err error
		base, err = os.MkdirTemp(evidence, strings.ReplaceAll(t.Name(), "/", "-")+"-")
		if err != nil {
			t.Fatal(err)
		}
	}
	base, e := filepath.EvalSymlinks(base)
	if e != nil {
		t.Fatal(e)
	}
	w := filepath.Join(base, "work")
	if e = os.MkdirAll(filepath.Join(w, "inputs"), 0700); e != nil {
		t.Fatal(e)
	}
	m := image.NewNRGBA64(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			m.SetNRGBA64(x, y, color.NRGBA64{uint16(50000 - x*500), uint16(8000 + y*600), 30000, 65535})
		}
	}
	// A tiny literal "42" label drawn as known raster glyphs, not generated facts.
	for y := 2; y < 10; y++ {
		for x := 2; x < 14; x++ {
			m.SetNRGBA64(x, y, color.NRGBA64{65535, 65535, 65535, 65535})
		}
	}
	for n, g := range []string{"101101111001001", "111001111100111"} {
		for i, v := range g {
			if v == '1' {
				m.SetNRGBA64(3+n*5+i%3, 3+i/3, color.NRGBA64{0, 0, 0, 65535})
			}
		}
	}
	// Nonopaque samples exercise exact straight-alpha 16-bit copying.
	m.SetNRGBA64(2, 2, color.NRGBA64{12345, 34567, 56789, 22222})
	m.SetNRGBA64(3, 2, color.NRGBA64{1000, 2000, 3000, 0})
	var buf bytes.Buffer
	if e = png.Encode(&buf, m); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(w, "inputs/source.png"), buf.Bytes())
	b := map[string]any{"version": 1, "source": "inputs/source.png", "style": style}
	if regions != nil {
		b["preserve_regions"] = regions
	}
	put(t, filepath.Join(w, "brief.json"), j(b))
	f := fixture{w: w, c: filepath.Join(base, "control"), log: filepath.Join(base, "calls"), brief: b}
	f.env = []string{"BITMAP_HELPER=" + helper, "BITMAP_GENERATOR=" + fake, "BITMAP_AUTHOR_MODEL=offline-fixture", "PATH=" + author + ":" + os.Getenv("PATH"), "FAKE_AGENT_MODE=", "FAKE_BACKEND_MODE=", "FAKE_CALL_LOG=" + f.log}
	return f
}
func (f fixture) run(t *testing.T, mode string, code int) {
	t.Helper()
	env := append(append([]string{}, f.env...), "FAKE_BACKEND_MODE="+mode)
	c := exec.Command(filepath.Join(root, "expert/bin/stylize"), f.w, f.c)
	c.Dir = root
	c.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	got := 0
	if err != nil {
		if ex, ok := err.(*exec.ExitError); ok {
			got = ex.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	b := append(append([]byte{}, stdout.Bytes()...), stderr.Bytes()...)
	put(t, filepath.Join(filepath.Dir(f.w), "adapter.stdout"), stdout.Bytes())
	put(t, filepath.Join(filepath.Dir(f.w), "adapter.stderr"), stderr.Bytes())
	if got == 0 {
		var artifact map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &artifact); err != nil {
			t.Fatalf("stdout is not one artifact JSON: %s", stdout.Bytes())
		}
		if artifact["final"] != filepath.Join(f.c, "final.png") {
			t.Fatal(artifact)
		}
	} else if stdout.Len() != 0 {
		t.Fatalf("failed adapter emitted success stdout: %s", stdout.Bytes())
	}
	if got != code {
		t.Fatalf("mode %s expected %d got %d\n%s", mode, code, got, b)
	}
	if code == 0 {
		accept(t, root, env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
		if string(get(t, f.log)) != "call\n" {
			t.Fatal("generator not called exactly once")
		}
	}
}
func decode(t *testing.T, p string) image.Image {
	t.Helper()
	m, e := png.Decode(bytes.NewReader(get(t, p)))
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func straight(c color.Color) color.NRGBA64 {
	switch v := c.(type) {
	case color.NRGBA64:
		return v
	case color.NRGBA:
		return color.NRGBA64{uint16(v.R) * 257, uint16(v.G) * 257, uint16(v.B) * 257, uint16(v.A) * 257}
	}
	return color.NRGBA64Model.Convert(c).(color.NRGBA64)
}
func assertPixels(t *testing.T, f fixture, regions []map[string]any) {
	t.Helper()
	src := decode(t, filepath.Join(f.w, "inputs/source.png"))
	out := decode(t, filepath.Join(f.c, "final.png"))
	if out.Bounds() != src.Bounds() {
		t.Fatal("dimensions changed")
	}
	changed := 0
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			protected := false
			for _, r := range regions {
				rx, ry, rw, rh := r["x"].(int), r["y"].(int), r["width"].(int), r["height"].(int)
				if x >= rx && x < rx+rw && y >= ry && y < ry+rh {
					protected = true
				}
			}
			same := straight(src.At(x, y)) == straight(out.At(x, y))
			if protected && !same {
				t.Fatalf("protected pixel changed at %d,%d: %v vs %v", x, y, src.At(x, y), out.At(x, y))
			}
			if !protected && !same {
				changed++
			}
		}
	}
	if changed == 0 {
		t.Fatal("art unchanged")
	}
}
func TestPositiveIndependentBindings(t *testing.T) {
	a := []map[string]any{rect(2, 2, 12, 8, "factual 42")}
	f := setup(t, "expressive anime cel planes", a)
	f.run(t, "resize", 0)
	assertPixels(t, f, a)
	var r map[string]any
	if e := json.Unmarshal(get(t, filepath.Join(f.c, "receipt.json")), &r); e != nil {
		t.Fatal(e)
	}
	if r["normalization"] != "lanczos3-premultiplied-separable" {
		t.Fatal(r)
	}
	b := []map[string]any{rect(2, 2, 12, 8, "factual 42"), rect(1, 1, 5, 4, "overlap margin")}
	g := setup(t, "woodcut hatching", b)
	g.run(t, "", 0)
	assertPixels(t, g, b)
	// A valid request from the first case must not validate against the second brief.
	p := filepath.Join(g.w, "output/request.json")
	old := get(t, p)
	put(t, p, get(t, filepath.Join(f.w, "output/request.json")))
	reject(t, g.w, g.env, filepath.Join(root, "expert/bin/check"))
	put(t, p, old)
	// Fresh-control policy never overwrites an existing run.
	reject(t, root, g.env, filepath.Join(root, "expert/bin/stylize"), g.w, g.c)
}
func TestZeroRegions(t *testing.T) {
	f := setup(t, "ink wash", []map[string]any{})
	f.run(t, "", 0)
	assertPixels(t, f, nil)
}
func TestBackendFailures(t *testing.T) {
	for _, tc := range []struct {
		mode string
		code int
	}{{"label-only", 1}, {"same", 1}, {"missing", 1}, {"malformed-provenance", 1}, {"empty-provenance", 1}, {"truncated", 1}, {"bomb", 1}, {"symlink", 1}, {"aspect", 1}, {"fail37", 37}, {"fail75", 75}, {"fail125", 125}} {
		t.Run(tc.mode, func(t *testing.T) {
			f := setup(t, "anime", []map[string]any{rect(2, 2, 12, 8, "42")})
			f.run(t, tc.mode, tc.code)
			if string(get(t, f.log)) != "call\n" {
				t.Fatal("retry or missing generator call")
			}
			if tc.code > 1 {
				// Public Record must be complete even when no output PNG exists.
				accept(t, root, nil, "record", "check", "-f", filepath.Join(f.c, "generation/record.jsonl"))
				code, _ := command(t, root, nil, "record", "replay", "-f", filepath.Join(f.c, "generation/record.jsonl"))
				if code != tc.code {
					t.Fatalf("recorded exit %d != %d", code, tc.code)
				}
			}
			if _, e := os.Stat(filepath.Join(f.c, "receipt.json")); !os.IsNotExist(e) {
				t.Fatal("failed contribution has success receipt")
			}
		})
	}
}
func TestAuthorFailuresAndSnapshot(t *testing.T) {
	for _, tc := range []struct {
		mode string
		code int
	}{{"fail75", 75}, {"bad-request", 1}, {"mutate-source", 1}, {"mutate-brief", 1}} {
		t.Run(tc.mode, func(t *testing.T) {
			f := setup(t, "anime", nil)
			f.env = append(f.env, "FAKE_AGENT_MODE="+tc.mode)
			f.run(t, "", tc.code)
			if _, e := os.Stat(f.log); !os.IsNotExist(e) {
				t.Fatal("backend ran after rejected author")
			}
		})
	}
	// Mutate to another valid PNG after snapshot, not just invalid bytes.
	f := setup(t, "anime", nil)
	accept(t, root, nil, helper, "snapshot", f.w, f.c)
	p := filepath.Join(f.w, "inputs/source.png")
	src := decode(t, p)
	m := image.NewNRGBA64(src.Bounds())
	var buf bytes.Buffer
	if e := png.Encode(&buf, m); e != nil {
		t.Fatal(e)
	}
	put(t, p, buf.Bytes())
	reject(t, root, nil, helper, "prepare", f.w, f.c)
}
func TestInvalidInputs(t *testing.T) {
	cases := []string{"missing", "invalid", "truncated", "bomb", "rect-outside", "rect-negative", "rect-fraction", "rect-missing-x", "rect-unlabeled", "full-frame", "full-union", "escape", "symlink", "ancestor-symlink", "unknown", "case-key", "duplicate", "null-regions", "oversize-style"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t, "anime", nil)
			p := filepath.Join(f.w, "inputs/source.png")
			bb := f.brief
			switch name {
			case "missing":
				if e := os.Remove(p); e != nil {
					t.Fatal(e)
				}
			case "invalid":
				put(t, p, []byte("not png"))
			case "truncated":
				b := get(t, p)
				put(t, p, b[:len(b)-6])
			case "bomb":
				b := get(t, p)
				binary.BigEndian.PutUint32(b[16:20], 0x7fffffff)
				binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
				put(t, p, b)
			case "rect-outside":
				bb["preserve_regions"] = []any{rect(31, 0, 2, 1, "bad")}
			case "rect-negative":
				bb["preserve_regions"] = []any{rect(-1, 0, 2, 1, "bad")}
			case "rect-fraction":
				r := rect(0, 0, 2, 1, "bad")
				r["x"] = 0.5
				bb["preserve_regions"] = []any{r}
			case "rect-missing-x":
				r := rect(0, 0, 2, 1, "bad")
				delete(r, "x")
				bb["preserve_regions"] = []any{r}
			case "rect-unlabeled":
				bb["preserve_regions"] = []any{rect(0, 0, 2, 1, " ")}
			case "full-frame":
				bb["preserve_regions"] = []any{rect(0, 0, 32, 24, "all")}
			case "full-union":
				bb["preserve_regions"] = []any{rect(0, 0, 16, 24, "left"), rect(16, 0, 16, 24, "right")}
			case "escape":
				bb["source"] = "inputs/../inputs/source.png"
			case "symlink":
				b := get(t, p)
				os.Remove(p)
				outside := filepath.Join(filepath.Dir(f.w), "outside.png")
				put(t, outside, b)
				if e := os.Symlink(outside, p); e != nil {
					t.Fatal(e)
				}
			case "ancestor-symlink":
				d := filepath.Join(f.w, "inputs")
				target := filepath.Join(filepath.Dir(f.w), "outside-inputs")
				if e := os.Rename(d, target); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(target, d); e != nil {
					t.Fatal(e)
				}
			case "unknown":
				bb["extra"] = true
			case "case-key":
				delete(bb, "version")
				bb["Version"] = 1
			case "null-regions":
				bb["preserve_regions"] = nil
			case "oversize-style":
				bb["style"] = strings.Repeat("a", 8001)
			}
			raw := j(bb)
			if name == "duplicate" {
				raw = append([]byte(`{"version":1,`), raw[1:]...)
			}
			put(t, filepath.Join(f.w, "brief.json"), raw)
			reject(t, root, nil, helper, "snapshot", f.w, f.c)
		})
	}
}
func TestRequestAndTampering(t *testing.T) {
	f := setup(t, "anime", []map[string]any{rect(2, 2, 12, 8, "42")})
	reject(t, f.w, f.env, filepath.Join(root, "expert/bin/check"))
	f.run(t, "", 0)
	msg := accept(t, f.w, f.env, filepath.Join(root, "expert/bin/check"))
	if !strings.Contains(string(msg), "request ready; image not generated") {
		t.Fatal(string(msg))
	}
	for _, p := range []string{"output/request.json", "output/image-notes.md"} {
		p = filepath.Join(f.w, p)
		old := get(t, p)
		put(t, p, []byte("{}"))
		reject(t, f.w, f.env, filepath.Join(root, "expert/bin/check"))
		put(t, p, old)
	}
	for _, rel := range []string{"request.json", "receipt.json", "generator-input.json", "snapshot.json", "generation/provenance.json", "generation/raw.png", "generation/record.jsonl", "generation/exit-status", "snapshot/source.png", "snapshot/brief.json"} {
		t.Run(rel, func(t *testing.T) {
			p := filepath.Join(f.c, rel)
			old := get(t, p)
			bad := append(append([]byte{}, old...), ' ')
			if rel == "receipt.json" {
				var r map[string]any
				if err := json.Unmarshal(old, &r); err != nil {
					t.Fatal(err)
				}
				r["version"] = 2
				bad = j(r)
			}
			if rel == "generation/exit-status" {
				bad = []byte("37\n")
			}
			put(t, p, bad)
			reject(t, root, f.env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
			put(t, p, old)
		})
	}
	// Tamper pixels AND the receipt hash: pixel recomputation must still reject.
	p := filepath.Join(f.c, "final.png")
	old := get(t, p)
	src := decode(t, p)
	m := image.NewNRGBA64(src.Bounds())
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			m.SetNRGBA64(x, y, straight(src.At(x, y)))
		}
	}
	m.SetNRGBA64(2, 3, color.NRGBA64{111, 222, 333, 65535})
	var buf bytes.Buffer
	png.Encode(&buf, m)
	put(t, p, buf.Bytes())
	rp := filepath.Join(f.c, "receipt.json")
	oldR := get(t, rp)
	var r map[string]any
	json.Unmarshal(oldR, &r)
	r["sha256"].(map[string]any)["final"] = hash(buf.Bytes())
	put(t, rp, j(r))
	reject(t, root, f.env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
	put(t, p, old)
	put(t, rp, oldR)
	accept(t, root, f.env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
}

// A real-sized file-spooled reference and 8-bit alpha, not only a tiny inline toy.
func TestLargeRasterAndAspectThreshold(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
		mode string
		code int
	}{
		{"large", 1024, 768, "resize", 0}, {"ratio-inclusive", 100, 100, "ratio-edge", 0}, {"ratio-over", 100, 100, "ratio-over", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t, "hand-painted texture", []map[string]any{rect(0, 0, 20, 16, "label and original alpha samples")})
			m := image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))
			for y := 0; y < tc.h; y++ {
				for x := 0; x < tc.w; x++ {
					m.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 201, 255})
				}
			}
			m.SetNRGBA(1, 1, color.NRGBA{153, 97, 221, 67})
			m.SetNRGBA(2, 1, color.NRGBA{13, 17, 19, 0})
			var buf bytes.Buffer
			if e := png.Encode(&buf, m); e != nil {
				t.Fatal(e)
			}
			put(t, filepath.Join(f.w, "inputs/source.png"), buf.Bytes())
			f.run(t, tc.mode, tc.code)
			if tc.code == 0 {
				out := decode(t, filepath.Join(f.c, "final.png"))
				if out.Bounds() != m.Bounds() {
					t.Fatal("normalized dimensions mismatch")
				}
				for y := 0; y < 16; y++ {
					for x := 0; x < 20; x++ {
						if straight(out.At(x, y)) != straight(m.At(x, y)) {
							t.Fatalf("source sample not retained at %d,%d", x, y)
						}
					}
				}
			}
		})
	}
}
func TestRecordIsIndependentlyVerified(t *testing.T) {
	f := setup(t, "cel planes", nil)
	f.run(t, "", 0)
	rp := filepath.Join(f.c, "receipt.json")
	originalR := get(t, rp)
	for _, tc := range []struct {
		path, key string
		bytes     []byte
	}{
		{"generation/provenance.json", "provenance", []byte(`{"backend":"fabricated-but-valid-object"}`)},
		{"generation/record.jsonl", "generator_record", []byte(`{"fabricated":"record"}`)},
	} {
		p := filepath.Join(f.c, tc.path)
		old := get(t, p)
		put(t, p, tc.bytes)
		var r map[string]any
		if e := json.Unmarshal(originalR, &r); e != nil {
			t.Fatal(e)
		}
		r["sha256"].(map[string]any)[tc.key] = hash(tc.bytes)
		put(t, rp, j(r))
		// Pixel/hash stage alone is insufficient; real public Record must reject.
		accept(t, root, f.env, helper, "check", f.w, f.c)
		reject(t, root, f.env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
		put(t, p, old)
		put(t, rp, originalR)
	}
}
func TestControlAndOutputPathBoundary(t *testing.T) {
	f := setup(t, "ink", nil)
	reject(t, root, nil, helper, "snapshot", f.w, filepath.Join(f.w, "bad-control"))
	f.run(t, "", 0)
	p := filepath.Join(f.c, "final.png")
	b := get(t, p)
	if e := os.Remove(p); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(f.c, "generation/raw.png"), p); e != nil {
		t.Fatal(e)
	}
	reject(t, root, f.env, filepath.Join(root, "expert/bin/check-output"), f.w, f.c)
	if e := os.Remove(p); e != nil {
		t.Fatal(e)
	}
	put(t, p, b)
	// No author-selected extra output or arbitrary references enter the backend.
	q := filepath.Join(f.w, "output/request.json")
	old := get(t, q)
	var r map[string]any
	json.Unmarshal(old, &r)
	r["references"] = []string{"/arbitrary/path.png"}
	put(t, q, j(r))
	reject(t, f.w, f.env, filepath.Join(root, "expert/bin/check"))
}
