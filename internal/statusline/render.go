package statusline

import (
	"strings"

	"github.com/rcverse/cc-watch/internal/config"
)

// Render appends the enabled statusline elements in their configured order.
// Each element owns its placement, so adding a new element cannot change the
// layout semantics of the others.
func Render(base string, cfg config.StatuslineConfig, values map[string]string) string {
	cfg = config.NormalizeStatusline(cfg)
	result := base
	hasOutput := result != ""
	for _, name := range cfg.Order {
		element := elementConfig(cfg, name)
		value := values[name]
		if !element.Enabled || value == "" {
			continue
		}
		if hasOutput {
			if element.Layout == config.StatuslineLayoutNewLine {
				result += "\n"
			} else {
				result += " | "
			}
		}
		result += value
		hasOutput = true
	}
	return result
}

func elementConfig(cfg config.StatuslineConfig, name string) config.StatuslineElementConfig {
	switch name {
	case config.StatuslineElementUsage:
		return cfg.Usage
	case config.StatuslineElementWarning:
		return cfg.Warning
	case config.StatuslineElementCache:
		return cfg.Cache
	default:
		return config.StatuslineElementConfig{}
	}
}

func Preview(cfg config.StatuslineConfig) string {
	cfg = config.NormalizeStatusline(cfg)
	return Render("base", cfg, map[string]string{
		config.StatuslineElementUsage:   previewUsage(cfg.Usage.Format),
		config.StatuslineElementWarning: previewWarning(cfg.Warning.Format),
		config.StatuslineElementCache:   previewCache(cfg.Cache.Format),
	})
}

func previewUsage(format string) string {
	if format == config.StatuslineFormatCompact {
		return "34%/41%"
	}
	return "⏱ 34% (5h) / 41% (7d) used"
}

func previewWarning(format string) string {
	if format == config.StatuslineWarningFormatVerbose {
		return "✓ KA OK"
	}
	return "⚠ KeepAlive at risk"
}

func previewCache(format string) string {
	if format == config.StatuslineFormatCompact {
		return "32m41s"
	}
	return "⌛ 32m41s left · 1h cache"
}

func OrderText(order []string) string {
	labels := make([]string, 0, len(order))
	for _, name := range order {
		labels = append(labels, elementLabel(name))
	}
	return strings.Join(labels, " → ")
}

func ElementLabel(name string) string {
	return elementLabel(name)
}

func elementLabel(name string) string {
	switch name {
	case config.StatuslineElementUsage:
		return "Usage"
	case config.StatuslineElementWarning:
		return "KA warning"
	case config.StatuslineElementCache:
		return "Cache timing"
	default:
		return name
	}
}
