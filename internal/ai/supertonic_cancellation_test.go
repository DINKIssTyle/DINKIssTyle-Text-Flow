package ai

import (
	"context"
	"errors"
	"testing"
)

func TestSupertonicSynthesisHonorsPreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := &SupertonicEngine{}
	_, err := engine.SynthesizeContext(ctx, "text", "en", nil, 5, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SynthesizeContext() error = %v, want context.Canceled", err)
	}
}
