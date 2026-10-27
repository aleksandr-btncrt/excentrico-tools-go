package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogger_EventsAndProgressAreSeparate(t *testing.T) {
	var events, progress bytes.Buffer
	l := NewWithWriters("test", &events, &progress)

	l.StartOperation("process_single_film").WithContext("k", "v").Complete("done")
	l.Progress("[%d/%d] %q → post #%d, %d images", 1, 2, "Film", 10, 3)

	if got := progress.String(); got != "[1/2] \"Film\" → post #10, 3 images\n" {
		t.Errorf("progress = %q", got)
	}
	if strings.Contains(events.String(), "post #10") {
		t.Error("progress line leaked into the event stream")
	}
	var ev map[string]any
	if err := json.Unmarshal(events.Bytes(), &ev); err != nil {
		t.Fatalf("event is not one JSON line: %v (%q)", err, events.String())
	}
	if ev["operation"] != "process_single_film" || ev["outcome"] != "success" {
		t.Errorf("unexpected event %v", ev)
	}
}

func TestLogger_DefaultSampleRateKeepsEverything(t *testing.T) {
	var events bytes.Buffer
	l := NewWithWriters("test", &events, &bytes.Buffer{})
	for i := 0; i < 50; i++ {
		l.StartOperation("op").Complete("ok")
	}
	if n := strings.Count(events.String(), "\n"); n != 50 {
		t.Errorf("logged %d of 50 successful events", n)
	}
}

func TestOperationTracker_ImportantBypassesSampling(t *testing.T) {
	var events bytes.Buffer
	l := NewWithWriters("test", &events, &bytes.Buffer{})
	l.SetSampleRate(0)

	l.StartOperation("sampled_away").Complete("ok")
	l.StartOperation("create_wordpress_post").Important().Complete("ok")

	out := events.String()
	if strings.Contains(out, "sampled_away") {
		t.Error("sample rate 0 should drop ordinary successful events")
	}
	if !strings.Contains(out, "create_wordpress_post") {
		t.Error("Important() event was sampled away")
	}
	if strings.Contains(out, "important") {
		t.Error("internal important flag must not be serialised")
	}
}
