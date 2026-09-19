// Weigh is a one-shot typed judgment filter. Policy belongs to its caller.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const version = "0.1.0"

var errTooLarge = errors.New("size limit")

const help = `weigh - one typed judgment, no action or policy

usage: weigh -m openrouter/MODEL [options] < request.json > result.json
       weigh help
       weigh version

options:
  -m MODEL       required explicit model; preserves the full routed name
  -timeout TIME  positive deadline for reading and inference (default 30s)
  -endpoint URL  full Decisions URL (HTTPS; HTTP only literal loopback IPs)
  -header-fd N   read a private Authorization header from descriptor N >= 3

default endpoint: https://openrouter.ai/api/alpha/decisions
environment: OPENROUTER_API_KEY, unless -header-fd is explicitly selected

stdin: one JSON object with version:1, state, and nonempty questions
state: a JSON string, object, or array; numeric literals retain precision
questions: named choice, score, or probability questions; see weigh.1
stdout: one fully validated JSON result, followed by a newline
stderr: diagnostics only; input and provider error bodies are never dumped

limits: 8 MiB input/response, depth 64, 1024 questions, 2-255 choices,
        2-10 ordered score levels, 8192-byte authorization header
distributions: complete support; bounded hundredth rounding (see weigh.1)
               native values retained; 1e-6 numerical slack

exit: 0 valid inference (including negative or uncertain answers)
      1 runtime, credential, provider, protocol, or output failure
      2 invalid invocation or input; no inference attempted

No retries, redirects, history, fallback, thresholds, or command execution.
Record can retain input/output; callers own decisions and check exit mapping.
`

type headerOpener func(int) (io.ReadCloser, error)

func main() {
	signal.Ignore(syscall.SIGPIPE) // Report a broken stdout as an output error.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv, func(fd int) (io.ReadCloser, error) {
		f := os.NewFile(uintptr(fd), "authorization")
		if f == nil {
			return nil, errors.New("invalid descriptor")
		}
		return f, nil
	}))
}

func run(ctx context.Context, args []string, in io.Reader, out, diag io.Writer, getenv func(string) string, openHeader headerOpener) int {
	fail := func(status int, message string) int {
		fmt.Fprintln(diag, "weigh: "+message)
		return status
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "version") {
		text := help
		if args[0] == "version" {
			text = "weigh " + version + "\n"
		}
		if _, err := io.WriteString(out, text); err != nil {
			return fail(1, "could not write output")
		}
		return 0
	}
	fs := flag.NewFlagSet("weigh", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors can contain caller-supplied secrets.
	model := fs.String("m", "", "explicit model")
	timeout := fs.Duration("timeout", 30*time.Second, "deadline")
	endpoint := fs.String("endpoint", defaultEndpoint, "Decisions endpoint")
	headerFD := fs.Int("header-fd", -1, "authorization descriptor")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := io.WriteString(out, help); err != nil {
				return fail(1, "could not write output")
			}
			return 0
		}
		return fail(2, "invalid flags; see weigh help")
	}
	if fs.NArg() != 0 {
		return fail(2, "unexpected arguments; see weigh help")
	}
	if !strings.HasPrefix(*model, "openrouter/") || len(*model) <= len("openrouter/") || !cleanText(*model) || strings.ContainsAny(*model, " \t\r\n") {
		return fail(2, "-m requires an explicit openrouter/MODEL")
	}
	if *timeout <= 0 {
		return fail(2, "-timeout must be positive")
	}
	if err := validateEndpoint(*endpoint); err != nil {
		return fail(2, err.Error())
	}
	hasHeader := false
	fs.Visit(func(f *flag.Flag) { hasHeader = hasHeader || f.Name == "header-fd" })
	if hasHeader && *headerFD < 3 {
		return fail(2, "-header-fd must be at least 3")
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	raw, err := readContext(ctx, in, maxBytes)
	if err != nil {
		if ctx.Err() != nil {
			return fail(1, "interrupted or timed out while reading input")
		}
		if errors.Is(err, errTooLarge) {
			return fail(2, "request exceeds the 8 MiB limit")
		}
		return fail(1, "could not read request")
	}
	req, err := parseRequest(raw)
	if err != nil {
		return fail(2, err.Error())
	}
	var authorization string
	if hasHeader {
		f, err := openHeader(*headerFD)
		if err != nil {
			return fail(1, "could not open authorization descriptor")
		}
		defer f.Close()
		b, err := readContext(ctx, f, maxHeader)
		if err != nil {
			return fail(1, "could not read bounded authorization header")
		}
		authorization, err = parseAuthorization(b)
		if err != nil {
			return fail(1, "invalid authorization header")
		}
	} else {
		key := getenv("OPENROUTER_API_KEY")
		if key == "" || len(key) > maxHeader-7 || !headerValue(key) || strings.TrimSpace(key) != key {
			return fail(1, "OPENROUTER_API_KEY is missing or invalid")
		}
		authorization = "Bearer " + key
	}
	result, err := infer(ctx, *endpoint, *model, authorization, req)
	if err != nil {
		return fail(1, err.Error())
	}
	if ctx.Err() != nil {
		return fail(1, "inference interrupted or timed out")
	}
	result = append(result, '\n')
	if n, err := out.Write(result); err != nil || n != len(result) {
		return fail(1, "could not write result")
	}
	return 0
}

// Ordinary stdin is finite by contract, but cancellation must not wait for EOF.
// The process exits after cancellation; owned descriptor/body readers are closed
// by their caller. A reader supplied by an embedding caller remains caller-owned.
func readContext(ctx context.Context, r io.Reader, limit int64) ([]byte, error) {
	type outcome struct {
		data []byte
		err  error
	}
	ready := make(chan outcome, 1)
	go func() {
		b, err := io.ReadAll(io.LimitReader(r, limit+1))
		if int64(len(b)) > limit {
			err = errTooLarge
		}
		ready <- outcome{b, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ready:
		return result.data, result.err
	}
}
