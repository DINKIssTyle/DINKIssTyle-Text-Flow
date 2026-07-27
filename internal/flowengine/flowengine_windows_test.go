//go:build windows

package flowengine

import (
	"reflect"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestWinInputLayout(t *testing.T) {
	expectedSize := uintptr(28)
	expectedDataOffset := uintptr(4)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		expectedSize = 40
		expectedDataOffset = 8
	}

	if size := unsafe.Sizeof(winInput{}); size != expectedSize {
		t.Fatalf("winInput size = %d, want %d", size, expectedSize)
	}
	if offset := unsafe.Offsetof(winInput{}.Data); offset != expectedDataOffset {
		t.Fatalf("winInput data offset = %d, want %d", offset, expectedDataOffset)
	}
}

func TestUnicodeInputsForRune(t *testing.T) {
	for _, value := range []rune{'A', '한', '😀'} {
		t.Run(string(value), func(t *testing.T) {
			units := utf16.Encode([]rune{value})
			inputs := unicodeInputsForRune(value)
			if len(inputs) != len(units)*2 {
				t.Fatalf("input count = %d, want %d", len(inputs), len(units)*2)
			}

			for index, unit := range units {
				down := keyboardInputFrom(inputs[index*2])
				up := keyboardInputFrom(inputs[index*2+1])
				if down.WVk != 0 || down.WScan != unit || down.DwFlags != keyeventfUnicode {
					t.Fatalf("keydown %d = %+v", index, down)
				}
				if up.WVk != 0 || up.WScan != unit || up.DwFlags != keyeventfUnicode|keyeventfKeyup {
					t.Fatalf("keyup %d = %+v", index, up)
				}
				if down.DwExtraInfo != syntheticInputMarker || up.DwExtraInfo != syntheticInputMarker {
					t.Fatalf("input %d is missing the synthetic marker", index)
				}
			}
		})
	}
}

func TestInsertSnippetTextUsesUnicodeInput(t *testing.T) {
	var sent []rune
	var pasted []string

	insertSnippetTextWith("A한😀", false, func(value rune) bool {
		sent = append(sent, value)
		return true
	}, func(text string) {
		pasted = append(pasted, text)
	})

	if !reflect.DeepEqual(sent, []rune("A한😀")) {
		t.Fatalf("sent = %q", string(sent))
	}
	if len(pasted) != 0 {
		t.Fatalf("unexpected paste: %q", pasted)
	}
}

func TestInsertSnippetTextFallsBackFromFailedRune(t *testing.T) {
	var sent []rune
	var pasted []string

	insertSnippetTextWith("가나다", false, func(value rune) bool {
		sent = append(sent, value)
		return value != '나'
	}, func(text string) {
		pasted = append(pasted, text)
	})

	if !reflect.DeepEqual(sent, []rune("가나")) {
		t.Fatalf("sent = %q", string(sent))
	}
	if !reflect.DeepEqual(pasted, []string{"나다"}) {
		t.Fatalf("pasted = %q", pasted)
	}
}

func TestInsertSnippetTextHonorsPastePreference(t *testing.T) {
	var sent []rune
	var pasted []string

	insertSnippetTextWith("hello", true, func(value rune) bool {
		sent = append(sent, value)
		return true
	}, func(text string) {
		pasted = append(pasted, text)
	})

	if len(sent) != 0 {
		t.Fatalf("unexpected Unicode input: %q", string(sent))
	}
	if !reflect.DeepEqual(pasted, []string{"hello"}) {
		t.Fatalf("pasted = %q", pasted)
	}
}

func TestInsertSnippetTextRemovesNUL(t *testing.T) {
	var sent []rune

	insertSnippetTextWith("a\x00b", false, func(value rune) bool {
		sent = append(sent, value)
		return true
	}, func(string) {})

	if !reflect.DeepEqual(sent, []rune("ab")) {
		t.Fatalf("sent = %q", string(sent))
	}
}

func keyboardInputFrom(input winInput) winKeyboardInput {
	return *(*winKeyboardInput)(unsafe.Pointer(&input.Data))
}
