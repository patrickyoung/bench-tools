// This verifier reads source syntax only. It is not a shared tool runtime.
package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

type source struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Imports []string `json:"imports"`
	Error   string   `json:"error,omitempty"`
}

func main() {
	var paths []string
	if err := json.NewDecoder(os.Stdin).Decode(&paths); err != nil {
		panic(err)
	}
	results := make([]source, 0, len(paths))
	for _, path := range paths {
		r := source{Path: path, Imports: []string{}}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Package = f.Name.Name
			for _, spec := range f.Imports {
				value, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					r.Error = err.Error()
					break
				}
				r.Imports = append(r.Imports, value)
			}
		}
		results = append(results, r)
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}
