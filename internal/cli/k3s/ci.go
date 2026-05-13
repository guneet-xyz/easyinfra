package k3s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/doctor"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/guneet-xyz/easyinfra/pkg/postrender"
	"github.com/guneet-xyz/easyinfra/pkg/render"
	"github.com/spf13/cobra"
)

const (
	stepStatusPass = "pass"
	stepStatusWarn = "warn"
	stepStatusFail = "fail"
	stepStatusSkip = "skip"
)

const validateReportVersion = "v1"

// StepResult is one entry in a ValidateReport.
type StepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ValidateReport is the JSON envelope emitted by `k3s ci validate`.
type ValidateReport struct {
	Version string       `json:"version"`
	Steps   []StepResult `json:"steps"`
	Failed  bool         `json:"failed"`
}

type ciValidateFlags struct {
	json           bool
	failOnWarnings bool
}

func newCICmd(flags *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "CI helpers for infra repos",
	}
	cmd.AddCommand(newCIValidateCmd(flags))
	return cmd
}

func newCIValidateCmd(flags *RootFlags) *cobra.Command {
	cf := &ciValidateFlags{}
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run all offline checks suitable for CI (no cluster required)",
		Long: "Run config validation, doctor checks (no-cluster), post-renderer probe, " +
			"and offline render of every app. Suitable for CI pipelines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCIValidate(cmd, flags, cf)
		},
	}
	cmd.Flags().BoolVar(&cf.json, "json", false, "Emit machine-readable JSON report")
	cmd.Flags().BoolVar(&cf.failOnWarnings, "fail-on-warnings", false, "Treat warnings as failures for exit code")
	return cmd
}

func runCIValidate(cmd *cobra.Command, flags *RootFlags, cf *ciValidateFlags) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	report := ValidateReport{Version: validateReportVersion}

	cfgPath := flags.Config
	if cfgPath == "" {
		p, err := paths.DefaultConfigPath()
		if err != nil {
			report.Steps = append(report.Steps, StepResult{
				Name:    "config",
				Status:  stepStatusFail,
				Message: fmt.Sprintf("resolving config path: %v", err),
			})
			report.Failed = true
			return finishCIValidate(out, report, cf)
		}
		cfgPath = p
	}

	cfg, err := config.LoadV2(cfgPath)
	if err != nil {
		report.Steps = append(report.Steps, StepResult{
			Name:    "config",
			Status:  stepStatusFail,
			Message: fmt.Sprintf("load %s: %v", cfgPath, err),
		})
		report.Failed = true
		return finishCIValidate(out, report, cf)
	}
	report.Steps = append(report.Steps, StepResult{
		Name:    "config",
		Status:  stepStatusPass,
		Message: fmt.Sprintf("loaded %s (%d apps)", cfgPath, len(cfg.Apps)),
	})
	baseDir := filepath.Dir(cfgPath)

	report.Steps = append(report.Steps, StepResult{
		Name:    "topo",
		Status:  stepStatusSkip,
		Message: "SKIP topo (not yet implemented)",
	})

	runner := newRunner(cmd, flags)
	checks := doctor.DefaultChecks(cfg, baseDir, runner, true)
	report.Steps = append(report.Steps, doctorStepResult(doctor.Run(ctx, checks)))

	if cfg.Defaults.PostRenderer != nil {
		probe := postrender.Probe(cfg.Defaults.PostRenderer)
		report.Steps = append(report.Steps, postRenderProbeResult(cfg.Defaults.PostRenderer, probe))
	} else {
		report.Steps = append(report.Steps, StepResult{
			Name:    "postrenderer",
			Status:  stepStatusSkip,
			Message: "no post-renderer configured",
		})
	}

	report.Steps = append(report.Steps, renderAllStep(ctx, runner, cfg, baseDir))

	report.Steps = append(report.Steps, StepResult{
		Name:    "deps",
		Status:  stepStatusSkip,
		Message: "SKIP deps (not yet implemented)",
	})

	report.Failed = anyFailed(report.Steps)
	return finishCIValidate(out, report, cf)
}

func doctorStepResult(r doctor.Report) StepResult {
	pass, warn, fail := 0, 0, 0
	for _, c := range r.Checks {
		switch c.Status {
		case doctor.StatusOK:
			pass++
		case doctor.StatusWarn:
			warn++
		case doctor.StatusFail:
			fail++
		}
	}
	status := stepStatusPass
	if fail > 0 {
		status = stepStatusFail
	} else if warn > 0 {
		status = stepStatusWarn
	}
	return StepResult{
		Name:    "doctor",
		Status:  status,
		Message: fmt.Sprintf("%d ok, %d warn, %d fail", pass, warn, fail),
	}
}

func postRenderProbeResult(pr *config.PostRenderer, res postrender.Result) StepResult {
	if !res.Found {
		return StepResult{
			Name:    "postrenderer",
			Status:  stepStatusWarn,
			Message: fmt.Sprintf("post-renderer %q not found on PATH", pr.Command),
		}
	}
	msg := fmt.Sprintf("found %s at %s", pr.Command, res.Path)
	if res.Version != "" {
		msg += " (" + res.Version + ")"
	}
	return StepResult{Name: "postrenderer", Status: stepStatusPass, Message: msg}
}

func renderAllStep(ctx context.Context, runner exec.Runner, cfg *config.InfraConfigV2, baseDir string) StepResult {
	client := &helm.Client{Runner: runner, Context: cfg.Cluster.KubeContext}
	results, err := render.RenderAll(ctx, client, cfg, render.Options{
		BaseDir:          baseDir,
		PostRendererMode: render.PostRendererSkip,
	})
	if err != nil {
		return StepResult{Name: "render", Status: stepStatusFail, Message: err.Error()}
	}
	rendered, skipped := 0, 0
	for _, res := range results {
		if res.Skipped {
			skipped++
			continue
		}
		rendered++
	}
	return StepResult{
		Name:    "render",
		Status:  stepStatusPass,
		Message: fmt.Sprintf("%d rendered, %d skipped (library)", rendered, skipped),
	}
}

func anyFailed(steps []StepResult) bool {
	for _, s := range steps {
		if s.Status == stepStatusFail {
			return true
		}
	}
	return false
}

func finishCIValidate(out io.Writer, report ValidateReport, cf *ciValidateFlags) error {
	if cf.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		writeCIValidateText(out, report)
	}

	if report.Failed {
		return fmt.Errorf("ci validate: one or more steps failed")
	}
	if cf.failOnWarnings {
		for _, s := range report.Steps {
			if s.Status == stepStatusWarn {
				return fmt.Errorf("ci validate: warnings present and --fail-on-warnings set")
			}
		}
	}
	return nil
}

func writeCIValidateText(out io.Writer, report ValidateReport) {
	for _, s := range report.Steps {
		fmt.Fprintf(out, "%s\t%s\t%s\n", ciStatusSymbol(s.Status), s.Name, s.Message)
	}
	if report.Failed {
		fmt.Fprintln(out, "ci validate: FAILED")
	} else {
		fmt.Fprintln(out, "ci validate: ok")
	}
}

func ciStatusSymbol(s string) string {
	switch s {
	case stepStatusPass:
		return "PASS"
	case stepStatusWarn:
		return "WARN"
	case stepStatusFail:
		return "FAIL"
	case stepStatusSkip:
		return "SKIP"
	default:
		return "?"
	}
}
