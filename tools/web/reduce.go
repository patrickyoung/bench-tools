package main

import (
	"embed"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

//go:embed fixtures/*
var fixtures embed.FS

type link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type block struct{ kind, text string }

var spaces = regexp.MustCompile(`[ \t\r\f\v]+`)
var indent = regexp.MustCompile(`\n +`)
var blankLines = regexp.MustCompile(`\n{3,}`)
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)

func hasTag(list, tag string) bool { return strings.Contains(" "+list+" ", " "+tag+" ") }
func reduceHTML(src, base string) (string, []link) {
	z := html.NewTokenizer(strings.NewReader(src))
	var title, buf, label strings.Builder
	var blocks []block
	links := []link{}
	kind, href := "p", ""
	drop, inTitle := 0, false
	flush := func() {
		s := strings.TrimSpace(indent.ReplaceAllString(spaces.ReplaceAllString(buf.String(), " "), "\n"))
		if s != "" {
			blocks = append(blocks, block{kind, s})
		}
		buf.Reset()
		kind = "p"
	}
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		tok := z.Token()
		tag := tok.Data
		isDrop := hasTag("script style noscript svg nav aside footer", tag)
		isBlock := hasTag("p div section article li tr br hr h1 h2 h3 h4 h5 h6 blockquote pre", tag)
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			if isDrop {
				if tt != html.SelfClosingTagToken {
					drop++
				}
				continue
			}
			if drop > 0 {
				continue
			}
			attrs := map[string]string{}
			for _, a := range tok.Attr {
				attrs[a.Key] = a.Val
			}
			if tag == "title" {
				inTitle = true
				continue
			}
			if isBlock {
				flush()
				kind = tag
			}
			if tag == "a" {
				href = ""
				if u, e := url.Parse(attrs["href"]); e == nil && attrs["href"] != "" {
					if b, e := url.Parse(base); e == nil {
						href = b.ResolveReference(u).String()
					}
				}
				label.Reset()
				buf.WriteByte('[')
			}
			if tag == "br" {
				buf.WriteByte('\n')
			}
			if tag == "img" {
				if alt := strings.TrimSpace(attrs["alt"]); alt != "" {
					fmt.Fprintf(&buf, "(%s)", alt)
				}
			}
		case html.EndTagToken:
			if isDrop {
				if drop > 0 {
					drop--
				}
				continue
			}
			if drop > 0 {
				continue
			}
			if tag == "title" {
				inTitle = false
				continue
			}
			if tag == "a" {
				fmt.Fprintf(&buf, "](%s)", href)
				if href != "" {
					v := strings.TrimSpace(label.String())
					if v == "" {
						v = href
					}
					links = append(links, link{v, href})
				}
				href = ""
				label.Reset()
				continue
			}
			if isBlock {
				flush()
			}
		case html.TextToken:
			if drop > 0 {
				continue
			}
			if inTitle {
				title.WriteString(tok.Data)
				continue
			}
			buf.WriteString(tok.Data)
			if href != "" {
				label.WriteString(tok.Data)
			}
		}
	}
	flush()
	var out strings.Builder
	prevLi := false
	for i, b := range blocks {
		s := b.text
		switch {
		case len(b.kind) == 2 && b.kind[0] == 'h' && b.kind[1] >= '1' && b.kind[1] <= '6':
			s = strings.Repeat("#", int(b.kind[1]-'0')) + " " + s
		case b.kind == "li":
			s = "- " + strings.ReplaceAll(s, "\n", "\n- ")
		case b.kind == "blockquote":
			s = "> " + strings.ReplaceAll(s, "\n", "\n> ")
		case b.kind == "pre":
			s = "```\n" + s + "\n```"
		}
		if i > 0 {
			if prevLi && b.kind == "li" {
				out.WriteByte('\n')
			} else {
				out.WriteString("\n\n")
			}
		}
		out.WriteString(s)
		prevLi = b.kind == "li"
	}
	md := strings.TrimSpace(blankLines.ReplaceAllString(out.String(), "\n\n"))
	t := strings.TrimSpace(title.String())
	if t != "" && !strings.HasPrefix(md, "# ") {
		md = "# " + t + "\n\n" + md
	}
	return strings.TrimRight(md, "\n") + "\n", links
}
func plainText(md string) string {
	return markdownLink.ReplaceAllStringFunc(md, func(s string) string {
		m := markdownLink.FindStringSubmatch(s)
		if m[1] != "" {
			return m[1]
		}
		return m[2]
	})
}
func linkText(links []link) string {
	seen := map[string]bool{}
	var s strings.Builder
	for _, l := range links {
		if !seen[l.URL] {
			fmt.Fprintf(&s, "%s\t%s\n", l.Label, l.URL)
			seen[l.URL] = true
		}
	}
	return s.String()
}
func selfCheck(out io.Writer) error {
	entries, e := fixtures.ReadDir("fixtures")
	if e != nil {
		return e
	}
	n := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		base := strings.TrimSuffix(name, ".html")
		src, _ := fixtures.ReadFile("fixtures/" + name)
		md, links := reduceHTML(string(src), "https://example.test/")
		for ext, got := range map[string]string{"md": md, "links": linkText(links)} {
			want, e := fixtures.ReadFile("fixtures/" + base + "." + ext)
			if e != nil {
				return e
			}
			if got != string(want) {
				return fmt.Errorf("%s.%s reduction differs", base, ext)
			}
		}
		n++
	}
	fmt.Fprintf(out, "web check: %d reduction fixtures passed (offline)\n", n)
	return nil
}
