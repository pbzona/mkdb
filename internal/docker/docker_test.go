package docker

import (
	"errors"
	"testing"
)

func TestWrapContainerErrorClassifiesPortConflict(t *testing.T) {
	err := wrapContainerError("start", "5432", errors.New("driver failed: port is already allocated"))
	if !IsPortConflict(err) {
		t.Fatalf("IsPortConflict() = false for %v", err)
	}

	var conflict *PortConflictError
	if !errors.As(err, &conflict) {
		t.Fatal("wrapped error is not a PortConflictError")
	}
	if conflict.Port != "5432" {
		t.Errorf("conflict port = %q, want 5432", conflict.Port)
	}
}

func TestWrapContainerErrorLeavesOtherErrorsUnclassified(t *testing.T) {
	err := wrapContainerError("start", "5432", errors.New("image failed to start"))
	if IsPortConflict(err) {
		t.Fatalf("IsPortConflict() = true for %v", err)
	}
}
