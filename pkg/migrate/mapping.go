// Package migrate provides a mapping table from legacy shell-script
// invocations (deploy.sh, validate.sh, backup.sh) to their equivalent
// easyinfra CLI commands. It is used by the `easyinfra migrate` command
// (and documentation generators) to help operators move from the bespoke
// scripts under machines/pax/k3s to the unified CLI.
package migrate

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Mapping represents a single row in the migration table: a legacy script
// invocation paired with its easyinfra equivalent.
type Mapping struct {
	// Script is the legacy script filename (e.g. "deploy.sh").
	Script string `json:"script"`
	// ScriptInvocation is an example invocation including arguments.
	ScriptInvocation string `json:"script_invocation"`
	// EasyinfraCmd is the equivalent easyinfra CLI command.
	EasyinfraCmd string `json:"easyinfra_cmd"`
	// Notes contains any caveats or additional context.
	Notes string `json:"notes,omitempty"`
}

// Mappings returns the canonical list of script→command mappings.
// The slice is constructed fresh on each call so callers may safely mutate
// it without affecting subsequent invocations.
func Mappings() []Mapping {
	return []Mapping{
		{
			Script:           "deploy.sh",
			ScriptInvocation: "deploy.sh <app> install",
			EasyinfraCmd:     "easyinfra k3s install <app>",
			Notes:            "Install a single chart by name.",
		},
		{
			Script:           "deploy.sh",
			ScriptInvocation: "deploy.sh <app> upgrade",
			EasyinfraCmd:     "easyinfra k3s upgrade <app>",
			Notes:            "Upgrade an existing release.",
		},
		{
			Script:           "deploy.sh",
			ScriptInvocation: "deploy.sh <app> uninstall",
			EasyinfraCmd:     "easyinfra k3s uninstall <app>",
			Notes:            "Uninstall a release; PVCs are preserved.",
		},
		{
			Script:           "deploy.sh",
			ScriptInvocation: "deploy.sh --all install",
			EasyinfraCmd:     "easyinfra k3s install --all",
			Notes:            "Install every chart discovered under apps/.",
		},
		{
			Script:           "deploy.sh",
			ScriptInvocation: "deploy.sh --all upgrade",
			EasyinfraCmd:     "easyinfra k3s upgrade --all",
			Notes:            "Upgrade every installed release.",
		},
		{
			Script:           "validate.sh",
			ScriptInvocation: "validate.sh",
			EasyinfraCmd:     "easyinfra k3s ci validate",
			Notes:            "Lint and template every chart for CI.",
		},
		{
			Script:           "validate.sh",
			ScriptInvocation: "validate.sh <app>",
			EasyinfraCmd:     "easyinfra k3s render <app>",
			Notes:            "Render a single chart's manifests to stdout.",
		},
		{
			Script:           "backup.sh",
			ScriptInvocation: "backup.sh backup",
			EasyinfraCmd:     "easyinfra k3s backup run --all",
			Notes:            "Snapshot every PVC across known apps.",
		},
		{
			Script:           "backup.sh",
			ScriptInvocation: "backup.sh backup <app>",
			EasyinfraCmd:     "easyinfra k3s backup run --app <app>",
			Notes:            "Snapshot a single app's PVCs.",
		},
		{
			Script:           "backup.sh",
			ScriptInvocation: "backup.sh restore <app> latest",
			EasyinfraCmd:     "easyinfra k3s restore <app> --latest",
			Notes:            "Restore the most recent snapshot for the app.",
		},
		{
			Script:           "backup.sh",
			ScriptInvocation: "backup.sh restore <app> <timestamp>",
			EasyinfraCmd:     "easyinfra k3s restore <app> --timestamp <ts>",
			Notes:            "Restore a specific snapshot identified by timestamp.",
		},
		{
			Script:           "backup.sh",
			ScriptInvocation: "backup.sh list",
			EasyinfraCmd:     "easyinfra k3s backup list",
			Notes:            "List available snapshots.",
		},
	}
}

// Render writes the mapping table to w in the requested format.
// Supported formats are "markdown" and "json".
func Render(w io.Writer, format string) error {
	switch strings.ToLower(format) {
	case "markdown", "md":
		return renderMarkdown(w, Mappings())
	case "json":
		return renderJSON(w, Mappings())
	default:
		return fmt.Errorf("migrate: unsupported render format %q (want \"markdown\" or \"json\")", format)
	}
}

func renderMarkdown(w io.Writer, rows []Mapping) error {
	var b strings.Builder
	b.WriteString("| Script | Invocation | easyinfra equivalent | Notes |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s |\n",
			r.Script,
			escapeMarkdownCell(r.ScriptInvocation),
			escapeMarkdownCell(r.EasyinfraCmd),
			escapeMarkdownCell(r.Notes),
		)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func renderJSON(w io.Writer, rows []Mapping) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
