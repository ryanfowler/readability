package readability

import (
	"io"
	"log/slog"
	"reflect"
	"regexp"
	"testing"

	"github.com/ryanfowler/readability/internal/engine"
)

func TestOptionsToEngineDefaults(t *testing.T) {
	got := optionsToEngine(nil)
	want := engine.DefaultOptions()
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("optionsToEngine(nil) = %+v, want %+v", *got, want)
	}
}

func TestOptionsToEngineMappings(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	videoPattern := regexp.MustCompile(`^https://videos\.example/`)

	got := optionsToEngine([]Option{
		WithMaxElemsToParse(123),
		WithNbTopCandidates(7),
		WithCharThreshold(321),
		WithClassesToPreserve("article", "highlight"),
		WithKeepClasses(true),
		WithDisableJSONLD(true),
		WithAllowedVideoRegex(videoPattern),
		WithLinkDensityModifier(0.25),
		WithLogger(logger),
		WithDebug(true),
	})

	if got.MaxElemsToParse != 123 {
		t.Errorf("MaxElemsToParse = %d, want 123", got.MaxElemsToParse)
	}
	if got.NbTopCandidates != 7 {
		t.Errorf("NbTopCandidates = %d, want 7", got.NbTopCandidates)
	}
	if got.CharThreshold != 321 {
		t.Errorf("CharThreshold = %d, want 321", got.CharThreshold)
	}
	if want := []string{"article", "highlight"}; !reflect.DeepEqual(got.ClassesToPreserve, want) {
		t.Errorf("ClassesToPreserve = %v, want %v", got.ClassesToPreserve, want)
	}
	if !got.KeepClasses {
		t.Error("KeepClasses = false, want true")
	}
	if !got.DisableJSONLD {
		t.Error("DisableJSONLD = false, want true")
	}
	if got.AllowedVideoRegex != videoPattern {
		t.Errorf("AllowedVideoRegex = %p, want %p", got.AllowedVideoRegex, videoPattern)
	}
	if got.LinkDensityModifier != 0.25 {
		t.Errorf("LinkDensityModifier = %v, want 0.25", got.LinkDensityModifier)
	}
	if got.Logger != logger {
		t.Errorf("Logger = %p, want %p", got.Logger, logger)
	}
	if !got.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestWithClassesToPreserveSnapshotsInput(t *testing.T) {
	classes := []string{"article"}
	option := WithClassesToPreserve(classes...)
	classes[0] = "changed"

	first := optionsToEngine([]Option{option})
	if want := []string{"article"}; !reflect.DeepEqual(first.ClassesToPreserve, want) {
		t.Fatalf("ClassesToPreserve = %v, want %v", first.ClassesToPreserve, want)
	}

	first.ClassesToPreserve[0] = "mutated"
	second := optionsToEngine([]Option{option})
	if want := []string{"article"}; !reflect.DeepEqual(second.ClassesToPreserve, want) {
		t.Fatalf("reused ClassesToPreserve = %v, want %v", second.ClassesToPreserve, want)
	}
}

func TestReaderableOptionsToEngineMappings(t *testing.T) {
	got := readerableOptionsToEngine([]ReaderableOption{
		WithMinScore(12.5),
		WithMinContentLength(75),
	})
	if got.MinScore != 12.5 {
		t.Errorf("MinScore = %v, want 12.5", got.MinScore)
	}
	if got.MinContentLength != 75 {
		t.Errorf("MinContentLength = %d, want 75", got.MinContentLength)
	}
}
