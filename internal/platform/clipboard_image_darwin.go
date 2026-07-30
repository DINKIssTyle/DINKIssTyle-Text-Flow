//go:build darwin

package platform

/*
#cgo CFLAGS: -mmacosx-version-min=12.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static bool DKSTWritePNGToClipboard(const unsigned char *bytes, int length) {
	@autoreleasepool {
		if (bytes == NULL || length <= 0) {
			return false;
		}
		NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
		NSImage *image = [[[NSImage alloc] initWithData:data] autorelease];
		if (image == nil) {
			return false;
		}
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard clearContents];
		return [pasteboard writeObjects:@[image]];
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func WriteClipboardPNG(data []byte) error {
	if len(data) == 0 {
		return errors.New("PNG data is empty")
	}
	if !bool(C.DKSTWritePNGToClipboard(
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
	)) {
		return errors.New("macOS pasteboard rejected the image")
	}
	return nil
}
