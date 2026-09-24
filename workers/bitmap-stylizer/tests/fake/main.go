// Offline fixtures only. This executable is never part of the reusable expert.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(91)
	}
}
func read(p string) []byte    { b, e := os.ReadFile(p); must(e); return b }
func save(p string, b []byte) { must(os.WriteFile(p, b, 0600)) }
func hash(b []byte) string    { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func arg(k string) string {
	for i, a := range os.Args {
		if a == k && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}
func author() {
	if arg("-m") != "offline-fixture" {
		panic("offline fixture requires explicit offline-fixture model")
	}
	for _, a := range os.Args {
		if a == "-no-cage" || a == "-net" {
			panic("unexpected broadened boundary")
		}
	}
	work := arg("-C")
	must(os.Chdir(work))
	if os.Getenv("FAKE_AGENT_MODE") == "fail75" {
		os.Exit(75)
	}
	bb := read("brief.json")
	var b struct {
		Source, Style string
		Regions       []map[string]any `json:"preserve_regions"`
	}
	must(json.Unmarshal(bb, &b))
	sb := read(b.Source)
	r := map[string]any{"prompt": "Preserve layout, facts, background palette and margins. Restyle artwork: " + b.Style + ". Do not add text, signatures, logos or pseudo-data. Keep protected negative space for raster restoration.",
		"references": []string{b.Source}, "brief_sha256": hash(bb), "source_sha256": hash(sb)}
	if os.Getenv("FAKE_AGENT_MODE") == "bad-request" {
		r["output"] = "/escape.png"
	}
	must(os.MkdirAll("output", 0700))
	rb, e := json.Marshal(r)
	must(e)
	save("output/request.json", rb)
	save("output/image-notes.md", []byte("Offline authored fixture, not model quality evidence. Keep factual label region and palette; seam and style review remain pending."))
	if os.Getenv("FAKE_AGENT_MODE") == "mutate-source" {
		save(b.Source, append(sb, 'x'))
	}
	if os.Getenv("FAKE_AGENT_MODE") == "mutate-brief" {
		save("brief.json", append(bb, ' '))
	}
	var def string
	for i, a := range os.Args {
		if a == "--" {
			def = os.Args[i-1]
			break
		}
	}
	c := exec.Command(filepath.Join(def, "bin/check"))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if e := c.Run(); e != nil {
		if ex, ok := e.(*exec.ExitError); ok {
			os.Exit(ex.ExitCode())
		}
		must(e)
	}
}
func backend() {
	mode := os.Getenv("FAKE_BACKEND_MODE")
	b, e := io.ReadAll(os.Stdin)
	must(e)
	var v struct {
		Prompt, Output string
		References     []string
	}
	must(json.Unmarshal(b, &v))
	if !filepath.IsAbs(v.Output) || len(v.References) != 1 || !filepath.IsAbs(v.References[0]) {
		panic("bad generator contract")
	}
	if p := os.Getenv("FAKE_CALL_LOG"); p != "" {
		f, e := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		must(e)
		_, e = f.WriteString("call\n")
		must(e)
		must(f.Close())
	}
	switch mode {
	case "fail37":
		fmt.Fprintln(os.Stderr, "fixture backend exact failure")
		os.Exit(37)
	case "fail75":
		os.Exit(75)
	case "fail125":
		os.Exit(125)
	}
	sb := read(v.References[0])
	m, e := png.Decode(bytes.NewReader(sb))
	must(e)
	w, h := m.Bounds().Dx(), m.Bounds().Dy()
	var output []byte
	switch mode {
	case "missing":
	case "same":
		output = sb
	case "label-only":
		n := image.NewNRGBA64(m.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				n.Set(x, y, m.At(x, y))
			}
		}
		for y := 2; y < 10; y++ {
			for x := 2; x < 14; x++ {
				n.SetNRGBA64(x, y, color.NRGBA64{123, 456, 789, 65535})
			}
		}
		var buf bytes.Buffer
		must(png.Encode(&buf, n))
		output = buf.Bytes()
	case "symlink":
		must(os.Symlink(v.References[0], v.Output))
	default:
		if mode == "resize" {
			w *= 2
			h *= 2
		}
		if mode == "ratio-edge" {
			w += w / 100
		}
		if mode == "ratio-over" {
			w += w/100 + 1
		}
		if mode == "aspect" {
			w *= 2
		}
		n := image.NewNRGBA64(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				n.SetNRGBA64(x, y, color.NRGBA64{uint16(12000 + x*20), uint16(43000 + y*50), 19000, 65535})
			}
		}
		var buf bytes.Buffer
		must(png.Encode(&buf, n))
		output = buf.Bytes()
		if mode == "truncated" {
			output = output[:len(output)-9]
		}
		if mode == "bomb" {
			binary.BigEndian.PutUint32(output[16:20], 0x7fffffff)
			binary.BigEndian.PutUint32(output[29:33], crc32.ChecksumIEEE(output[12:29]))
		}
	}
	if output != nil {
		save(v.Output, output)
	}
	if mode == "malformed-provenance" {
		fmt.Println("[]")
		return
	}
	if mode == "empty-provenance" {
		fmt.Println("{}")
		return
	}
	fmt.Fprintln(os.Stderr, "offline fixture; no actual image capability used")
	fmt.Printf("{\"backend\":\"offline-fixture\",\"mode\":%q}\n", mode)
}
func main() {
	if strings.HasSuffix(filepath.Base(os.Args[0]), "agent") {
		author()
	} else {
		backend()
	}
}
