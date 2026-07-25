//go:build darwin

package windowing

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static void resizeWindowFromTop(void *windowPointer, int width, int height) {
	if (windowPointer == NULL) {
		return;
	}

	NSWindow *window = (NSWindow *)windowPointer;
	NSRect currentFrame = [window frame];
	NSRect targetFrame = NSMakeRect(
		currentFrame.origin.x,
		NSMaxY(currentFrame) - height,
		width,
		height
	);
	[window setFrame:targetFrame display:YES animate:YES];
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

func ResizeFromTop(window application.Window, width, height int) {
	C.resizeWindowFromTop(window.NativeWindow(), C.int(width), C.int(height))
}
