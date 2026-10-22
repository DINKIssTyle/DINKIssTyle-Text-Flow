#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <assert.h>
#include <string.h>

// Exercise the native callback and resource lifecycle without registering a
// global keyboard hook, posting keys, or modifying the user's clipboard.
static bool tapEnabled = false;
static bool failTapCreation = false;
static bool failSourceCreation = false;
static int tapCreations = 0;
static int enableCalls = 0;
static int inputCalls = 0;
static int postedEvents = 0;
static int inputSourceReads = 0;

static TISInputSourceRef MockCurrentInputSource(void) {
    assert([NSThread isMainThread]);
    inputSourceReads++;
    return NULL;
}

static void MockMachCallback(CFMachPortRef port, void *message, CFIndex size, void *info) {}

static CFMachPortRef MockTapCreate(CGEventTapLocation location, CGEventTapPlacement placement,
                                  CGEventTapOptions options, CGEventMask mask,
                                  CGEventTapCallBack callback, void *info) {
    tapCreations++;
    if (failTapCreation) return NULL;
    return CFMachPortCreate(kCFAllocatorDefault, MockMachCallback, NULL, NULL);
}

static void MockTapEnable(CFMachPortRef tap, bool enabled) {
    enableCalls++;
    tapEnabled = enabled;
}

static bool MockTapIsEnabled(CFMachPortRef tap) { return tapEnabled; }

static CFRunLoopSourceRef MockCreateSource(CFAllocatorRef allocator, CFMachPortRef port, CFIndex order) {
    if (failSourceCreation) return NULL;
    return CFMachPortCreateRunLoopSource(allocator, port, order);
}

static void MockPostEvent(CGEventTapLocation location, CGEventRef event);

#define CGEventTapCreate MockTapCreate
#define CGEventTapEnable MockTapEnable
#define CGEventTapIsEnabled MockTapIsEnabled
#define CFMachPortCreateRunLoopSource MockCreateSource
#define CGEventPost MockPostEvent
#define TISCopyCurrentKeyboardInputSource MockCurrentInputSource
#include "../flowengine_darwin.m"

void DKSTKeyboardInput(char *value, int backspace) {
    inputCalls++;
    assert(backspace == 0);
    assert(strcmp(value, "a") == 0);
}

static void MockPostEvent(CGEventTapLocation location, CGEventRef event) {
    postedEvents++;
    assert(CGEventGetIntegerValueField(event, kCGEventSourceUserData) == dkstSyntheticInputMarker);
    int previousCalls = inputCalls;
    DKSTKeyboardCallback(NULL, CGEventGetType(event), event, NULL);
    assert(inputCalls == previousCalls);
}

int main(void) {
    @autoreleasepool {
        assert(!DKSTKeyboardTapRunning());
        failTapCreation = true;
        assert(!DKSTStartKeyboardTap());
        assert(dkstEventTap == NULL);
        failTapCreation = false;

        failSourceCreation = true;
        assert(!DKSTStartKeyboardTap());
        assert(dkstEventTap == NULL && dkstRunLoopSource == NULL);
        failSourceCreation = false;

        assert(DKSTStartKeyboardTap());
        assert(DKSTKeyboardTapRunning());
        int previousCreations = tapCreations;
        tapEnabled = false;
        assert(!DKSTKeyboardTapRunning());
        assert(DKSTStartKeyboardTap());
        assert(tapCreations == previousCreations);

        tapEnabled = false;
        DKSTKeyboardCallback(NULL, kCGEventTapDisabledByTimeout, NULL, NULL);
        assert(DKSTKeyboardTapRunning());
        tapEnabled = false;
        DKSTKeyboardCallback(NULL, kCGEventTapDisabledByUserInput, NULL, NULL);
        assert(DKSTKeyboardTapRunning());

        CFMachPortInvalidate(dkstEventTap);
        assert(!DKSTKeyboardTapRunning());
        assert(DKSTStartKeyboardTap());
        assert(tapCreations == previousCreations + 1);

        CGEventRef key = CGEventCreateKeyboardEvent(NULL, 0, true);
        UniChar character = 'a';
        CGEventKeyboardSetUnicodeString(key, 1, &character);
        DKSTKeyboardCallback(NULL, kCGEventKeyDown, key, NULL);
        assert(inputCalls == 1);
        CGEventSetIntegerValueField(key, kCGEventSourceUserData, dkstSyntheticInputMarker);
        DKSTKeyboardCallback(NULL, kCGEventKeyDown, key, NULL);
        assert(inputCalls == 1);
        CFRelease(key);

        DKSTPostBackspaces(2);
        DKSTPostKey(48);
        DKSTTypeText("a😀");
        DKSTPostPasteShortcut();
        assert(postedEvents == 12);
        assert(inputCalls == 1);

        __block bool inputSourceReadDone = false;
        dispatch_async(dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0), ^{
            assert(DKSTKoreanInputActive() == 0);
            dispatch_async(dispatch_get_main_queue(), ^{ inputSourceReadDone = true; });
        });
        CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 5;
        while (!inputSourceReadDone && CFAbsoluteTimeGetCurrent() < deadline) {
            CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.01, false);
        }
        assert(inputSourceReadDone && inputSourceReads == 1);

        DKSTStopKeyboardTap();
        assert(!DKSTKeyboardTapRunning());
        assert(dkstEventTap == NULL && dkstRunLoopSource == NULL);
        int previousEnableCalls = enableCalls;
        DKSTKeyboardCallback(NULL, kCGEventTapDisabledByTimeout, NULL, NULL);
        assert(enableCalls == previousEnableCalls);
        DKSTStopKeyboardTap();
    }
    return 0;
}
