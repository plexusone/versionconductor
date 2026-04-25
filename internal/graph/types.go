package graph

import (
	"strings"

	"github.com/plexusone/versionconductor/pkg/model"
)

// Language represents the programming language of a module.
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguageSwift      Language = "swift"
	LanguagePython     Language = "python"
	LanguageRust       Language = "rust"
)

// ManifestFile returns the manifest file name for this language.
func (l Language) ManifestFile() string {
	switch l {
	case LanguageGo:
		return "go.mod"
	case LanguageTypeScript:
		return "package.json"
	case LanguageSwift:
		return "Package.swift"
	case LanguagePython:
		return "pyproject.toml"
	case LanguageRust:
		return "Cargo.toml"
	default:
		return ""
	}
}

// Portfolio represents a collection of GitHub orgs managed together.
type Portfolio struct {
	Name      string   `json:"name" yaml:"name"`
	Orgs      []string `json:"orgs" yaml:"orgs"`            // ["github.com/grokify", "github.com/agentplexus"]
	GraphRepo string   `json:"graphRepo" yaml:"graph_repo"` // Where to persist the graph
	Languages []string `json:"languages" yaml:"languages"`  // ["go", "typescript"]
}

// Module represents a dependency module in the graph.
type Module struct {
	// ID is a universal identifier: "go:github.com/grokify/mogo" or "npm:@agentplexus/core"
	ID string `json:"id"`

	// Language of this module
	Language Language `json:"language"`

	// Name is the module name (e.g., "github.com/grokify/mogo")
	Name string `json:"name"`

	// Org is the GitHub org (e.g., "github.com/grokify")
	Org string `json:"org"`

	// Version is the current version tag
	Version string `json:"version"`

	// Repo is the GitHub repository info (nil for external modules)
	Repo *model.Repo `json:"repo,omitempty"`

	// IsManaged is true if this module is in the portfolio
	IsManaged bool `json:"isManaged"`

	// Dependencies are modules this module depends on
	Dependencies []ModuleRef `json:"dependencies,omitempty"`

	// Dependents are modules that depend on this module
	Dependents []ModuleRef `json:"dependents,omitempty"`
}

// ModuleRef is a lightweight reference to a module with version.
type ModuleRef struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	IsManaged bool   `json:"isManaged"`
}

// NewModuleID creates a module ID from language and name.
func NewModuleID(lang Language, name string) string {
	return string(lang) + ":" + name
}

// ParseModuleID parses a module ID into language and name.
func ParseModuleID(id string) (Language, string) {
	idx := strings.Index(id, ":")
	if idx == -1 {
		return "", id
	}
	return Language(id[:idx]), id[idx+1:]
}

// ExtractOrg extracts the org from a module name.
// For Go modules: "github.com/grokify/mogo" -> "github.com/grokify"
// For npm: "@agentplexus/core" -> "@agentplexus"
func ExtractOrg(lang Language, name string) string {
	switch lang {
	case LanguageGo:
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return name
	case LanguageTypeScript:
		if strings.HasPrefix(name, "@") {
			idx := strings.Index(name, "/")
			if idx > 0 {
				return name[:idx]
			}
		}
		return ""
	default:
		return ""
	}
}

// GoModInfo contains parsed go.mod information.
type GoModInfo struct {
	Module  string          `json:"module"`
	Go      string          `json:"go"`
	Require []ModuleVersion `json:"require,omitempty"`
	Replace []ModuleReplace `json:"replace,omitempty"`
	Exclude []ModuleVersion `json:"exclude,omitempty"`
}

// ModuleVersion represents a module with its version.
type ModuleVersion struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Indirect bool   `json:"indirect,omitempty"`
}

// ModuleReplace represents a replace directive in go.mod.
type ModuleReplace struct {
	Old ModuleVersion `json:"old"`
	New ModuleVersion `json:"new"`
}

// GraphSnapshot represents a point-in-time snapshot of the dependency graph.
type GraphSnapshot struct {
	Portfolio Portfolio         `json:"portfolio"`
	Timestamp string            `json:"timestamp"`
	Modules   map[string]Module `json:"modules"` // keyed by module ID
}

// UpgradeOrder represents the recommended order for upgrading modules.
type UpgradeOrder struct {
	Modules []Module `json:"modules"`
	Cycles  []Cycle  `json:"cycles,omitempty"`
}

// Cycle represents a dependency cycle (should not happen in Go).
type Cycle struct {
	Modules []string `json:"modules"`
}

// StaleModule represents a module using an outdated dependency.
type StaleModule struct {
	Module     Module `json:"module"`
	Dependency string `json:"dependency"`
	Current    string `json:"current"`
	Latest     string `json:"latest"`
}
