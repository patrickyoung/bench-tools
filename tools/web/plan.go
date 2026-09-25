package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

var kinds = []string{"goto", "click", "type", "select", "wait", "read", "html", "shot", "submit"}

func stepKind(step map[string]any) string {
	for _, k := range kinds {
		if _, ok := step[k]; ok {
			return k
		}
	}
	return ""
}
func consequential(kind string, step map[string]any) bool {
	return kind == "click" || kind == "submit" || step["irreversible"] == true
}
func gateWords(kind string, step map[string]any, origin string) string {
	if s, ok := step["may"].(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	if kind == "submit" {
		return fmt.Sprint(step[kind])
	}
	value := step[kind]
	if a, ok := value.([]any); ok && len(a) > 0 {
		value = a[0]
	}
	text := fmt.Sprintf("%s %v", kind, value)
	if origin != "" {
		text += " on " + origin
	}
	return text
}
func parsePlan(raw []byte) ([]map[string]any, error) {
	var root json.RawMessage = raw
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, bad("empty plan")
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		var object map[string]json.RawMessage
		if e := json.Unmarshal(raw, &object); e != nil {
			return nil, bad("invalid plan: %v", e)
		}
		var ok bool
		root, ok = object["steps"]
		if !ok {
			return nil, bad("plan object needs a steps array")
		}
	}
	var plan []map[string]any
	if e := json.Unmarshal(root, &plan); e != nil || string(root) == "null" {
		return nil, bad("plan must be an array of step objects")
	}
	for i, s := range plan {
		if s == nil {
			return nil, bad("step %d must be an object", i)
		}
	}
	return plan, nil
}
func runPlan(ctx context.Context, o options, in io.Reader, out, stderr io.Writer) (err error) {
	var raw []byte
	if len(o.args) == 0 || o.args[0] == "-" {
		raw, err = boundedRead(in)
	} else {
		raw, err = readFile(o.args[0])
	}
	if err != nil {
		return err
	}
	plan, e := parsePlan(raw)
	if e != nil {
		return e
	}
	for _, s := range plan {
		if v, ok := s["profile"]; ok {
			p, ok := v.(string)
			if !ok || p == "" {
				return bad("profile must name a file")
			}
			o.profile = p
		}
	}
	if e := o.validateIdentity(); e != nil {
		return e
	}
	start := time.Now()
	steps := []map[string]any{}
	code := 0
	origin := ""
	auditURL := ""
	if len(plan) > 0 {
		auditURL, _ = plan[0]["goto"].(string)
	}
	s, e := startSession(ctx, o, false, stderr)
	if e != nil {
		return e
	}
	completed := false
	defer func() { s.close(o.keep && completed, stderr) }()
	for i, step := range plan {
		kind := stepKind(step)
		r := map[string]any{"i": i, "ok": true}
		if kind == "" && len(step) > 0 {
			keys := []string{}
			for k := range step {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			comments := true
			for _, k := range keys {
				comments = comments && strings.HasPrefix(k, "//")
			}
			if comments {
				r["note"] = step[keys[0]]
				r["skipped"] = true
				steps = append(steps, r)
				continue
			}
		}
		if consequential(kind, step) {
			e = gate(ctx, gateWords(kind, step, origin), "run", origin, mode(o), o.job, stderr)
			if e != nil {
				var f *failure
				if errors.As(e, &f) {
					code = f.code
				}
				r["ok"] = false
				key := "error"
				if code == 75 {
					key = "parked"
				}
				if code == 3 {
					key = "declined"
				}
				r[key] = e.Error()
				steps = append(steps, r)
				break
			}
		}
		e = executeStep(ctx, s.page, kind, step, r)
		if e != nil {
			r["ok"] = false
			r["error"] = e.Error()
			code = 1
		}
		steps = append(steps, r)
		if e != nil {
			break
		}
		if kind == "goto" {
			u, _ := url.Parse(step[kind].(string))
			origin = u.Host
		}
	}
	completed = code == 0 && ctx.Err() == nil
	if !completed && code == 0 {
		code = 1
	}
	outcome := map[int]string{0: "ok", 1: "fail", 3: "declined", 75: "parked", 77: "no_approver"}[code]
	audit("run", auditURL, len(raw), start, outcome, mode(o), "", "")
	if e = writeJSON(out, map[string]any{"ok": completed, "ms": time.Since(start).Milliseconds(), "steps": steps}); e != nil {
		completed = false
		return e
	}
	if code != 0 {
		return &failure{code, "run " + outcome}
	}
	return nil
}
func stepTimeout(step map[string]any, kind string) (time.Duration, error) {
	d := 10 * time.Second
	if kind == "goto" {
		d = 30 * time.Second
	}
	if v, ok := step["timeout"]; ok {
		n, ok := v.(float64)
		if !ok || n <= 0 || n > 86400000 || math.Trunc(n) != n {
			return 0, bad("timeout must be positive integer milliseconds, at most 86400000")
		}
		d = time.Duration(n) * time.Millisecond
	}
	return d, nil
}
func executeStep(ctx context.Context, page *rod.Page, kind string, step, r map[string]any) error {
	timeout, e := stepTimeout(step, kind)
	if e != nil {
		return e
	}
	if kind == "wait" {
		if n, ok := step[kind].(float64); ok {
			if n < 0 || n > 86400000 {
				return bad("wait must be 0..86400000 milliseconds")
			}
			timer := time.NewTimer(time.Duration(n * float64(time.Millisecond)))
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				r[kind] = n
				return nil
			}
		}
	}
	run, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	p := page.Context(run)
	if kind == "" {
		return bad("unknown step")
	}
	value, ok := step[kind].(string)
	if kind == "type" || kind == "select" {
		pair, ok := step[kind].([]any)
		if !ok || len(pair) != 2 {
			return bad("%s needs [CSS selector, value]", kind)
		}
		selector, ok := pair[0].(string)
		if !ok {
			return bad("selector must be a string")
		}
		el, e := p.Element(selector)
		if e != nil {
			return e
		}
		r[kind] = selector
		if kind == "type" {
			text, ok := pair[1].(string)
			if !ok {
				return bad("type value must be a string")
			}
			password, e := el.Eval(`() => this.matches('input[type="password"]')`)
			if e != nil {
				return e
			}
			if password.Value.Bool() {
				return fmt.Errorf("web never types into password fields; use manual web auth")
			}
			if e = el.SelectAllText(); e != nil {
				return e
			}
			return el.Input(text)
		}
		// Select options by value (including multiple selections) or an explicit
		// Playwright-compatible {value,label,index} descriptor. JS arguments are data.
		_, e = el.Eval(`(want) => {if(this.tagName !== 'SELECT') throw new Error('select needs a select element'); const values=Array.isArray(want)?want:[want];const matches=[...this.options].filter((o,i)=>values.some(v=>typeof v==='string'?o.value===v:typeof v==='object'&&v!==null&&(v.value!==undefined?o.value===v.value:v.label!==undefined?o.label===v.label:i===v.index)));if(matches.length<values.length)throw new Error('option not found');for(const o of this.options)o.selected=matches.includes(o);this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}));}`, pair[1])
		return e
	}
	if !ok {
		return bad("%s needs a string%s", kind, map[bool]string{true: " or number of milliseconds"}[kind == "wait"])
	}
	r[kind] = value
	switch kind {
	case "goto":
		wait := "domcontentloaded"
		if v, ok := step["wait"]; ok {
			wait, ok = v.(string)
			if !ok {
				return bad("goto wait must be a mode")
			}
		}
		if wait != "domcontentloaded" && wait != "load" && wait != "networkidle" {
			return bad("unknown wait mode %q", wait)
		}
		return navigate(p, value, wait)
	case "shot":
		return screenshot(p, value)
	case "submit":
		if e := input.Enter.Encode(proto.InputDispatchKeyEventTypeKeyDown, 0).Call(p); e != nil {
			return e
		}
		return input.Enter.Encode(proto.InputDispatchKeyEventTypeKeyUp, 0).Call(p)
	case "click", "wait", "read", "html":
		el, e := p.Element(value)
		if e != nil {
			return e
		}
		switch kind {
		case "click":
			pt, e := el.WaitInteractable()
			if e != nil {
				return e
			}
			if e = el.WaitEnabled(); e != nil {
				return e
			}
			for _, kind := range []proto.InputDispatchMouseEventType{proto.InputDispatchMouseEventTypeMouseMoved, proto.InputDispatchMouseEventTypeMousePressed, proto.InputDispatchMouseEventTypeMouseReleased} {
				if e = (proto.InputDispatchMouseEvent{Type: kind, X: pt.X, Y: pt.Y, Button: proto.InputMouseButtonLeft, ClickCount: 1}).Call(p); e != nil {
					return e
				}
			}
			return nil
		case "wait":
			return el.WaitVisible()
		case "read":
			v, e := el.Eval(`() => this.innerText`)
			if e == nil {
				r["text"] = v.Value.Str()
			}
			return e
		case "html":
			v, e := el.Eval(`() => this.innerHTML`)
			if e != nil {
				return e
			}
			markup := v.Value.Str()
			if path, ok := step["out"].(string); ok && path != "" {
				if e = os.WriteFile(path, []byte(markup), 0600); e != nil {
					return e
				}
				r["out"] = path
				r["bytes"] = len(markup)
			} else {
				if utf8.RuneCountInString(markup) > 200000 {
					return fmt.Errorf("inline HTML exceeds 200000 characters; use out for the complete file")
				}
				r["markup"] = markup
			}
			return nil
		}
	}
	return bad("unknown step")
}
