package app

import "testing"

func TestLastTTSAudioSnapshotIsIndependent(t *testing.T) {
	application := New(nil, nil)
	source := []byte("RIFF test data")
	application.setLastTTSAudio(source)

	source[0] = 'X'
	first := application.lastTTSAudioSnapshot()
	first[1] = 'Y'
	second := application.lastTTSAudioSnapshot()

	if string(second) != "RIFF test data" {
		t.Fatalf("stored TTS audio was mutated: %q", second)
	}
}

func TestWAVFilePathAddsNormalizedExtension(t *testing.T) {
	tests := map[string]string{
		"/tmp/speech":     "/tmp/speech.wav",
		"/tmp/speech.WAV": "/tmp/speech.wav",
		"/tmp/speech.mp3": "/tmp/speech.mp3.wav",
	}
	for source, want := range tests {
		if got := wavFilePath(source); got != want {
			t.Fatalf("wavFilePath(%q) = %q, want %q", source, got, want)
		}
	}
}
