package usererr

import (
	"strings"
	"testing"
)

func TestErrorf(t *testing.T) {
	err := Errorf("value %q is invalid", "bad")
	if err == nil || !strings.Contains(err.Error(), `value "bad" is invalid`) {
		t.Fatalf("Errorf() = %v", err)
	}
}
