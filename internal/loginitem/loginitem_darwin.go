//go:build darwin

package loginitem

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework ServiceManagement -framework Foundation
#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <stdlib.h>

static int DKSTLaunchAtLoginEnabled(void) {
    if (@available(macOS 13.0, *)) {
        return [SMAppService mainAppService].status == SMAppServiceStatusEnabled ? 1 : 0;
    }
    return 0;
}

static char* DKSTSetLaunchAtLoginEnabled(int enabled) {
    if (!@available(macOS 13.0, *)) {
        return enabled ? strdup("Launch at login requires macOS 13 or newer") : NULL;
    }

    SMAppService *service = [SMAppService mainAppService];
    BOOL isEnabled = service.status == SMAppServiceStatusEnabled;
    if ((enabled && isEnabled) || (!enabled && !isEnabled)) {
        return NULL;
    }

    NSError *error = nil;
    BOOL ok = enabled
        ? [service registerAndReturnError:&error]
        : [service unregisterAndReturnError:&error];
    if (ok) {
        return NULL;
    }

    NSString *message = error.localizedDescription ?: @"Could not update launch at login";
    return strdup([message UTF8String]);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func Enabled() bool {
	return C.DKSTLaunchAtLoginEnabled() == 1
}

func SetEnabled(enabled bool) error {
	value := C.int(0)
	if enabled {
		value = 1
	}

	message := C.DKSTSetLaunchAtLoginEnabled(value)
	if message == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(message))
	return errors.New(C.GoString(message))
}
