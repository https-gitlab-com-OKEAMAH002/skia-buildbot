// Parse the DEPS file and generate Go code with its contents.
//
//go:generate bazelisk run --config=mayberemote //:go -- run ./generate.go
package deps

import (
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
)

// Get retrieves the given dependency. Returns an error of the given dependency
// does not exist.
func Get(dep string) (*deps_parser.DepsEntry, error) {
	entry := deps.Get(dep)
	if entry == nil {
		return nil, skerr.Fmt("unknown dependency %q (normalized as %q)", dep, deps_parser.NormalizeDep(dep))
	}
	// Return a copy to prevent modification of the package-local entries.
	return &deps_parser.DepsEntry{
		Id:      entry.Id,
		Version: entry.Version,
		Path:    entry.Path,
	}, nil
}
