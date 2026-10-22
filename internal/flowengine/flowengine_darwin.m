#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <stdlib.h>
#include <string.h>
#include <pthread.h>

extern void DKSTKeyboardInput(char *value, int backspace);

static CFMachPortRef dkstEventTap = nil;
static CFRunLoopSourceRef dkstRunLoopSource = nil;
static pthread_mutex_t dkstTapMutex = PTHREAD_MUTEX_INITIALIZER;
static const int64_t dkstSyntheticInputMarker = 0x444B5354;

static void DKSTReleaseKeyboardTap(void) {
    if (dkstEventTap == nil) {
        return;
    }
    CGEventTapEnable(dkstEventTap, false);
    if (dkstRunLoopSource != nil) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), dkstRunLoopSource, kCFRunLoopCommonModes);
        CFRelease(dkstRunLoopSource);
        dkstRunLoopSource = nil;
    }
    CFMachPortInvalidate(dkstEventTap);
    CFRelease(dkstEventTap);
    dkstEventTap = nil;
}

static CGEventRef DKSTKeyboardCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        pthread_mutex_lock(&dkstTapMutex);
        if (dkstEventTap != nil && CFMachPortIsValid(dkstEventTap)) {
            CGEventTapEnable(dkstEventTap, true);
        }
        pthread_mutex_unlock(&dkstTapMutex);
        return event;
    }
    if (type != kCGEventKeyDown) {
        return event;
    }
    if (CGEventGetIntegerValueField(event, kCGEventSourceUserData) == dkstSyntheticInputMarker) {
        return event;
    }

    CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    if (keyCode == 51) {
        DKSTKeyboardInput("", 1);
        return event;
    }
    if (keyCode == 36 || keyCode == 76) {
        DKSTKeyboardInput("\n", 0);
        return event;
    }
    if (keyCode == 48) {
        DKSTKeyboardInput("\t", 0);
        return event;
    }

    UniChar chars[8];
    UniCharCount length = 0;
    CGEventKeyboardGetUnicodeString(event, 8, &length, chars);
    if (length == 0) {
        return event;
    }

    NSString *text = [NSString stringWithCharacters:chars length:length];
    const char *utf8 = [text UTF8String];
    if (utf8 != NULL) {
        DKSTKeyboardInput((char *)utf8, 0);
    }
    return event;
}

int DKSTStartKeyboardTap(void) {
    pthread_mutex_lock(&dkstTapMutex);
    if (dkstEventTap != nil && CFMachPortIsValid(dkstEventTap)) {
        CGEventTapEnable(dkstEventTap, true);
        int enabled = CGEventTapIsEnabled(dkstEventTap);
        pthread_mutex_unlock(&dkstTapMutex);
        return enabled;
    }
    DKSTReleaseKeyboardTap();

    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown);
    dkstEventTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, 0, mask, DKSTKeyboardCallback, NULL);
    if (dkstEventTap == nil) {
        pthread_mutex_unlock(&dkstTapMutex);
        return 0;
    }

    dkstRunLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, dkstEventTap, 0);
    if (dkstRunLoopSource == nil) {
        DKSTReleaseKeyboardTap();
        pthread_mutex_unlock(&dkstTapMutex);
        return 0;
    }
    CFRunLoopAddSource(CFRunLoopGetMain(), dkstRunLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(dkstEventTap, true);
    int enabled = CGEventTapIsEnabled(dkstEventTap);
    pthread_mutex_unlock(&dkstTapMutex);
    return enabled;
}

void DKSTStopKeyboardTap(void) {
    pthread_mutex_lock(&dkstTapMutex);
    DKSTReleaseKeyboardTap();
    pthread_mutex_unlock(&dkstTapMutex);
}

int DKSTKeyboardTapRunning(void) {
    pthread_mutex_lock(&dkstTapMutex);
    int running = dkstEventTap != nil && CFMachPortIsValid(dkstEventTap) && CGEventTapIsEnabled(dkstEventTap);
    pthread_mutex_unlock(&dkstTapMutex);
    return running;
}

static void DKSTPostSyntheticEvent(CGEventRef event) {
    CGEventSetIntegerValueField(event, kCGEventSourceUserData, dkstSyntheticInputMarker);
    CGEventPost(kCGHIDEventTap, event);
}

void DKSTPostBackspaces(int count) {
    for (int i = 0; i < count; i++) {
        CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)51, true);
        CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)51, false);
        DKSTPostSyntheticEvent(down);
        DKSTPostSyntheticEvent(up);
        CFRelease(down);
        CFRelease(up);
        usleep(8000);
    }
}

void DKSTPostKey(int keyCode) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, false);
    DKSTPostSyntheticEvent(down);
    DKSTPostSyntheticEvent(up);
    CFRelease(down);
    CFRelease(up);
}

void DKSTTypeText(const char *ctext) {
    @autoreleasepool {
        NSString *text = [NSString stringWithUTF8String:ctext ?: ""];
        if (text == nil || [text length] == 0) {
            return;
        }

        [text enumerateSubstringsInRange:NSMakeRange(0, [text length])
                                  options:NSStringEnumerationByComposedCharacterSequences
                               usingBlock:^(NSString *substring, NSRange substringRange, NSRange enclosingRange, BOOL *stop) {
            NSUInteger length = [substring length];
            UniChar *chars = malloc(sizeof(UniChar) * length);
            if (chars == NULL) {
                return;
            }
            [substring getCharacters:chars range:NSMakeRange(0, length)];

            CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)0, true);
            CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)0, false);
            CGEventKeyboardSetUnicodeString(down, (UniCharCount)length, chars);
            CGEventKeyboardSetUnicodeString(up, (UniCharCount)length, chars);
            DKSTPostSyntheticEvent(down);
            DKSTPostSyntheticEvent(up);
            CFRelease(down);
            CFRelease(up);
            free(chars);
            usleep(1500);
        }];
    }
}

char *DKSTClipboardText(void) {
    __block char *result = strdup("");
    void (^readPasteboard)(void) = ^{
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSString *text = [pasteboard stringForType:NSPasteboardTypeString] ?: @"";
        free(result);
        result = strdup([text UTF8String]);
    };

    if ([NSThread isMainThread]) {
        readPasteboard();
    } else {
        dispatch_sync(dispatch_get_main_queue(), readPasteboard);
    }
    return result;
}

static void DKSTPostPasteShortcut(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, false);
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);
    DKSTPostSyntheticEvent(down);
    DKSTPostSyntheticEvent(up);
    CFRelease(down);
    CFRelease(up);
}

void DKSTPasteText(const char *ctext) {
    @autoreleasepool {
        NSString *text = [NSString stringWithUTF8String:ctext ?: ""];
        void (^writePasteboard)(void) = ^{
            NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
            [pasteboard clearContents];
            [pasteboard setString:text forType:NSPasteboardTypeString];
        };

        if ([NSThread isMainThread]) {
            writePasteboard();
        } else {
            dispatch_sync(dispatch_get_main_queue(), writePasteboard);
        }

        DKSTPostPasteShortcut();
    }
}

int DKSTKoreanInputActive(void) {
    // Text Input Sources APIs are only safe on the main thread in UI apps.
    __block int result = 0;
    void (^readInputSource)(void) = ^{
        @autoreleasepool {
            TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
            if (source == NULL) {
                return;
            }

            NSString *identifier = (__bridge NSString *)TISGetInputSourceProperty(source, kTISPropertyInputSourceID);
            NSString *name = (__bridge NSString *)TISGetInputSourceProperty(source, kTISPropertyLocalizedName);
            NSArray *languages = (__bridge NSArray *)TISGetInputSourceProperty(source, kTISPropertyInputSourceLanguages);
            BOOL isKorean = NO;
            if (identifier != nil) {
                isKorean = [identifier rangeOfString:@"Korean" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [identifier rangeOfString:@"Hangul" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [identifier rangeOfString:@"2Set" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [identifier rangeOfString:@"3Set" options:NSCaseInsensitiveSearch].location != NSNotFound;
            }
            if (!isKorean && name != nil) {
                isKorean = [name rangeOfString:@"Korean" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [name rangeOfString:@"Hangul" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [name rangeOfString:@"한글" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [name rangeOfString:@"두벌식" options:NSCaseInsensitiveSearch].location != NSNotFound ||
                           [name rangeOfString:@"세벌식" options:NSCaseInsensitiveSearch].location != NSNotFound;
            }
            if (!isKorean && languages != nil) {
                for (id language in languages) {
                    if (![language isKindOfClass:[NSString class]]) {
                        continue;
                    }
                    NSString *code = (NSString *)language;
                    if ([code caseInsensitiveCompare:@"ko"] == NSOrderedSame ||
                        [code rangeOfString:@"ko-" options:NSCaseInsensitiveSearch].location == 0 ||
                        [code rangeOfString:@"ko_" options:NSCaseInsensitiveSearch].location == 0) {
                        isKorean = YES;
                        break;
                    }
                }
            }

            CFRelease(source);
            result = isKorean ? 1 : 0;
        }
    };
    if ([NSThread isMainThread]) {
        readInputSource();
    } else {
        dispatch_sync(dispatch_get_main_queue(), readInputSource);
    }
    return result;
}
