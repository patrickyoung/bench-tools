// Bitmap's deterministic file boundary and compositor. No model or subprocess calls.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maxBytes = 32 << 20
const maxPixels = 8 << 20
const maxSide = 8192

type Region struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Label  string `json:"label"`
}

func (r *Region) UnmarshalJSON(b []byte) error {
	var v struct {
		X      *int    `json:"x"`
		Y      *int    `json:"y"`
		Width  *int    `json:"width"`
		Height *int    `json:"height"`
		Label  *string `json:"label"`
	}
	if err := strict(b, &v); err != nil {
		return err
	}
	if v.X == nil || v.Y == nil || v.Width == nil || v.Height == nil || v.Label == nil {
		return errors.New("rectangle requires x,y,width,height,label")
	}
	*r = Region{*v.X, *v.Y, *v.Width, *v.Height, *v.Label}
	return nil
}

type Brief struct {
	Version int      `json:"version"`
	Source  string   `json:"source"`
	Style   string   `json:"style"`
	Regions []Region `json:"preserve_regions,omitempty"`
}
type Request struct {
	Prompt     string   `json:"prompt"`
	References []string `json:"references"`
	BriefHash  string   `json:"brief_sha256"`
	SourceHash string   `json:"source_sha256"`
}
type Snapshot struct {
	Version    int    `json:"version"`
	Work       string `json:"work"`
	Control    string `json:"control"`
	BriefHash  string `json:"brief_sha256"`
	SourceHash string `json:"source_sha256"`
}
type GeneratorInput struct {
	Prompt     string   `json:"prompt"`
	Output     string   `json:"output"`
	References []string `json:"references"`
}
type Receipt struct {
	Version               int               `json:"version"`
	Kind                  string            `json:"kind"`
	Hashes                map[string]string `json:"sha256"`
	SourceSize            [2]int            `json:"source_dimensions"`
	RawSize               [2]int            `json:"raw_dimensions"`
	Normalization         string            `json:"normalization"`
	Regions               []Region          `json:"preserve_regions"`
	EditablePixelsChanged int               `json:"editable_pixels_changed"`
	VisualReview          string            `json:"visual_review"`
}

// Reject duplicate keys, trailing values, unknown fields, invalid UTF-8 and null roots.
func strict(b []byte, v any) error {
	if !utf8.Valid(b) {
		return errors.New("invalid UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return errors.New("duplicate/invalid JSON key")
					}
					seen[s] = true
					if e = walk(); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e := walk(); e != nil {
						return e
					}
				}
			default:
				return errors.New("bad JSON delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if e := walk(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errors.New("null JSON")
	}
	rv := reflect.TypeOf(v)
	if rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct {
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(b, &fields); e != nil {
			return e
		}
		allowed := map[string]bool{}
		for i := 0; i < rv.Elem().NumField(); i++ {
			allowed[strings.Split(rv.Elem().Field(i).Tag.Get("json"), ",")[0]] = true
		}
		for k := range fields {
			if !allowed[k] {
				return fmt.Errorf("unknown JSON field: %s", k)
			}
		}
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func noLinks(p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return fmt.Errorf("require clean absolute path: %s", p)
	}
	for q := p; ; q = filepath.Dir(q) {
		i, e := os.Lstat(q)
		if e != nil {
			return e
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink forbidden: %s", q)
		}
		if q == filepath.Dir(q) {
			break
		}
	}
	return nil
}
func read(p string, limit int64) ([]byte, error) {
	if e := noLinks(p); e != nil {
		return nil, e
	}
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("not a bounded regular file: %s", p)
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > limit {
		return nil, fmt.Errorf("not a bounded regular file: %s", p)
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e == nil && int64(len(b)) > limit {
		e = errors.New("file too large")
	}
	return b, e
}
func write(p string, b []byte) error {
	if e := noLinks(filepath.Dir(p)); e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func jsonBytes(v any) []byte {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	return append(b, '\n')
}
func writeJSON(p string, v any) error { return write(p, jsonBytes(v)) }
func loadJSON(p string, v any) error {
	b, e := read(p, 128<<10)
	if e != nil {
		return e
	}
	return strict(b, v)
}
func within(child, parent string) bool {
	if parent == string(os.PathSeparator) {
		return filepath.IsAbs(child)
	}
	return child == parent || strings.HasPrefix(child, parent+string(os.PathSeparator))
}
func dimensions(w, h int) bool {
	return w > 0 && h > 0 && w <= maxSide && h <= maxSide && int64(w)*int64(h) <= maxPixels
}
func decode(b []byte) (*image.NRGBA64, error) {
	if len(b) > maxBytes || len(b) < 8 || !bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return nil, errors.New("missing/invalid PNG")
	}
	// A complete, bounded chunk stream: no truncated IEND or trailing payload.
	ended := false
	for pos := 8; pos < len(b); {
		if len(b)-pos < 12 {
			return nil, errors.New("truncated PNG chunk")
		}
		n := int64(binary.BigEndian.Uint32(b[pos : pos+4]))
		if n > int64(len(b)-pos-12) {
			return nil, errors.New("truncated PNG payload")
		}
		end := pos + 8 + int(n)
		if crc32.ChecksumIEEE(b[pos+4:end]) != binary.BigEndian.Uint32(b[end:end+4]) {
			return nil, errors.New("PNG CRC mismatch")
		}
		if string(b[pos+4:pos+8]) == "IEND" {
			if n != 0 || end+4 != len(b) {
				return nil, errors.New("invalid PNG end")
			}
			ended = true
		}
		pos = end + 4
	}
	if !ended {
		return nil, errors.New("missing PNG IEND")
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	if !dimensions(cfg.Width, cfg.Height) {
		return nil, errors.New("PNG dimensions exceed 8192/8M pixel bound")
	}
	m, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	out := image.NewNRGBA64(image.Rect(0, 0, cfg.Width, cfg.Height))
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			out.SetNRGBA64(x, y, straight(m.At(x, y)))
		}
	}
	return out, nil
}
func straight(c color.Color) color.NRGBA64 {
	switch v := c.(type) {
	case color.NRGBA64:
		return v
	case color.NRGBA:
		return color.NRGBA64{uint16(v.R) * 257, uint16(v.G) * 257, uint16(v.B) * 257, uint16(v.A) * 257}
	default:
		return color.NRGBA64Model.Convert(c).(color.NRGBA64)
	}
}
func source(work string) (Brief, []byte, []byte, *image.NRGBA64, error) {
	var b Brief
	fail := func(e error) (Brief, []byte, []byte, *image.NRGBA64, error) { return b, nil, nil, nil, e }
	bb, e := read(filepath.Join(work, "brief.json"), 64<<10)
	if e != nil {
		return fail(e)
	}
	if e = strict(bb, &b); e != nil {
		return fail(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(bb, &fields); e != nil {
		return fail(e)
	}
	if regions, ok := fields["preserve_regions"]; ok && bytes.Equal(bytes.TrimSpace(regions), []byte("null")) {
		return fail(errors.New("preserve_regions must be an array"))
	}
	if b.Version != 1 || len(b.Style) > 8000 || strings.TrimSpace(b.Style) == "" {
		return fail(errors.New("version must be 1; style must be 1..8000 bytes"))
	}
	if len(b.Source) > 1024 || !strings.HasPrefix(b.Source, "inputs/") || filepath.Clean(b.Source) != b.Source || strings.ContainsAny(b.Source, "\\\x00") || b.Source == "inputs/" {
		return fail(errors.New("source must be a clean path under inputs/"))
	}
	sb, e := read(filepath.Join(work, b.Source), maxBytes)
	if e != nil {
		return fail(e)
	}
	im, e := decode(sb)
	if e != nil {
		return fail(e)
	}
	if len(b.Regions) > 256 {
		return fail(errors.New("at most 256 protected rectangles"))
	}
	mask := make([]bool, im.Bounds().Dx()*im.Bounds().Dy())
	covered := 0
	for _, r := range b.Regions {
		if r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0 || r.X > im.Bounds().Dx() || r.Y > im.Bounds().Dy() || r.Width > im.Bounds().Dx()-r.X || r.Height > im.Bounds().Dy()-r.Y || strings.TrimSpace(r.Label) == "" || len(r.Label) > 256 {
			return fail(errors.New("invalid labeled protected rectangle"))
		}
		for y := r.Y; y < r.Y+r.Height; y++ {
			for x := r.X; x < r.X+r.Width; x++ {
				i := y*im.Bounds().Dx() + x
				if !mask[i] {
					mask[i] = true
					covered++
				}
			}
		}
	}
	if covered == len(mask) {
		return fail(errors.New("no stylizable pixels: protected union covers frame"))
	}
	return b, bb, sb, im, nil
}
func request(work string, b Brief, bb, sb []byte) (Request, []byte, []byte, error) {
	var r Request
	rb, e := read(filepath.Join(work, "output/request.json"), 64<<10)
	if e != nil {
		return r, nil, nil, e
	}
	if e = strict(rb, &r); e != nil {
		return r, nil, nil, e
	}
	if strings.TrimSpace(r.Prompt) == "" || len(r.Prompt) > 32000 || r.BriefHash != hash(bb) || r.SourceHash != hash(sb) || len(r.References) != 1 || r.References[0] != b.Source {
		return r, nil, nil, errors.New("request prompt, source reference or SHA-256 bindings invalid")
	}
	notes, e := read(filepath.Join(work, "output/image-notes.md"), 64<<10)
	if e != nil {
		return r, nil, nil, e
	}
	if !utf8.Valid(notes) || len(bytes.TrimSpace(notes)) < 40 {
		return r, nil, nil, errors.New("composition notes missing/too short")
	}
	return r, rb, notes, nil
}
func snapshot(work, control string) error {
	if e := noLinks(work); e != nil {
		return e
	}
	if !filepath.IsAbs(control) || filepath.Clean(control) != control || within(control, work) || within(work, control) {
		return errors.New("control must be clean absolute and disjoint from work")
	}
	if e := noLinks(filepath.Dir(control)); e != nil {
		return e
	}
	_, bb, sb, _, e := source(work)
	if e != nil {
		return e
	}
	if e = os.Mkdir(control, 0700); e != nil {
		return e
	}
	for _, dir := range []string{"snapshot", "generation"} {
		if e = os.Mkdir(filepath.Join(control, dir), 0700); e != nil {
			return e
		}
	}
	if e = write(filepath.Join(control, "snapshot/brief.json"), bb); e != nil {
		return e
	}
	if e = write(filepath.Join(control, "snapshot/source.png"), sb); e != nil {
		return e
	}
	return writeJSON(filepath.Join(control, "snapshot.json"), Snapshot{1, work, control, hash(bb), hash(sb)})
}
func bound(work, control string) (Brief, []byte, []byte, *image.NRGBA64, error) {
	var s Snapshot
	fail := func(e error) (Brief, []byte, []byte, *image.NRGBA64, error) { return Brief{}, nil, nil, nil, e }
	if e := loadJSON(filepath.Join(control, "snapshot.json"), &s); e != nil {
		return fail(e)
	}
	if s.Version != 1 || s.Work != work || s.Control != control || within(control, work) || within(work, control) {
		return fail(errors.New("snapshot workspace binding mismatch"))
	}
	b, bb, sb, im, e := source(work)
	if e != nil {
		return fail(e)
	}
	oldB, e := read(filepath.Join(control, "snapshot/brief.json"), 64<<10)
	if e != nil {
		return fail(e)
	}
	oldS, e := read(filepath.Join(control, "snapshot/source.png"), maxBytes)
	if e != nil {
		return fail(e)
	}
	if !bytes.Equal(bb, oldB) || !bytes.Equal(sb, oldS) || s.BriefHash != hash(bb) || s.SourceHash != hash(sb) {
		return fail(errors.New("admitted brief/source changed after snapshot"))
	}
	return b, bb, sb, im, nil
}
func genInput(r Request, control string) GeneratorInput {
	return GeneratorInput{r.Prompt, filepath.Join(control, "generation/raw.png"), []string{filepath.Join(control, "snapshot/source.png")}}
}
func prepare(work, control string) error {
	b, bb, sb, _, e := bound(work, control)
	if e != nil {
		return e
	}
	r, rb, notes, e := request(work, b, bb, sb)
	if e != nil {
		return e
	}
	if e = write(filepath.Join(control, "request.json"), rb); e != nil {
		return e
	}
	if e = write(filepath.Join(control, "image-notes.md"), notes); e != nil {
		return e
	}
	input := jsonBytes(genInput(r, control))
	if len(input) > 40000 {
		return errors.New("encoded generator input exceeds 40000 bytes")
	}
	return write(filepath.Join(control, "generator-input.json"), input)
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

type tap struct {
	i int
	w float64
}

func weights(old, n int) [][]tap {
	out := make([][]tap, n)
	scale := math.Max(1, float64(old)/float64(n))
	for i := 0; i < n; i++ {
		c := (float64(i)+.5)*float64(old)/float64(n) - .5
		sum := 0.
		for k := int(math.Ceil(c - 3*scale)); k <= int(math.Floor(c+3*scale)); k++ {
			x := (float64(k) - c) / scale
			if math.Abs(x) >= 3 {
				continue
			}
			w := sinc(x) * sinc(x/3)
			j := max(0, min(old-1, k))
			out[i] = append(out[i], tap{j, w})
			sum += w
		}
		for j := range out[i] {
			out[i][j].w /= sum
		}
	}
	return out
}
func channel(x float64) uint16 { return uint16(math.Round(math.Max(0, math.Min(65535, x)))) }
func axis(src *image.NRGBA64, n int, horizontal bool) *image.NRGBA64 {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	old := h
	if horizontal {
		old = w
		w = n
	} else {
		h = n
	}
	ws := weights(old, n)
	out := image.NewNRGBA64(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			j := y
			if horizontal {
				j = x
			}
			var rr, gg, bb, aa float64
			for _, t := range ws[j] {
				sx, sy := x, t.i
				if horizontal {
					sx, sy = t.i, y
				}
				c := src.NRGBA64At(sx, sy)
				a := float64(c.A) / 65535
				rr += t.w * float64(c.R) * a
				gg += t.w * float64(c.G) * a
				bb += t.w * float64(c.B) * a
				aa += t.w * float64(c.A)
			}
			c := color.NRGBA64{A: channel(aa)}
			if aa > 0 {
				c.R = channel(rr * 65535 / aa)
				c.G = channel(gg * 65535 / aa)
				c.B = channel(bb * 65535 / aa)
			}
			out.SetNRGBA64(x, y, c)
		}
	}
	return out
}
func compose(src, raw *image.NRGBA64, regions []Region) (*image.NRGBA64, string, int, error) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	rw, rh := raw.Bounds().Dx(), raw.Bounds().Dy()
	// Relative aspect-ratio tolerance of 1%, inclusive.
	a, b := int64(rw)*int64(sh), int64(sw)*int64(rh)
	delta := a - b
	if delta < 0 {
		delta = -delta
	}
	if delta*100 > b {
		return nil, "", 0, errors.New("generated aspect ratio differs by more than 1%")
	}
	mode := "identity"
	out := raw
	if sw != rw || sh != rh {
		mode = "lanczos3-premultiplied-separable"
		if sw*rh <= rw*sh {
			if sw != rw {
				out = axis(out, sw, true)
			}
			if sh != rh {
				out = axis(out, sh, false)
			}
		} else {
			if sh != rh {
				out = axis(out, sh, false)
			}
			if sw != rw {
				out = axis(out, sw, true)
			}
		}
	} else {
		out = image.NewNRGBA64(src.Bounds())
		copy(out.Pix, raw.Pix)
	}
	for _, r := range regions {
		rect := image.Rect(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			start := src.PixOffset(rect.Min.X, y)
			end := start + rect.Dx()*8
			copy(out.Pix[out.PixOffset(rect.Min.X, y):], src.Pix[start:end])
		}
	}
	changed := 0
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			if out.NRGBA64At(x, y) != src.NRGBA64At(x, y) {
				changed++
			}
		}
	}
	if changed == 0 {
		return nil, "", 0, errors.New("unchanged final: no editable pixels changed")
	}
	return out, mode, changed, nil
}
func expected(work, control string) (*image.NRGBA64, Receipt, error) {
	var receipt Receipt
	fail := func(e error) (*image.NRGBA64, Receipt, error) { return nil, receipt, e }
	b, bb, sb, src, e := bound(work, control)
	if e != nil {
		return fail(e)
	}
	r, rb, notes, e := request(work, b, bb, sb)
	if e != nil {
		return fail(e)
	}
	kept, e := read(filepath.Join(control, "request.json"), 64<<10)
	if e != nil {
		return fail(e)
	}
	keptNotes, e := read(filepath.Join(control, "image-notes.md"), 64<<10)
	if e != nil {
		return fail(e)
	}
	if !bytes.Equal(rb, kept) || !bytes.Equal(notes, keptNotes) {
		return fail(errors.New("request/notes changed after preparation"))
	}
	gib, e := read(filepath.Join(control, "generator-input.json"), 64<<10)
	if e != nil {
		return fail(e)
	}
	var gi GeneratorInput
	if e = strict(gib, &gi); e != nil {
		return fail(e)
	}
	if !reflect.DeepEqual(gi, genInput(r, control)) {
		return fail(errors.New("generator input binding mismatch"))
	}
	rawBytes, e := read(filepath.Join(control, "generation/raw.png"), maxBytes)
	if e != nil {
		return fail(e)
	}
	raw, e := decode(rawBytes)
	if e != nil {
		return fail(e)
	}
	prov, e := read(filepath.Join(control, "generation/provenance.json"), 1<<20)
	if e != nil {
		return fail(e)
	}
	var obj map[string]json.RawMessage
	if e = strict(prov, &obj); e != nil || len(obj) == 0 {
		return fail(errors.New("generator provenance must be a nonempty JSON object"))
	}
	exit, e := read(filepath.Join(control, "generation/exit-status"), 32)
	if e != nil {
		return fail(e)
	}
	if string(exit) != "0\n" {
		return fail(errors.New("generator did not finish successfully"))
	}
	rec, e := read(filepath.Join(control, "generation/record.jsonl"), 128<<20)
	if e != nil {
		return fail(e)
	}
	snap, e := read(filepath.Join(control, "snapshot.json"), 128<<10)
	if e != nil {
		return fail(e)
	}
	out, mode, changed, e := compose(src, raw, b.Regions)
	if e != nil {
		return fail(e)
	}
	kind := "hybrid-generative-bitmap-with-original-raster-regions"
	if len(b.Regions) == 0 {
		kind = "generative-bitmap-no-protected-regions"
	}
	receipt = Receipt{Version: 1, Kind: kind,
		Hashes:     map[string]string{"brief": hash(bb), "source": hash(sb), "request": hash(rb), "notes": hash(notes), "generator_input": hash(gib), "raw": hash(rawBytes), "provenance": hash(prov), "generator_record": hash(rec), "generator_exit": hash(exit), "snapshot": hash(snap)},
		SourceSize: [2]int{src.Bounds().Dx(), src.Bounds().Dy()}, RawSize: [2]int{raw.Bounds().Dx(), raw.Bounds().Dy()},
		Normalization: mode, Regions: b.Regions, EditablePixelsChanged: changed, VisualReview: "not performed by deterministic checker"}
	return out, receipt, nil
}
func finish(work, control string) error {
	out, r, e := expected(work, control)
	if e != nil {
		return e
	}
	var buf bytes.Buffer
	if e = png.Encode(&buf, out); e != nil {
		return e
	}
	if e = write(filepath.Join(control, "final.png"), buf.Bytes()); e != nil {
		return e
	}
	r.Hashes["final"] = hash(buf.Bytes())
	return writeJSON(filepath.Join(control, "receipt.json"), r)
}
func finalCheck(work, control string) error {
	want, r, e := expected(work, control)
	if e != nil {
		return e
	}
	fb, e := read(filepath.Join(control, "final.png"), maxBytes)
	if e != nil {
		return e
	}
	got, e := decode(fb)
	if e != nil {
		return e
	}
	if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
		return errors.New("final pixels differ from independently recomputed normalized composition")
	}
	r.Hashes["final"] = hash(fb)
	var actual Receipt
	if e = loadJSON(filepath.Join(control, "receipt.json"), &actual); e != nil {
		return e
	}
	if !reflect.DeepEqual(r, actual) {
		return errors.New("receipt does not bind current inputs/artifacts")
	}
	return nil
}
func guard(work string, paths []string) error {
	for _, p := range paths {
		if e := noLinks(p); e != nil {
			return e
		}
		i, e := os.Stat(p)
		if e != nil {
			return e
		}
		if within(p, work) || within(work, p) {
			return errors.New("definition/tools must be outside writable workspace")
		}
		if !i.IsDir() && (!i.Mode().IsRegular() || i.Mode()&0111 == 0) {
			return errors.New("selected tool must be a regular executable")
		}
	}
	return nil
}
func run(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: bitmap request WORK | snapshot/prepare/finish/check/artifact WORK CONTROL | guard WORK PATH...")
	}
	cmd, work := args[0], args[1]
	if e := noLinks(work); e != nil {
		return e
	}
	if cmd == "request" {
		b, bb, sb, _, e := source(work)
		if e != nil {
			return e
		}
		if _, _, _, e = request(work, b, bb, sb); e != nil {
			return e
		}
		fmt.Println("request ready; image not generated")
		return nil
	}
	if cmd == "guard" {
		return guard(work, args[2:])
	}
	if len(args) != 3 {
		return errors.New("expected WORK CONTROL")
	}
	control := args[2]
	switch cmd {
	case "snapshot":
		return snapshot(work, control)
	case "prepare":
		return prepare(work, control)
	case "finish":
		return finish(work, control)
	case "check":
		return finalCheck(work, control)
	case "artifact":
		if e := finalCheck(work, control); e != nil {
			return e
		}
		var r Receipt
		if e := loadJSON(filepath.Join(control, "receipt.json"), &r); e != nil {
			return e
		}
		_, e := os.Stdout.Write(jsonBytes(map[string]any{"version": 1, "kind": r.Kind, "final": filepath.Join(control, "final.png"), "receipt": filepath.Join(control, "receipt.json"), "source": filepath.Join(control, "snapshot/source.png"), "raw": filepath.Join(control, "generation/raw.png"), "request": filepath.Join(control, "request.json"), "notes": filepath.Join(control, "image-notes.md"), "provenance": filepath.Join(control, "generation/provenance.json"), "visual_review": "required; not established by pixel checks"}))
		return e
	default:
		return errors.New("unknown command")
	}
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "bitmap:", e)
		os.Exit(1)
	}
}
