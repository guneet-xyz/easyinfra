package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/doctor"
)

func sampleReport() doctor.Report {
	return doctor.Report{
		Checks: []doctor.Result{
			{Name: "binary:helm", Status: doctor.StatusOK, Message: "helm found", Detail: "/usr/local/bin/helm"},
			{Name: "kube-context", Status: doctor.StatusWarn, Message: "skipped", Detail: ""},
			{Name: "config", Status: doctor.StatusFail, Message: "invalid", Detail: "missing field"},
		},
		Failed: true,
	}
}

func TestJSONWriter_RoundTrip(t *testing.T) {
	rep := sampleReport()
	var buf bytes.Buffer
	if err := (JSONWriter{}).WriteReport(&buf, rep); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}

	var got Envelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\npayload=%s", err, buf.String())
	}

	if got.Version != EnvelopeVersion {
		t.Errorf("version: got %q want %q", got.Version, EnvelopeVersion)
	}
	if !got.Failed {
		t.Errorf("failed: got false want true")
	}
	if len(got.Checks) != len(rep.Checks) {
		t.Fatalf("checks len: got %d want %d", len(got.Checks), len(rep.Checks))
	}
	for i, c := range got.Checks {
		want := rep.Checks[i]
		if c.Name != want.Name || c.Status != string(want.Status) || c.Message != want.Message || c.Detail != want.Detail {
			t.Errorf("check[%d]: got %+v want %+v", i, c, want)
		}
	}
}

func TestJSONWriter_UnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	err := (JSONWriter{}).WriteReport(&buf, 42)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestTextWriter(t *testing.T) {
	rep := sampleReport()
	var buf bytes.Buffer
	if err := (TextWriter{}).WriteReport(&buf, rep); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"STATUS", "NAME", "MESSAGE", "binary:helm", "kube-context", "one or more checks FAILED"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestTextWriter_UnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	if err := (TextWriter{}).WriteReport(&buf, "nope"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}
