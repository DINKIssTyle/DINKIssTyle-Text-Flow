//go:build windows

package platform

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	clipboardOpenAttempts = 20
	clipboardRetryDelay   = 10 * time.Millisecond
)

var (
	clipboardMu = &sync.Mutex{}

	clipboardUser32            = syscall.NewLazyDLL("user32.dll")
	clipboardKernel32          = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard          = clipboardUser32.NewProc("OpenClipboard")
	procCloseClipboard         = clipboardUser32.NewProc("CloseClipboard")
	procEmptyClipboard         = clipboardUser32.NewProc("EmptyClipboard")
	procIsClipboardFormatReady = clipboardUser32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData       = clipboardUser32.NewProc("GetClipboardData")
	procSetClipboardData       = clipboardUser32.NewProc("SetClipboardData")
	procGlobalAlloc            = clipboardKernel32.NewProc("GlobalAlloc")
	procGlobalFree             = clipboardKernel32.NewProc("GlobalFree")
	procGlobalLock             = clipboardKernel32.NewProc("GlobalLock")
	procGlobalUnlock           = clipboardKernel32.NewProc("GlobalUnlock")
	procGlobalSize             = clipboardKernel32.NewProc("GlobalSize")
)

func ReadClipboardText() (string, error) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()

	var result string
	err := withOpenClipboard(func() error {
		available, _, _ := procIsClipboardFormatReady.Call(cfUnicodeText)
		if available == 0 {
			result = ""
			return nil
		}

		handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
		if handle == 0 {
			return windowsClipboardCallError("get Unicode clipboard data", callErr)
		}
		data, _, callErr := procGlobalLock.Call(handle)
		if data == 0 {
			return windowsClipboardCallError("lock clipboard data", callErr)
		}
		defer procGlobalUnlock.Call(handle)

		size, _, callErr := procGlobalSize.Call(handle)
		if size == 0 {
			if callErr != nil && callErr != syscall.Errno(0) {
				return windowsClipboardCallError("get clipboard data size", callErr)
			}
			result = ""
			return nil
		}

		units := unsafe.Slice((*uint16)(unsafe.Pointer(data)), int(size)/2)
		end := 0
		for end < len(units) && units[end] != 0 {
			end++
		}
		result = syscall.UTF16ToString(units[:end])
		return nil
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

func WriteClipboardText(text string) error {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()

	text = strings.ReplaceAll(text, "\x00", "")
	units, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}

	return withOpenClipboard(func() error {
		emptied, _, callErr := procEmptyClipboard.Call()
		if emptied == 0 {
			return windowsClipboardCallError("empty clipboard", callErr)
		}

		byteSize := uintptr(len(units) * 2)
		handle, _, callErr := procGlobalAlloc.Call(gmemMoveable, byteSize)
		if handle == 0 {
			return windowsClipboardCallError("allocate clipboard data", callErr)
		}
		ownedByClipboard := false
		defer func() {
			if !ownedByClipboard {
				procGlobalFree.Call(handle)
			}
		}()

		data, _, callErr := procGlobalLock.Call(handle)
		if data == 0 {
			return windowsClipboardCallError("lock allocated clipboard data", callErr)
		}
		copy(unsafe.Slice((*uint16)(unsafe.Pointer(data)), len(units)), units)
		procGlobalUnlock.Call(handle)

		result, _, callErr := procSetClipboardData.Call(cfUnicodeText, handle)
		if result == 0 {
			return windowsClipboardCallError("set Unicode clipboard data", callErr)
		}
		ownedByClipboard = true
		return nil
	})
}

func withOpenClipboard(action func() error) error {
	var lastErr error
	for attempt := 0; attempt < clipboardOpenAttempts; attempt++ {
		opened, _, callErr := procOpenClipboard.Call(0)
		if opened != 0 {
			defer procCloseClipboard.Call()
			return action()
		}
		lastErr = callErr
		time.Sleep(clipboardRetryDelay)
	}
	return windowsClipboardCallError("open clipboard", lastErr)
}

func windowsClipboardCallError(action string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return fmt.Errorf("%s failed", action)
	}
	return fmt.Errorf("%s: %w", action, err)
}
