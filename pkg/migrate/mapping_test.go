package migrate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMappingCompleteness(t *testing.T) {
	got := len(Mappings())
	if got < 10 {
		t.Fatalf("expected at least 10 mappings, got %d", got)
	}
}

func TestMappingNonEmpty(t *testing.T) {
	for i, m := range Mappings() {
		if strings.TrimSpace(m.EasyinfraCmd) == "" {
			t.Errorf("row %d (%q) has empty EasyinfraCmd", i, m.ScriptInvocation)
		}
		if strings.TrimSpace(m.Script) == "" {
			t.Errorf("row %d has empty Script", i)
		}
		if strings.TrimSpace(m.ScriptInvocation) == "" {
			t.Errorf("row %d has empty ScriptInvocation", i)
		}
	}
}

func TestRenderMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "markdown"); err != nil {
		t.Fatalf("Render markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "| deploy.sh |") {
		t.Errorf("markdown output missing %q row; got:\n%s", "| deploy.sh |", out)
	}
	if !strings.Contains(out, "easyinfra equivalent") {
		t.Errorf("markdown header missing")
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "json"); err != nil {
		t.Fatalf("Render json: %v", err)
	}
	var rows []Mapping
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("json output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != len(Mappings()) {
		t.Errorf("json row count = %d, want %d", len(rows), len(Mappings()))
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "yaml"); err == nil {
		t.Fatalf("expected error for unsupported format, got nil")
	}
}
