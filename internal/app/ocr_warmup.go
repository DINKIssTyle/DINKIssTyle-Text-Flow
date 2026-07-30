package app

import (
	"runtime"
	"time"

	"dkst-text-flow/internal/ocr"
)

const ocrWarmUpDelay = 500 * time.Millisecond

func shouldWarmUpOCR(previous, next GeneralSettings) bool {
	if runtime.GOOS != "darwin" || !next.AppleVisionOCREnabled {
		return false
	}
	return !previous.AppleVisionOCREnabled ||
		previous.OCRRecognitionLanguage != next.OCRRecognitionLanguage
}

// scheduleOCRWarmUp debounces startup/settings changes. The native OCR package
// serializes the warm-up with real recognition requests.
func (a *App) scheduleOCRWarmUp(language string) {
	if runtime.GOOS != "darwin" {
		return
	}

	a.ocrWarmupMu.Lock()
	a.ocrWarmupGeneration++
	generation := a.ocrWarmupGeneration
	ctx := a.ctx
	a.ocrWarmupMu.Unlock()

	go func() {
		timer := time.NewTimer(ocrWarmUpDelay)
		defer timer.Stop()
		if ctx != nil {
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
		} else {
			<-timer.C
		}

		a.ocrWarmupMu.Lock()
		isCurrent := generation == a.ocrWarmupGeneration
		a.ocrWarmupMu.Unlock()
		if !isCurrent {
			return
		}
		if err := ocr.WarmUp(language); err != nil {
			println("Apple Vision OCR warm-up failed:", err.Error())
		}
	}()
}
