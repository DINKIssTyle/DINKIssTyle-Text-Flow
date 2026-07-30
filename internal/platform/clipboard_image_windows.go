//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"unsafe"
)

const (
	cfDIB              = 8
	bitmapInfoHeaderV1 = 40
	biRGB              = 0
)

// WriteClipboardPNG publishes the screenshot as a 32-bit device-independent
// bitmap. CF_DIB is understood by native Windows applications and avoids
// relying on WebView clipboard permissions.
func WriteClipboardPNG(data []byte) error {
	if len(data) == 0 {
		return errors.New("PNG data is empty")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode PNG for clipboard: %w", err)
	}
	bounds := decoded.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return errors.New("PNG dimensions are empty")
	}

	const bytesPerPixel = 4
	pixelBytes := width * height * bytesPerPixel
	dib := make([]byte, bitmapInfoHeaderV1+pixelBytes)
	binary.LittleEndian.PutUint32(dib[0:4], bitmapInfoHeaderV1)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(width))
	binary.LittleEndian.PutUint32(dib[8:12], uint32(height))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	binary.LittleEndian.PutUint32(dib[16:20], biRGB)
	binary.LittleEndian.PutUint32(dib[20:24], uint32(pixelBytes))

	for sourceY := 0; sourceY < height; sourceY++ {
		targetY := height - 1 - sourceY
		rowOffset := bitmapInfoHeaderV1 + targetY*width*bytesPerPixel
		for x := 0; x < width; x++ {
			red, green, blue, _ := decoded.At(bounds.Min.X+x, bounds.Min.Y+sourceY).RGBA()
			offset := rowOffset + x*bytesPerPixel
			dib[offset] = byte(blue >> 8)
			dib[offset+1] = byte(green >> 8)
			dib[offset+2] = byte(red >> 8)
			dib[offset+3] = 255
		}
	}

	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	return withOpenClipboard(func() error {
		emptied, _, callErr := procEmptyClipboard.Call()
		if emptied == 0 {
			return windowsClipboardCallError("empty clipboard", callErr)
		}

		handle, _, callErr := procGlobalAlloc.Call(gmemMoveable, uintptr(len(dib)))
		if handle == 0 {
			return windowsClipboardCallError("allocate clipboard image", callErr)
		}
		ownedByClipboard := false
		defer func() {
			if !ownedByClipboard {
				procGlobalFree.Call(handle)
			}
		}()

		target, _, callErr := procGlobalLock.Call(handle)
		if target == 0 {
			return windowsClipboardCallError("lock clipboard image", callErr)
		}
		copy(unsafe.Slice((*byte)(unsafe.Pointer(target)), len(dib)), dib)
		procGlobalUnlock.Call(handle)

		result, _, callErr := procSetClipboardData.Call(cfDIB, handle)
		if result == 0 {
			return windowsClipboardCallError("set clipboard image", callErr)
		}
		ownedByClipboard = true
		return nil
	})
}
