package util

import "testing"

func TestParseRepo(t *testing.T) {
	owner, repo, err := ParseRepo("JohnTitor/example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "JohnTitor" || repo != "example" {
		t.Fatalf("unexpected result: owner=%s repo=%s", owner, repo)
	}
}

func TestParseRepoErrors(t *testing.T) {
	cases := []string{"", "foo", "foo/", "/bar", "foo/bar/baz"}
	for _, c := range cases {
		if _, _, err := ParseRepo(c); err == nil {
			t.Fatalf("expected error for input %q", c)
		}
	}
}
