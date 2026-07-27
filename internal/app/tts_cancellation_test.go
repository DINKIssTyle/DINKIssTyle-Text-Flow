package app

import (
	"context"
	"testing"
)

func TestStopSpeakingCancelsActiveSynthesis(t *testing.T) {
	application := New(nil, nil)
	ctx, _ := application.beginSpeaking()

	application.StopSpeaking()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("speech context error = %v, want context.Canceled", ctx.Err())
		}
	default:
		t.Fatal("StopSpeaking did not cancel the active speech context")
	}
}

func TestStartingSpeechCancelsPreviousSynthesis(t *testing.T) {
	application := New(nil, nil)
	firstContext, _ := application.beginSpeaking()
	secondContext, _ := application.beginSpeaking()
	defer application.StopSpeaking()

	select {
	case <-firstContext.Done():
	default:
		t.Fatal("starting new speech did not cancel the previous synthesis")
	}
	if err := secondContext.Err(); err != nil {
		t.Fatalf("new speech context was cancelled unexpectedly: %v", err)
	}
}
