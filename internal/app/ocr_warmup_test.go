package app

import (
	"runtime"
	"testing"
)

func TestOCRProcessingAllowsOnlyOnePipeline(t *testing.T) {
	app := &App{}
	if !app.tryBeginOCRProcessing() {
		t.Fatal("first OCR pipeline should start")
	}
	if app.tryBeginOCRProcessing() {
		t.Fatal("second OCR pipeline should be rejected while the first is active")
	}

	app.finishOCRProcessing()
	if !app.tryBeginOCRProcessing() {
		t.Fatal("OCR pipeline should start again after the previous one finishes")
	}
	app.finishOCRProcessing()
}

func TestShouldWarmUpOCR(t *testing.T) {
	disabled := DefaultGeneralSettings()
	enabledAuto := disabled
	enabledAuto.AppleVisionOCREnabled = true
	enabledKorean := enabledAuto
	enabledKorean.OCRRecognitionLanguage = "ko-KR"

	wantDarwin := runtime.GOOS == "darwin"
	tests := []struct {
		name     string
		previous GeneralSettings
		next     GeneralSettings
		want     bool
	}{
		{name: "enable OCR", previous: disabled, next: enabledAuto, want: wantDarwin},
		{name: "change language", previous: enabledAuto, next: enabledKorean, want: wantDarwin},
		{name: "unchanged", previous: enabledAuto, next: enabledAuto, want: false},
		{name: "disable OCR", previous: enabledAuto, next: disabled, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldWarmUpOCR(test.previous, test.next); got != test.want {
				t.Fatalf("shouldWarmUpOCR() = %v, want %v", got, test.want)
			}
		})
	}
}
