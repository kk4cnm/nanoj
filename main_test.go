package main

import "testing"

func TestJSONWorkPath(t *testing.T) {
	cases := map[string]string{
		"config.yaml":          "config.json",
		"config.yml":           "config.json",
		"dir/app.toml":         "dir/app.json",
		"noext":                "noext.json",
		"dir.d/noext":          "dir.d/noext.json", // dot in a directory, not the name
		".hidden":              ".hidden.json",     // leading dot is not an extension
		"a/b/settings.old.yml": "a/b/settings.old.json",
	}
	for in, want := range cases {
		if got := jsonWorkPath(in); got != want {
			t.Errorf("jsonWorkPath(%q) = %q, want %q", in, got, want)
		}
	}
}
