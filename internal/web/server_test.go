package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderLogLinePreservesANSIFormatting(t *testing.T) {
	line := "2026/05/13 - 11:22:44 | sishcgo.gn.gy |\x1b[97;42m 200 \x1b[0m|  112.948676ms |   148.123.47.18 |\x1b[97;44m GET     \x1b[0m /static/styles.css"

	rendered := string(renderLogLine(line))
	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered output still contains ANSI escape codes: %q", rendered)
	}
	if !strings.Contains(rendered, `class="ansi-fg-97 ansi-bg-42"`) {
		t.Fatalf("missing expected foreground/background class: %q", rendered)
	}
	if !strings.Contains(rendered, `class="ansi-fg-97 ansi-bg-44"`) {
		t.Fatalf("missing expected GET class: %q", rendered)
	}
	if !strings.Contains(rendered, " 200 ") || !strings.Contains(rendered, " GET     ") {
		t.Fatalf("rendered output lost log text: %q", rendered)
	}
}

func TestRenderLogLinesReverseOrder(t *testing.T) {
	lines := []string{"oldest", "middle", "newest"}
	rendered := renderLogLines(lines)
	if len(rendered) != 3 {
		t.Fatalf("len(rendered) = %d, want 3", len(rendered))
	}
	if got := string(rendered[0]); got != "newest" {
		t.Fatalf("rendered[0] = %q, want newest", got)
	}
	if got := string(rendered[2]); got != "oldest" {
		t.Fatalf("rendered[2] = %q, want oldest", got)
	}
}

func TestHandleSettingsGetRendersContent(t *testing.T) {
	s := New("", "", "", "/")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/settings", nil)

	s.handleSettingsGet(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Fatalf("settings page did not render expected content: %q", body)
	}
}
