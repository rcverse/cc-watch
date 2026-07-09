package main

import (
	"os"
	"strings"
	"testing"
)

func TestDemoToolSourceRequiresDemoBuildTag(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "//go:build demo") {
		t.Fatalf("main.go missing demo build tag:\n%s", text)
	}
	if strings.Contains(text, "internal/app") || strings.Contains(text, "ParseArgs") {
		t.Fatalf("demo tool should not add production app flags:\n%s", text)
	}
}
