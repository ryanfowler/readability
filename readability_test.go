package readability_test

import (
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/ryanfowler/readability"
)

func TestFunctionalOptions(t *testing.T) {
	_, err := readability.Parse(
		strings.NewReader("<html><body><p>article</p></body></html>"),
		"",
		readability.WithMaxElemsToParse(100),
		readability.WithMaxElemsToParse(1), // Later options take precedence.
	)
	var limitErr *readability.TooManyElementsError
	if !errors.As(err, &limitErr) {
		t.Fatalf("Parse() error = %v, want *TooManyElementsError", err)
	}
	if limitErr.Count <= limitErr.Max || limitErr.Max != 1 {
		t.Fatalf("TooManyElementsError = %+v", limitErr)
	}
}

func TestReaderableFunctionalOptions(t *testing.T) {
	source := `<article><p>` + strings.Repeat("readable content ", 20) + `</p></article>`
	if readability.IsProbablyReaderable(source, readability.WithMinScore(0), readability.WithMinContentLength(1)) != true {
		t.Fatal("IsProbablyReaderable() = false with permissive options")
	}
	if readability.IsProbablyReaderable(source, readability.WithMinScore(1e9), readability.WithMinContentLength(1)) != false {
		t.Fatal("IsProbablyReaderable() = true with high minimum score")
	}
}

func TestAllOptionsCanBeCombined(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	options := []readability.Option{
		readability.WithMaxElemsToParse(0),
		readability.WithNbTopCandidates(5),
		readability.WithCharThreshold(0),
		readability.WithClassesToPreserve("page"),
		readability.WithKeepClasses(false),
		readability.WithDisableJSONLD(false),
		readability.WithAllowedVideoRegex(regexp.MustCompile(`^https://example.com/video`)),
		readability.WithLinkDensityModifier(0),
		readability.WithLogger(logger),
		readability.WithDebug(false),
	}
	if _, err := readability.Parse(strings.NewReader("<html><body><article><p>article text</p></article></body></html>"), "", options...); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
}

func TestPublicErrors(t *testing.T) {
	_, err := readability.Parse(strings.NewReader("<html><head></head></html>"), "")
	if !errors.Is(err, readability.ErrNoBody) {
		t.Fatalf("Parse() error = %v, want ErrNoBody", err)
	}
}
