package cmd

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is ok", nil, exitOK},
		{"untagged error is general", errors.New("boom"), exitGeneral},
		{"tagged not found", withExitCode(exitNotFound, errors.New("missing")), exitNotFound},
		{"tagged timeout", withExitCode(exitTimeout, errors.New("slow")), exitTimeout},
		{"wrapped tagged error", fmt.Errorf("context: %w", withExitCode(exitDockerUnavailable, errors.New("down"))), exitDockerUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.err); got != tt.want {
				t.Errorf("exitCodeFor() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithExitCodeNil(t *testing.T) {
	if withExitCode(exitTimeout, nil) != nil {
		t.Error("withExitCode(code, nil) should return nil")
	}
}
