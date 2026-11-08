package clock

import (
	"errors"
	"strings"
	"testing"
)

// TestChooseUsesFirstResolvableSource: usage error (a resolver that succeeds)
// must win and later candidates must not be probed.
func TestChooseUsesFirstResolvableSource(t *testing.T) {
	var tried []string
	record := func(name string, err error) func() error {
		return func() error {
			tried = append(tried, name)
			return err
		}
	}
	cands := []candidate{
		{name: "first", find: record("first", errors.New("not found"))},
		{name: "second", find: record("second", nil)},
		{name: "third", find: record("third", nil)},
	}

	i, err := choose(cands)
	if err != nil {
		t.Fatalf("choose() error = %v, want nil", err)
	}
	if i != 1 {
		t.Fatalf("choose() = %d, want 1", i)
	}
	if want := []string{"first", "second"}; strings.Join(tried, ",") != strings.Join(want, ",") {
		t.Fatalf("probed %v, want %v", tried, want)
	}
}

// TestChooseReportsEveryMissingSource: when nothing resolves the error must
// name each attempt, which is what the error dialog shows the user.
func TestChooseReportsEveryMissingSource(t *testing.T) {
	cands := []candidate{
		{name: "kernelbase.dll!QueryInterruptTime", find: func() error { return errors.New("not found") }},
		{name: "kernel32.dll!GetTickCount64", find: func() error { return errors.New("not found") }},
	}

	i, err := choose(cands)
	if err == nil {
		t.Fatal("choose() error = nil, want failure")
	}
	if i != -1 {
		t.Fatalf("choose() index = %d, want -1", i)
	}
	for _, want := range []string{"kernelbase.dll!QueryInterruptTime", "kernel32.dll!GetTickCount64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
