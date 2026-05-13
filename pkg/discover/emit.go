// Package discover converts a Layout into a v2 infra.yaml document.
package discover

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Emit writes a deterministic v2 infra.yaml derived from layout to w.
// Library charts are skipped; localDeps are converted to dependsOn basenames.
func Emit(layout *Layout, w io.Writer) error {
	if layout == nil {
		return fmt.Errorf("discover: nil layout")
	}

	clusterName := filepath.Base(layout.Root)
	if clusterName == "." || clusterName == string(filepath.Separator) || clusterName == "" {
		clusterName = "cluster"
	}

	if _, err := fmt.Fprintf(w,
		"apiVersion: easyinfra/v2\n"+
			"cluster:\n"+
			"  name: %s\n"+
			"  type: k3s\n"+
			"  kubeContext: %s\n",
		clusterName, clusterName,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "defaults:"); err != nil {
		return err
	}
	shared := append([]string(nil), layout.SharedValues...)
	sort.Strings(shared)
	relShared := make([]string, 0, len(shared))
	for _, s := range shared {
		if rel, err := filepath.Rel(layout.Root, s); err == nil {
			relShared = append(relShared, rel)
		} else {
			relShared = append(relShared, s)
		}
	}
	if _, err := fmt.Fprintf(w, "  valueFiles: %s\n", yamlInlineStringList(relShared)); err != nil {
		return err
	}
	if layout.PostRendererHint != nil {
		if _, err := fmt.Fprintln(w, "  postRenderer:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "    binary: %s\n", yamlScalar(layout.PostRendererHint.Binary)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "    args: %s\n", yamlInlineStringList(layout.PostRendererHint.Args)); err != nil {
			return err
		}
	}

	apps := make([]AppLayout, 0, len(layout.Apps))
	for _, a := range layout.Apps {
		if a.IsLibrary {
			continue
		}
		apps = append(apps, a)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })

	if _, err := fmt.Fprintln(w, "apps:"); err != nil {
		return err
	}

	for _, a := range apps {
		chartRel := a.ChartPath
		if rel, err := filepath.Rel(layout.Root, a.ChartPath); err == nil {
			chartRel = rel
		}
		if _, err := fmt.Fprintf(w, "  # discovered at: %s\n", chartRel); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  - name: %s\n", yamlScalar(a.Name)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "    chart: %s\n", yamlScalar(chartRel)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "    namespace: %s\n", yamlScalar(a.Name)); err != nil {
			return err
		}
		vfs := make([]string, 0, len(a.ValueFiles))
		for _, vf := range a.ValueFiles {
			vfs = append(vfs, filepath.Base(vf))
		}
		sort.Strings(vfs)
		if _, err := fmt.Fprintf(w, "    valueFiles: %s\n", yamlInlineStringList(vfs)); err != nil {
			return err
		}
		if len(a.LocalDeps) > 0 {
			deps := make([]string, 0, len(a.LocalDeps))
			for _, d := range a.LocalDeps {
				deps = append(deps, filepath.Base(d))
			}
			sort.Strings(deps)
			if _, err := fmt.Fprintf(w, "    dependsOn: %s\n", yamlInlineStringList(deps)); err != nil {
				return err
			}
		}
	}

	return nil
}

func yamlInlineStringList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = yamlScalar(it)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`\t\n") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		esc := strings.ReplaceAll(s, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		return `"` + esc + `"`
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return `"` + s + `"`
	}
	return s
}
