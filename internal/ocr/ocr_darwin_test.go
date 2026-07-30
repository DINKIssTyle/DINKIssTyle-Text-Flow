//go:build darwin

package ocr

import "testing"

func TestSupportedLanguagesReturnsVisionLanguages(t *testing.T) {
	languages, err := SupportedLanguages()
	if err != nil {
		t.Fatal(err)
	}
	if len(languages) == 0 {
		t.Fatal("SupportedLanguages returned no Apple Vision languages")
	}
}
