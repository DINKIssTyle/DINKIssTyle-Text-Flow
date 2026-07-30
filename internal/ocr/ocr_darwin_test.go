//go:build darwin

package ocr

import (
	"os"
	"testing"
)

func TestSupportedLanguagesReturnsVisionLanguages(t *testing.T) {
	languages, err := SupportedLanguages()
	if err != nil {
		t.Fatal(err)
	}
	if len(languages) == 0 {
		t.Fatal("SupportedLanguages returned no Apple Vision languages")
	}
}

func TestWarmUpIntegration(t *testing.T) {
	if os.Getenv("DKST_TEST_VISION_WARMUP") != "1" {
		t.Skip("set DKST_TEST_VISION_WARMUP=1 to exercise the native Vision model")
	}
	if err := WarmUp(LanguageAutomatic); err != nil {
		t.Fatal(err)
	}
}
