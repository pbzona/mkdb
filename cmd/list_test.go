package cmd

import "testing"

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{version: "18", want: "18"},
		{version: "16.3", want: "16"},
		{version: "8.0.35-oracle", want: "8"},
		{version: "v7.2", want: "7"},
		{version: "latest", want: "latest"},
		{version: "", want: "-"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := majorVersion(tt.version); got != tt.want {
				t.Errorf("majorVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
