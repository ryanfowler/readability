package readability_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ryanfowler/readability"
)

func TestPublicDefaults(t *testing.T) {
	options := readability.DefaultOptions()
	if options.NbTopCandidates != 5 || options.CharThreshold != 500 {
		t.Fatalf("DefaultOptions() = %+v", options)
	}
	if len(options.ClassesToPreserve) != 1 || options.ClassesToPreserve[0] != "page" {
		t.Fatalf("ClassesToPreserve = %v", options.ClassesToPreserve)
	}
	if options.AllowedVideoRegex == nil {
		t.Fatal("AllowedVideoRegex is nil")
	}

	readerable := readability.DefaultReaderableOptions()
	if readerable.MinScore != 20 || readerable.MinContentLength != 140 {
		t.Fatalf("DefaultReaderableOptions() = %+v", readerable)
	}
}

func TestPublicErrors(t *testing.T) {
	_, err := readability.Parse(strings.NewReader("<html><head></head></html>"), "", nil)
	if !errors.Is(err, readability.ErrNoBody) {
		t.Fatalf("Parse() error = %v, want ErrNoBody", err)
	}

	options := readability.DefaultOptions()
	options.MaxElemsToParse = 1
	_, err = readability.Parse(strings.NewReader("<html><body><p>article</p></body></html>"), "", &options)
	var limitErr *readability.TooManyElementsError
	if !errors.As(err, &limitErr) {
		t.Fatalf("Parse() error = %v, want *TooManyElementsError", err)
	}
	if limitErr.Count <= limitErr.Max || limitErr.Max != 1 {
		t.Fatalf("TooManyElementsError = %+v", limitErr)
	}
}
