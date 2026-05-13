// Package output provides pluggable writers for command output.
//
// A Writer renders a structured report (typically a domain-specific Report
// struct) to an io.Writer in either human-readable or machine-readable form.
// This lets commands stay agnostic of the presentation layer.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/guneet-xyz/easyinfra/pkg/doctor"
)

// Envelope is the canonical JSON shape emitted by JSONWriter for doctor reports.
// Future commands may define their own envelopes; the version field allows
// consumers to detect breaking changes.
type Envelope struct {
	Version string   `json:"version"`
	Checks  []Check  `json:"checks"`
	Failed  bool     `json:"failed"`
}

// Check is a single result entry in the JSON envelope.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// EnvelopeVersion is the current schema version emitted by JSONWriter.
const EnvelopeVersion = "v1"

// Writer renders a report to w. The concrete type accepted depends on the
// implementation; unsupported types must return an error.
type Writer interface {
	WriteReport(w io.Writer, report any) error
}

// JSONWriter emits a JSON envelope with stable field names and a schema version.
type JSONWriter struct{}

// WriteReport marshals the supplied report into the canonical JSON envelope.
// Currently supports doctor.Report.
func (JSONWriter) WriteReport(w io.Writer, report any) error {
	env, err := toEnvelope(report)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// TextWriter emits a human-readable tabwriter table, mirroring doctor's
// historical text output. Currently supports doctor.Report.
type TextWriter struct{}

// WriteReport renders the report as an aligned text table.
func (TextWriter) WriteReport(w io.Writer, report any) error {
	r, ok := report.(doctor.Report)
	if !ok {
		return fmt.Errorf("output: TextWriter does not support type %T", report)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "STATUS\tNAME\tMESSAGE"); err != nil {
		return err
	}
	for _, c := range r.Checks {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", statusSymbol(c.Status), c.Name, c.Message); err != nil {
			return err
		}
		if c.Detail != "" {
			if _, err := fmt.Fprintf(tw, "\t\t  %s\n", c.Detail); err != nil {
				return err
			}
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	summary := "all checks passed"
	if r.Failed {
		summary = "one or more checks FAILED"
	}
	_, err := fmt.Fprintf(w, "\n%s\n", summary)
	return err
}

func toEnvelope(report any) (Envelope, error) {
	switch r := report.(type) {
	case doctor.Report:
		env := Envelope{
			Version: EnvelopeVersion,
			Checks:  make([]Check, 0, len(r.Checks)),
			Failed:  r.Failed,
		}
		for _, c := range r.Checks {
			env.Checks = append(env.Checks, Check{
				Name:    c.Name,
				Status:  string(c.Status),
				Message: c.Message,
				Detail:  c.Detail,
			})
		}
		return env, nil
	case Envelope:
		return r, nil
	default:
		return Envelope{}, fmt.Errorf("output: unsupported report type %T", report)
	}
}

func statusSymbol(s doctor.Status) string {
	switch s {
	case doctor.StatusOK:
		return "✓ OK  "
	case doctor.StatusWarn:
		return "⚠ WARN"
	case doctor.StatusFail:
		return "✗ FAIL"
	default:
		return string(s)
	}
}
