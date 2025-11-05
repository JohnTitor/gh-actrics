package cmd

import (
	"testing"
	"time"
)

func TestParseLastDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"":      30 * 24 * time.Hour,
		"1.5m":  90 * time.Second,
		"5h":    5 * time.Hour,
		"7d":    7 * 24 * time.Hour,
		"2w":    14 * 24 * time.Hour,
		"3mo":   90 * 24 * time.Hour,
	}

	for input, want := range cases {
		got, err := parseLastDuration(input)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("unexpected duration for %q: %s", input, got)
		}
	}

	if _, err := parseLastDuration("bad"); err == nil {
		t.Fatalf("expected error for invalid input")
	}
}

func TestResolveTimeRange(t *testing.T) {
	now := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	from, to, err := resolveTimeRange(now, "", "", "2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if to != now {
		t.Fatalf("expected to=now")
	}
	if want := now.Add(-48 * time.Hour); from != want {
		t.Fatalf("expected from=%s got %s", want, from)
	}

	from, to, err = resolveTimeRange(now, "2025-03-30T00:00:00Z", "2025-03-31T00:00:00Z", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !to.Equal(time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected to: %s", to)
	}
	if !from.Equal(time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected from: %s", from)
	}

	if _, _, err := resolveTimeRange(now, "2025-04-02T00:00:00Z", "2025-04-01T00:00:00Z", ""); err == nil {
		t.Fatalf("expected error when from >= to")
	}
}
