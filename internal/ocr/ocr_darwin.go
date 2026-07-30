//go:build darwin

package ocr

/*
#cgo darwin CFLAGS: -x objective-c -fblocks
#cgo darwin LDFLAGS: -framework Foundation -framework Vision -framework ImageIO -framework CoreGraphics -framework CoreText

#include <stdlib.h>

char *DKSTVisionRecognizeText(
	const unsigned char *data,
	size_t length,
	const char *language,
	char **errorMessage
);
int DKSTVisionWarmUp(const char *language, char **errorMessage);
char *DKSTVisionSupportedLanguages(char **errorMessage);
*/
import "C"

import (
	"errors"
	"strings"
	"sync"
	"unsafe"
)

var recognitionMu sync.Mutex

func RecognizePNG(data []byte, language string) (Result, error) {
	if len(data) == 0 {
		return Result{}, errors.New("OCR image data is empty")
	}

	language = normalizeLanguage(language)
	cLanguage := C.CString(language)
	defer C.free(unsafe.Pointer(cLanguage))

	recognitionMu.Lock()
	defer recognitionMu.Unlock()

	var cError *C.char
	cText := C.DKSTVisionRecognizeText(
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		cLanguage,
		&cError,
	)
	if cError != nil {
		defer C.free(unsafe.Pointer(cError))
		return Result{}, errors.New(C.GoString(cError))
	}
	if cText == nil {
		return Result{}, errors.New("Apple Vision OCR returned no result")
	}
	defer C.free(unsafe.Pointer(cText))
	return Result{Text: strings.TrimSpace(C.GoString(cText))}, nil
}

// WarmUp performs a real recognition request on a small in-memory text image.
// It shares the recognition lock with user requests so Vision is never asked to
// load or run more than one OCR model at a time.
func WarmUp(language string) error {
	cLanguage := C.CString(normalizeLanguage(language))
	defer C.free(unsafe.Pointer(cLanguage))

	recognitionMu.Lock()
	defer recognitionMu.Unlock()

	var cError *C.char
	if C.DKSTVisionWarmUp(cLanguage, &cError) == 0 {
		if cError != nil {
			defer C.free(unsafe.Pointer(cError))
			return errors.New(C.GoString(cError))
		}
		return errors.New("Apple Vision OCR warm-up failed")
	}
	if cError != nil {
		defer C.free(unsafe.Pointer(cError))
		return errors.New(C.GoString(cError))
	}
	return nil
}

func SupportedLanguages() ([]string, error) {
	var cError *C.char
	cLanguages := C.DKSTVisionSupportedLanguages(&cError)
	if cError != nil {
		defer C.free(unsafe.Pointer(cError))
		return nil, errors.New(C.GoString(cError))
	}
	if cLanguages == nil {
		return []string{}, nil
	}
	defer C.free(unsafe.Pointer(cLanguages))

	var languages []string
	for _, language := range strings.Split(C.GoString(cLanguages), "\n") {
		language = strings.TrimSpace(language)
		if language != "" {
			languages = append(languages, language)
		}
	}
	return languages, nil
}

func normalizeLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return LanguageAutomatic
	}
	return language
}
