package readability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

const loggingTestDocument = `<html><head><title>Logging</title></head><body><article><p>This is article content used to exercise extraction logging.</p></article></body></html>`

func TestParseUsesOptionsLogger(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	opts := DefaultOptions()
	opts.CharThreshold = 0
	opts.Logger = logger
	opts.Debug = true

	if _, err := Parse(strings.NewReader(loggingTestDocument), "https://example.com/article", &opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "grabArticle") {
		t.Fatalf("logger output = %q, want extraction log", output.String())
	}
}

func TestParseDebugDisabled(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	opts := DefaultOptions()
	opts.CharThreshold = 0
	opts.Logger = logger

	if _, err := Parse(strings.NewReader(loggingTestDocument), "https://example.com/article", &opts); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("logger unexpectedly received debug output: %q", output.String())
	}
}

func TestParseDoesNotUseDefaultLogger(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	opts := DefaultOptions()
	opts.CharThreshold = 0
	if _, err := Parse(strings.NewReader(loggingTestDocument), "https://example.com/article", &opts); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("default logger unexpectedly received output: %q", output.String())
	}
}
