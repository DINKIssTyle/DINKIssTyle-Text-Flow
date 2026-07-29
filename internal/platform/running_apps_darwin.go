//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

static NSString *DKSTApplicationIconDataURL(NSImage *icon) {
    if (icon == nil) {
        return @"";
    }

    const NSInteger iconSize = 32;
    NSBitmapImageRep *bitmap = [[[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL
                      pixelsWide:iconSize
                      pixelsHigh:iconSize
                   bitsPerSample:8
                 samplesPerPixel:4
                        hasAlpha:YES
                        isPlanar:NO
                  colorSpaceName:NSDeviceRGBColorSpace
                     bytesPerRow:0
                    bitsPerPixel:0] autorelease];
    if (bitmap == nil) {
        return @"";
    }

    [NSGraphicsContext saveGraphicsState];
    NSGraphicsContext *context =
        [NSGraphicsContext graphicsContextWithBitmapImageRep:bitmap];
    [NSGraphicsContext setCurrentContext:context];
    [icon drawInRect:NSMakeRect(0, 0, iconSize, iconSize)
            fromRect:NSZeroRect
           operation:NSCompositingOperationSourceOver
            fraction:1.0
      respectFlipped:YES
               hints:nil];
    [NSGraphicsContext restoreGraphicsState];

    NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG
                                        properties:@{}];
    if (png == nil || [png length] == 0) {
        return @"";
    }
    return [NSString stringWithFormat:@"data:image/png;base64,%@",
        [png base64EncodedStringWithOptions:0]];
}

static char *DKSTRunningApplicationsJSON(void) {
    @autoreleasepool {
        NSMutableArray *result = [NSMutableArray array];
        NSMutableSet *seenBundleIDs = [NSMutableSet set];
        NSArray<NSRunningApplication *> *applications =
            [[NSWorkspace sharedWorkspace] runningApplications];

        for (NSRunningApplication *application in applications) {
            NSString *bundleID = [application bundleIdentifier] ?: @"";
            NSString *name = [application localizedName] ?: @"";
            if ([bundleID length] == 0 || [name length] == 0 ||
                [seenBundleIDs containsObject:bundleID]) {
                continue;
            }
            [seenBundleIDs addObject:bundleID];

            NSString *path = [[[application bundleURL] path] copy];
            NSString *iconDataURL = DKSTApplicationIconDataURL([application icon]);
            [result addObject:@{
                @"name": name,
                @"bundleId": bundleID,
                @"path": path ?: @"",
                @"iconDataUrl": iconDataURL ?: @""
            }];
            [path release];
        }

        NSError *error = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:result
                                                       options:0
                                                         error:&error];
        if (data == nil || error != nil) {
            return strdup("[]");
        }
        NSString *json = [[[NSString alloc] initWithData:data
                                                encoding:NSUTF8StringEncoding]
            autorelease];
        return strdup([json UTF8String]);
    }
}
*/
import "C"

import (
	"encoding/json"
	"sort"
	"strings"
)

func listRunningApps() []AppInfo {
	value := cString(C.DKSTRunningApplicationsJSON())
	var apps []AppInfo
	if err := json.Unmarshal([]byte(value), &apps); err != nil {
		return []AppInfo{}
	}
	sort.SliceStable(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps
}
