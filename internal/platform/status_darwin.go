//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework ApplicationServices -framework AppKit -framework UniformTypeIdentifiers
#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int DKSTAccessibilityTrusted(void) {
    return AXIsProcessTrusted();
}

static int DKSTRequestAccessibilityPermission(void) {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

static char* DKSTFrontmostAppName(void) {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSString *value = [app localizedName] ?: @"";
    return strdup([value UTF8String]);
}

static char* DKSTFrontmostBundleID(void) {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSString *value = [app bundleIdentifier] ?: @"";
    return strdup([value UTF8String]);
}

static char* DKSTAppNameForPID(pid_t pid) {
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    NSString *value = [app localizedName] ?: @"";
    return strdup([value UTF8String]);
}

static char* DKSTBundleIDForPID(pid_t pid) {
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    NSString *value = [app bundleIdentifier] ?: @"";
    return strdup([value UTF8String]);
}

static int DKSTFrontmostPID(void) {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    return (int)[app processIdentifier];
}

static int DKSTWaitForFrontmostPID(pid_t pid, int timeoutMilliseconds) {
    int attempts = timeoutMilliseconds / 20;
    for (int i = 0; i < attempts; i++) {
        if (DKSTFrontmostPID() == pid) {
            return 1;
        }
        usleep(20000);
    }
    return DKSTFrontmostPID() == pid ? 1 : 0;
}

static int DKSTActivatePID(pid_t pid) {
    if (pid <= 0 || pid == getpid()) {
        return 0;
    }
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (app == nil) {
        return 0;
    }
    return [app activateWithOptions:NSApplicationActivateIgnoringOtherApps] ? 1 : 0;
}

static char* DKSTAppNameForBundlePath(const char *path) {
    if (path == NULL) {
        return strdup("");
    }
    NSString *bundlePath = [NSString stringWithUTF8String:path] ?: @"";
    NSBundle *bundle = [NSBundle bundleWithPath:bundlePath];
    NSString *value = [[bundle localizedInfoDictionary] objectForKey:@"CFBundleDisplayName"];
    if (value == nil || [value length] == 0) {
        value = [[bundle infoDictionary] objectForKey:@"CFBundleDisplayName"];
    }
    if (value == nil || [value length] == 0) {
        value = [[bundle localizedInfoDictionary] objectForKey:@"CFBundleName"];
    }
    if (value == nil || [value length] == 0) {
        value = [[bundle infoDictionary] objectForKey:@"CFBundleName"];
    }
    if (value == nil || [value length] == 0) {
        value = [[bundlePath lastPathComponent] stringByDeletingPathExtension];
    }
    return strdup([value UTF8String]);
}

static char* DKSTBundleIDForBundlePath(const char *path) {
    if (path == NULL) {
        return strdup("");
    }
    NSString *bundlePath = [NSString stringWithUTF8String:path] ?: @"";
    NSBundle *bundle = [NSBundle bundleWithPath:bundlePath];
    NSString *value = [bundle bundleIdentifier] ?: @"";
    return strdup([value UTF8String]);
}

static char* DKSTCopySelectedTextFromElement(AXUIElementRef element) {
    if (element == NULL) {
        return strdup("");
    }

    CFTypeRef selectedValue = NULL;
    AXError selectedError = AXUIElementCopyAttributeValue(
        element,
        kAXSelectedTextAttribute,
        &selectedValue
    );
    if (selectedError == kAXErrorSuccess && selectedValue != NULL && CFGetTypeID(selectedValue) == CFStringGetTypeID()) {
        NSString *selected = (__bridge NSString *)selectedValue;
        char *result = strdup([selected UTF8String]);
        CFRelease(selectedValue);
        return result;
    }
    if (selectedValue != NULL) {
        CFRelease(selectedValue);
    }

    CFTypeRef rangeValue = NULL;
    AXError rangeError = AXUIElementCopyAttributeValue(
        element,
        kAXSelectedTextRangeAttribute,
        &rangeValue
    );
    if (rangeError != kAXErrorSuccess || rangeValue == NULL) {
        return strdup("");
    }

    CFTypeRef rangedText = NULL;
    AXError textError = AXUIElementCopyParameterizedAttributeValue(
        element,
        kAXStringForRangeParameterizedAttribute,
        rangeValue,
        &rangedText
    );
    CFRelease(rangeValue);
    if (textError != kAXErrorSuccess || rangedText == NULL) {
        return strdup("");
    }

    char *result = strdup("");
    if (CFGetTypeID(rangedText) == CFStringGetTypeID()) {
        NSString *selected = (__bridge NSString *)rangedText;
        result = strdup([selected UTF8String]);
    }
    CFRelease(rangedText);
    return result;
}

static char* DKSTSelectedTextByAccessibility(void) {
    AXUIElementRef systemWide = AXUIElementCreateSystemWide();
    if (systemWide != NULL) {
        AXUIElementRef focused = NULL;
        AXError focusedError = AXUIElementCopyAttributeValue(
            systemWide,
            kAXFocusedUIElementAttribute,
            (CFTypeRef *)&focused
        );
        CFRelease(systemWide);
        if (focusedError == kAXErrorSuccess && focused != NULL) {
            char *selected = DKSTCopySelectedTextFromElement(focused);
            CFRelease(focused);
            if (strlen(selected) > 0) {
                return selected;
            }
            free(selected);
        }
    }

    NSRunningApplication *frontmost = [[NSWorkspace sharedWorkspace] frontmostApplication];
    pid_t pid = [frontmost processIdentifier];
    if (pid <= 0) {
        return strdup("");
    }

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app == NULL) {
        return strdup("");
    }
    AXUIElementRef focused = NULL;
    AXError focusedError = AXUIElementCopyAttributeValue(
        app,
        kAXFocusedUIElementAttribute,
        (CFTypeRef *)&focused
    );
    CFRelease(app);
    if (focusedError != kAXErrorSuccess || focused == NULL) {
        return strdup("");
    }

    char *result = DKSTCopySelectedTextFromElement(focused);
    CFRelease(focused);
    return result;
}

static char* DKSTSelectedTextByAccessibilityForPID(pid_t pid) {
    if (pid <= 0 || pid == getpid()) {
        return strdup("");
    }

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app == NULL) {
        return strdup("");
    }

    AXUIElementRef focused = NULL;
    AXError focusedError = AXUIElementCopyAttributeValue(
        app,
        kAXFocusedUIElementAttribute,
        (CFTypeRef *)&focused
    );
    CFRelease(app);
    if (focusedError != kAXErrorSuccess || focused == NULL) {
        return strdup("");
    }

    char *result = DKSTCopySelectedTextFromElement(focused);
    CFRelease(focused);
    return result;
}

static int DKSTIsElementEditable(AXUIElementRef element) {
    if (element == NULL) {
        return 0;
    }

    int isTextInputRole = 0;
    CFTypeRef roleValue = NULL;
    AXError roleError = AXUIElementCopyAttributeValue(
        element,
        kAXRoleAttribute,
        &roleValue
    );
    if (roleError == kAXErrorSuccess && roleValue != NULL) {
        if (CFGetTypeID(roleValue) == CFStringGetTypeID()) {
            NSString *role = (__bridge NSString *)roleValue;
            if ([role isEqualToString:(__bridge NSString *)kAXStaticTextRole]) {
                CFRelease(roleValue);
                return 0;
            }
            isTextInputRole =
                [role isEqualToString:(__bridge NSString *)kAXTextFieldRole] ||
                [role isEqualToString:(__bridge NSString *)kAXTextAreaRole];
        }
        CFRelease(roleValue);
    }

    Boolean isSettable = false;
    AXError settableError = AXUIElementIsAttributeSettable(element, kAXValueAttribute, &isSettable);
    if (settableError == kAXErrorSuccess && isSettable) {
        return 1;
    }

    settableError = AXUIElementIsAttributeSettable(element, kAXSelectedTextAttribute, &isSettable);
    if (settableError == kAXErrorSuccess && isSettable) {
        return 1;
    }

    settableError = AXUIElementIsAttributeSettable(element, kAXSelectedTextRangeAttribute, &isSettable);
    if (settableError == kAXErrorSuccess && isSettable) {
        return 1;
    }

    CFTypeRef editableValue = NULL;
    AXError editableError = AXUIElementCopyAttributeValue(
        element,
        CFSTR("AXEditable"),
        &editableValue
    );
    if (editableError == kAXErrorSuccess && editableValue != NULL) {
        int editable = CFGetTypeID(editableValue) == CFBooleanGetTypeID() &&
            CFBooleanGetValue((CFBooleanRef)editableValue);
        CFRelease(editableValue);
        return editable;
    }

    // Some rich-text and canvas editors accept keyboard input and paste but do
    // not expose their text value as settable through Accessibility.
    if (isTextInputRole) {
        return 1;
    }

    return 0;
}

static int DKSTIsElementOrAncestorEditable(AXUIElementRef element) {
    if (element == NULL) {
        return 0;
    }

    const int maxDepth = 8;
    AXUIElementRef current = (AXUIElementRef)CFRetain(element);
    for (int depth = 0; depth < maxDepth; depth++) {
        if (DKSTIsElementEditable(current)) {
            CFRelease(current);
            return 1;
        }

        CFTypeRef parentValue = NULL;
        AXError parentError = AXUIElementCopyAttributeValue(
            current,
            kAXParentAttribute,
            &parentValue
        );
        if (parentError != kAXErrorSuccess ||
            parentValue == NULL ||
            CFGetTypeID(parentValue) != AXUIElementGetTypeID()) {
            if (parentValue != NULL) {
                CFRelease(parentValue);
            }
            break;
        }

        AXUIElementRef parent = (AXUIElementRef)parentValue;
        if (CFEqual(current, parent)) {
            CFRelease(parent);
            break;
        }

        CFRelease(current);
        current = parent;
    }

    CFRelease(current);
    return 0;
}

static int DKSTIsFocusedElementEditableForPID(pid_t pid) {
    if (pid <= 0 || pid == getpid()) {
        AXUIElementRef systemWide = AXUIElementCreateSystemWide();
        if (systemWide != NULL) {
            AXUIElementRef focused = NULL;
            AXError focusedError = AXUIElementCopyAttributeValue(
                systemWide,
                kAXFocusedUIElementAttribute,
                (CFTypeRef *)&focused
            );
            CFRelease(systemWide);
            if (focusedError == kAXErrorSuccess && focused != NULL) {
                int editable = DKSTIsElementOrAncestorEditable(focused);
                CFRelease(focused);
                return editable;
            }
        }
        return 0;
    }

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app == NULL) {
        return 0;
    }

    AXUIElementRef focused = NULL;
    AXError focusedError = AXUIElementCopyAttributeValue(
        app,
        kAXFocusedUIElementAttribute,
        (CFTypeRef *)&focused
    );
    CFRelease(app);
    if (focusedError != kAXErrorSuccess || focused == NULL) {
        return 0;
    }

    int editable = DKSTIsElementOrAncestorEditable(focused);
    CFRelease(focused);
    return editable;
}


static NSArray<NSDictionary<NSPasteboardType, NSData *> *> *DKSTSnapshotPasteboard(NSPasteboard *pasteboard) {
    NSMutableArray<NSDictionary<NSPasteboardType, NSData *> *> *snapshot = [NSMutableArray array];
    for (NSPasteboardItem *item in [pasteboard pasteboardItems]) {
        NSMutableDictionary<NSPasteboardType, NSData *> *types = [NSMutableDictionary dictionary];
        for (NSPasteboardType type in [item types]) {
            NSData *data = [item dataForType:type];
            if (data != nil) {
                [types setObject:data forKey:type];
            }
        }
        if ([types count] > 0) {
            [snapshot addObject:types];
        }
    }
    return [snapshot copy];
}

static void DKSTRestorePasteboard(NSPasteboard *pasteboard, NSArray<NSDictionary<NSPasteboardType, NSData *> *> *snapshot) {
    [pasteboard clearContents];
    if (snapshot == nil || [snapshot count] == 0) {
        return;
    }

    NSMutableArray<NSPasteboardItem *> *items = [NSMutableArray arrayWithCapacity:[snapshot count]];
    for (NSDictionary<NSPasteboardType, NSData *> *types in snapshot) {
        NSPasteboardItem *item = [[NSPasteboardItem alloc] init];
        for (NSPasteboardType type in types) {
            [item setData:[types objectForKey:type] forType:type];
        }
        [items addObject:item];
        [item release];
    }
    [pasteboard writeObjects:items];
}

static void DKSTPostModifierKeyUpsToPID(pid_t pid) {
    CGKeyCode modifiers[] = {56, 60, 59, 62, 58, 61, 55, 54};
    size_t count = sizeof(modifiers) / sizeof(modifiers[0]);
    for (size_t i = 0; i < count; i++) {
        CGEventRef up = CGEventCreateKeyboardEvent(NULL, modifiers[i], false);
        if (up == NULL) {
            continue;
        }
        CGEventSetFlags(up, 0);
        if (pid > 0 && pid != getpid()) {
            CGEventPostToPid(pid, up);
        } else {
            CGEventPost(kCGSessionEventTap, up);
        }
        CFRelease(up);
    }
}

static void DKSTPostModifierKeyUpsToHID(void) {
    CGKeyCode modifiers[] = {56, 60, 59, 62, 58, 61, 55, 54};
    size_t count = sizeof(modifiers) / sizeof(modifiers[0]);
    for (size_t i = 0; i < count; i++) {
        CGEventRef up = CGEventCreateKeyboardEvent(NULL, modifiers[i], false);
        if (up == NULL) {
            continue;
        }
        CGEventSetFlags(up, 0);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(up);
    }
}

static void DKSTPostCommandShortcutToPID(pid_t pid, CGKeyCode keyCode) {
    CGEventRef commandDown = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, true);
    CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, keyCode, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(NULL, keyCode, false);
    CGEventRef commandUp = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, false);
    if (commandDown == NULL || keyDown == NULL || keyUp == NULL || commandUp == NULL) {
        if (commandDown != NULL) {
            CFRelease(commandDown);
        }
        if (keyDown != NULL) {
            CFRelease(keyDown);
        }
        if (keyUp != NULL) {
            CFRelease(keyUp);
        }
        if (commandUp != NULL) {
            CFRelease(commandUp);
        }
        return;
    }

    CGEventSetFlags(commandDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);
    CGEventSetFlags(commandUp, 0);
    if (pid > 0 && pid != getpid()) {
        CGEventPostToPid(pid, commandDown);
        usleep(10000);
        CGEventPostToPid(pid, keyDown);
        usleep(20000);
        CGEventPostToPid(pid, keyUp);
        usleep(10000);
        CGEventPostToPid(pid, commandUp);
    } else {
        CGEventPost(kCGSessionEventTap, commandDown);
        usleep(10000);
        CGEventPost(kCGSessionEventTap, keyDown);
        usleep(20000);
        CGEventPost(kCGSessionEventTap, keyUp);
        usleep(10000);
        CGEventPost(kCGSessionEventTap, commandUp);
    }
    CFRelease(commandDown);
    CFRelease(keyDown);
    CFRelease(keyUp);
    CFRelease(commandUp);
    DKSTPostModifierKeyUpsToPID(pid);
}

static void DKSTPostCommandShortcutToHID(CGKeyCode keyCode) {
    CGEventRef commandDown = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, true);
    CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, keyCode, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(NULL, keyCode, false);
    CGEventRef commandUp = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, false);
    if (commandDown == NULL || keyDown == NULL || keyUp == NULL || commandUp == NULL) {
        if (commandDown != NULL) {
            CFRelease(commandDown);
        }
        if (keyDown != NULL) {
            CFRelease(keyDown);
        }
        if (keyUp != NULL) {
            CFRelease(keyUp);
        }
        if (commandUp != NULL) {
            CFRelease(commandUp);
        }
        return;
    }

    CGEventSetFlags(commandDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);
    CGEventSetFlags(commandUp, 0);
    CGEventPost(kCGHIDEventTap, commandDown);
    usleep(10000);
    CGEventPost(kCGHIDEventTap, keyDown);
    usleep(20000);
    CGEventPost(kCGHIDEventTap, keyUp);
    usleep(10000);
    CGEventPost(kCGHIDEventTap, commandUp);
    CFRelease(commandDown);
    CFRelease(keyDown);
    CFRelease(keyUp);
    CFRelease(commandUp);
    DKSTPostModifierKeyUpsToHID();
}

static void DKSTPostCopyShortcutToPID(pid_t pid) {
    DKSTPostCommandShortcutToPID(pid, (CGKeyCode)8);
}

static void DKSTWaitForModifierKeysUp(void) {
    CGEventFlags modifierMask =
        kCGEventFlagMaskShift |
        kCGEventFlagMaskControl |
        kCGEventFlagMaskAlternate |
        kCGEventFlagMaskCommand;

    for (int i = 0; i < 30; i++) {
        CGEventFlags flags = CGEventSourceFlagsState(kCGEventSourceStateHIDSystemState);
        if ((flags & modifierMask) == 0) {
            return;
        }
        usleep(20000);
    }
}

static void DKSTPostCopyShortcutToHID(void) {
    DKSTPostCommandShortcutToHID((CGKeyCode)8);
}

static NSString *DKSTReadCopiedString(NSPasteboard *pasteboard, NSInteger oldChangeCount) {
    for (int i = 0; i < 18; i++) {
        usleep(30000);
        if ([pasteboard changeCount] != oldChangeCount) {
            return [pasteboard stringForType:NSPasteboardTypeString] ?: @"";
        }
    }
    return @"";
}

static void DKSTPostPasteShortcutToPID(pid_t pid) {
    DKSTPostCommandShortcutToPID(pid, (CGKeyCode)9);
}

static int DKSTReplaceTextByAccessibilityForPID(pid_t pid, const char *replacement) {
    if (pid <= 0 || pid == getpid() || replacement == NULL) {
        return 0;
    }

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app == NULL) {
        return 0;
    }

    AXUIElementRef focused = NULL;
    AXError focusedError = AXUIElementCopyAttributeValue(
        app,
        kAXFocusedUIElementAttribute,
        (CFTypeRef *)&focused
    );
    CFRelease(app);
    if (focusedError != kAXErrorSuccess || focused == NULL) {
        return 0;
    }

    CFTypeRef valueRef = NULL;
    AXError valueError = AXUIElementCopyAttributeValue(
        focused,
        kAXValueAttribute,
        &valueRef
    );
    if (valueError != kAXErrorSuccess || valueRef == NULL || CFGetTypeID(valueRef) != CFStringGetTypeID()) {
        if (valueRef != NULL) {
            CFRelease(valueRef);
        }
        CFRelease(focused);
        return 0;
    }

    NSString *current = (__bridge NSString *)valueRef;
    CFRange selectedRange = CFRangeMake([current length], 0);
    CFTypeRef rangeRef = NULL;
    AXError rangeError = AXUIElementCopyAttributeValue(
        focused,
        kAXSelectedTextRangeAttribute,
        &rangeRef
    );
    if (rangeError == kAXErrorSuccess && rangeRef != NULL && CFGetTypeID(rangeRef) == AXValueGetTypeID()) {
        AXValueGetValue((AXValueRef)rangeRef, kAXValueCFRangeType, &selectedRange);
    }
    if (rangeRef != NULL) {
        CFRelease(rangeRef);
    }

    NSUInteger length = [current length];
    NSUInteger location = selectedRange.location < 0 ? length : (NSUInteger)selectedRange.location;
    NSUInteger rangeLength = selectedRange.length < 0 ? 0 : (NSUInteger)selectedRange.length;
    if (location > length) {
        location = length;
    }
    if (location + rangeLength > length) {
        rangeLength = length - location;
    }

    NSString *insert = [NSString stringWithUTF8String:replacement] ?: @"";
    NSMutableString *next = [current mutableCopy];
    [next replaceCharactersInRange:NSMakeRange(location, rangeLength) withString:insert];

    AXError setError = AXUIElementSetAttributeValue(
        focused,
        kAXValueAttribute,
        (__bridge CFTypeRef)next
    );
    if (setError == kAXErrorSuccess) {
        CFRange nextRange = CFRangeMake((CFIndex)(location + [insert length]), 0);
        AXValueRef nextRangeValue = AXValueCreate(kAXValueCFRangeType, &nextRange);
        if (nextRangeValue != NULL) {
            AXUIElementSetAttributeValue(focused, kAXSelectedTextRangeAttribute, nextRangeValue);
            CFRelease(nextRangeValue);
        }
    }

    [next release];
    CFRelease(valueRef);
    CFRelease(focused);
    return setError == kAXErrorSuccess ? 1 : 0;
}

static int DKSTReplaceSelectedTextForPID(pid_t pid, const char *replacement, int preferPaste) {
    if (pid <= 0 || pid == getpid() || replacement == NULL) {
        return 0;
    }

    DKSTWaitForModifierKeysUp();
    DKSTPostModifierKeyUpsToPID(pid);
    if (preferPaste != 1 && DKSTReplaceTextByAccessibilityForPID(pid, replacement) == 1) {
        DKSTPostModifierKeyUpsToPID(pid);
        return 1;
    }

    __block int didPaste = 0;
    void (^pasteBlock)(void) = ^{
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSArray<NSDictionary<NSPasteboardType, NSData *> *> *oldItems = DKSTSnapshotPasteboard(pasteboard);

        NSString *text = [NSString stringWithUTF8String:replacement] ?: @"";
        [pasteboard clearContents];
        [pasteboard setString:text forType:NSPasteboardTypeString];
        NSInteger replacementChangeCount = [pasteboard changeCount];

        DKSTActivatePID(pid);
        DKSTWaitForFrontmostPID(pid, preferPaste == 1 ? 900 : 650);
        DKSTWaitForModifierKeysUp();
        DKSTPostModifierKeyUpsToPID(pid);
        usleep(80000);
        DKSTPostPasteShortcutToPID(pid);
        usleep(preferPaste == 1 ? 1000000 : 750000);
        DKSTPostModifierKeyUpsToPID(pid);

        // A slow target must read the replacement before the previous clipboard is restored.
        // If the user copied something meanwhile, preserve their newer clipboard instead.
        if ([pasteboard changeCount] == replacementChangeCount) {
            DKSTRestorePasteboard(pasteboard, oldItems);
        }
        [oldItems release];
        didPaste = 1;
    };

    if ([NSThread isMainThread]) {
        pasteBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), pasteBlock);
    }
    return didPaste;
}

static char* DKSTSelectedTextByCopyShortcutForPID(pid_t pid) {
    __block char *result = NULL;
    void (^copyBlock)(void) = ^{
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSArray<NSDictionary<NSPasteboardType, NSData *> *> *oldItems = DKSTSnapshotPasteboard(pasteboard);

        DKSTWaitForModifierKeysUp();
        [pasteboard clearContents];
        NSInteger oldChangeCount = [pasteboard changeCount];
        DKSTPostCopyShortcutToPID(pid);
        NSString *selected = DKSTReadCopiedString(pasteboard, oldChangeCount);

        if ([selected length] == 0) {
            [pasteboard clearContents];
            oldChangeCount = [pasteboard changeCount];
            DKSTPostCopyShortcutToHID();
            selected = DKSTReadCopiedString(pasteboard, oldChangeCount);
        }

        DKSTRestorePasteboard(pasteboard, oldItems);
        [oldItems release];

        result = strdup([selected UTF8String]);
    };

    if ([NSThread isMainThread]) {
        copyBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), copyBlock);
    }
    return result;
}

*/
import "C"

import (
	"strings"
	"unsafe"
)

func CurrentStatus() Status {
	name := cString(C.DKSTFrontmostAppName())
	bundleID := cString(C.DKSTFrontmostBundleID())
	trusted := C.DKSTAccessibilityTrusted() == 1

	message := "Accessibility permission is required before global expansion can run."
	if trusted {
		message = "Accessibility permission is ready. Flow Engine event tap is the next step."
	}

	return Status{
		AccessibilityTrusted: trusted,
		SecureInputActive:    false,
		ActiveAppName:        name,
		ActiveBundleID:       bundleID,
		FlowEngineRunning:    false,
		Message:              message,
	}
}

func requestAccessibilityPermission() bool {
	return C.DKSTRequestAccessibilityPermission() == 1
}

func selectedText() (string, error) {
	if selected := cString(C.DKSTSelectedTextByAccessibility()); strings.TrimSpace(selected) != "" {
		return selected, nil
	}
	return cString(C.DKSTSelectedTextByCopyShortcutForPID(0)), nil
}

func selectedTextFromProcess(processID int) (string, error) {
	if processID <= 0 {
		return selectedText()
	}
	pid := C.pid_t(processID)
	if selected := cString(C.DKSTSelectedTextByAccessibilityForPID(pid)); strings.TrimSpace(selected) != "" {
		return selected, nil
	}
	return cString(C.DKSTSelectedTextByCopyShortcutForPID(pid)), nil
}

func replaceSelectedTextInProcess(processID int, replacement string, preferPaste bool) error {
	if processID <= 0 {
		return nil
	}
	value := C.CString(replacement)
	defer C.free(unsafe.Pointer(value))
	preferPasteValue := C.int(0)
	if preferPaste {
		preferPasteValue = 1
	}
	if C.DKSTReplaceSelectedTextForPID(C.pid_t(processID), value, preferPasteValue) != 1 {
		return nil
	}
	_ = activateProcess(processID)
	return nil
}

func activateProcess(processID int) error {
	if processID <= 0 {
		return nil
	}
	C.DKSTActivatePID(C.pid_t(processID))
	return nil
}

func appInfoFromProcess(processID int) AppInfo {
	if processID <= 0 {
		return AppInfo{}
	}
	pid := C.pid_t(processID)
	return AppInfo{
		Name:     cString(C.DKSTAppNameForPID(pid)),
		BundleID: cString(C.DKSTBundleIDForPID(pid)),
	}
}

func appInfoFromBundlePath(path string) AppInfo {
	path = strings.TrimSpace(path)
	if path == "" {
		return AppInfo{}
	}
	value := C.CString(path)
	defer C.free(unsafe.Pointer(value))
	return AppInfo{
		Name:     cString(C.DKSTAppNameForBundlePath(value)),
		BundleID: cString(C.DKSTBundleIDForBundlePath(value)),
		Path:     path,
	}
}

func getFrontmostPID() int {
	return int(C.DKSTFrontmostPID())
}

func isFocusedElementEditableForProcess(processID int) bool {
	pid := C.pid_t(processID)
	return C.DKSTIsFocusedElementEditableForPID(pid) == 1
}

func cString(value *C.char) string {

	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
