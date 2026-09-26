// Moniker reserves friendly team names in a caller-selected private registry.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const version = "0.1.0"

func main() { os.Exit(command(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func command(args []string, in io.Reader, out, diag io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "version", "-V", "--version":
			if len(args) != 1 {
				return problem(diag, "version takes no arguments", 2)
			}
			if _, err := fmt.Fprintln(out, "moniker "+version); err != nil {
				return problem(diag, err, 1)
			}
			return 0
		case "help", "-h", "--help":
			fmt.Fprint(out, help)
			return 0
		case "mcp":
			return mcpCommand(args[1:], in, out, diag)
		}
	}
	fs := flag.NewFlagSet("moniker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "absolute private registry")
	theme := fs.String("theme", "playful", "playful, space or nature")
	asJSON := fs.Bool("json", false, "print a JSON object")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(out, help)
			return 0
		}
		return problem(diag, err, 2)
	}
	if fs.NArg() != 0 {
		return problem(diag, "unexpected arguments; see moniker help", 2)
	}
	if err := registryArgument(*dir); err != nil {
		return problem(diag, err, 2)
	}
	if _, ok := themes[*theme]; !ok {
		return problem(diag, "theme must be playful, space or nature", 2)
	}
	name, err := generate(*dir, *theme)
	if err != nil {
		return problem(diag, err, 1)
	}
	if *asJSON {
		err = json.NewEncoder(out).Encode(name)
	} else {
		_, err = fmt.Fprintln(out, name.Name)
	}
	if err != nil {
		return problem(diag, err, 1)
	}
	return 0
}

func registryArgument(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return fmt.Errorf("-dir must select an absolute private registry path")
	}
	return nil
}

func problem(diag io.Writer, err any, code int) int {
	fmt.Fprintln(diag, "moniker:", err)
	return code
}

const help = `moniker - reserve a friendly, unique team name

  moniker -dir ABS_REGISTRY [-theme playful|space|nature] [-json]
  moniker mcp -dir ABS_REGISTRY tools/call
  moniker version

The explicit private registry remembers reservations across concurrent calls
and restarts. Missing directories are created with mode 0700. Names are never
automatically released. Plain stdout is one name; -json returns id, name, slug,
and theme. The default theme is playful. No model, network or ambient state.

MCP uses mcp/manifest.json with the separate mcpserve command. The adapter
reads bounded tools/call params from stdin, reserves locally, and writes one
MCP result. Tool errors use isError with exit 0; unsupported methods use a
dispatcher error with exit 1. Arguments cannot select another registry.

Ordinary exits: 0 reserved, 1 runtime failure, 2 invalid invocation.
A reservation can remain after a failed write or interrupted response;
retrying makes a new reservation, never an acknowledgement of the old one.
`
