package types

import "testing"

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		wantError bool
	}{
		{"running", StatusRunning, false},
		{"up", StatusRunning, false},
		{"  UP  ", StatusRunning, false},
		{"down", StatusStopped, false},
		{"stopped", StatusStopped, false},
		{"expired", StatusExpired, false},
		{"removed", StatusRemoved, false},
		{"bogus", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeStatus(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("NormalizeStatus(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeStatus(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeDBType(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		wantError bool
	}{
		{"pg", DBTypePostgres, false},
		{"postgresql", DBTypePostgres, false},
		{"mariadb", DBTypeMySQL, false},
		{"redis", DBTypeRedis, false},
		{"mongo", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeDBType(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("NormalizeDBType(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeDBType(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeDBType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
