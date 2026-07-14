package statusline

import (
	"strings"
	"testing"

	"github.com/rcverse/cc-watch/internal/config"
)

func TestRenderUsesIndependentElementLayoutAndOrder(t *testing.T) {
	cfg := config.DefaultStatusline()
	cfg.Order = []string{config.StatuslineElementCache, config.StatuslineElementUsage, config.StatuslineElementWarning}
	cfg.Cache.Layout = config.StatuslineLayoutSameLine
	cfg.Usage.Layout = config.StatuslineLayoutNewLine
	cfg.Warning.Enabled = false

	got := Render("base", cfg, map[string]string{
		config.StatuslineElementUsage:   "usage",
		config.StatuslineElementCache:   "cache",
		config.StatuslineElementWarning: "warning",
	})
	if got != "base | cache\nusage" {
		t.Fatalf("Render() = %q, want independently ordered and placed elements", got)
	}
}

func TestRenderSkipsDisabledAndEmptyElements(t *testing.T) {
	cfg := config.DefaultStatusline()
	cfg.Warning.Enabled = false
	got := Render("base", cfg, map[string]string{
		config.StatuslineElementUsage:   "",
		config.StatuslineElementWarning: "warning",
	})
	if got != "base" {
		t.Fatalf("Render() = %q, want base only when enabled values are empty", got)
	}
}

func TestPreviewUsesConfiguredFormatsAndLayouts(t *testing.T) {
	cfg := config.DefaultStatusline()
	cfg.Usage.Format = config.StatuslineFormatCompact
	cfg.Usage.Layout = config.StatuslineLayoutNewLine
	cfg.Warning.Format = config.StatuslineWarningFormatVerbose
	cfg.Warning.Layout = config.StatuslineLayoutSameLine
	cfg.Cache.Format = config.StatuslineFormatCompact

	got := Preview(cfg)
	for _, want := range []string{"34%/41%", "✓ KA OK", "32m41s"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Preview() = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "⏱") || strings.Contains(got, "KeepAlive at risk") {
		t.Fatalf("Preview() retained full-format samples: %q", got)
	}
	if got != "base\n34%/41% | ✓ KA OK\n32m41s" {
		t.Fatalf("Preview() = %q, want configured order and layouts", got)
	}
}
