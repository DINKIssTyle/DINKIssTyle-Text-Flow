#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <stdlib.h>
#include <string.h>

extern void DKSTKeyboardInput(char *value, int backspace);

static CFMachPortRef dkstEventTap = nil;
static CFRunLoopSourceRef dkstRunLoopSource = nil;

static CGEventRef DKSTKeyboardCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type != kCGEventKeyDown) {
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
    if (dkstEventTap != nil) {
        return 1;
    }

    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown);
    dkstEventTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, 0, mask, DKSTKeyboardCallback, NULL);
    if (dkstEventTap == nil) {
        return 0;
    }

    dkstRunLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, dkstEventTap, 0);
    CFRunLoopAddSource(CFRunLoopGetMain(), dkstRunLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(dkstEventTap, true);
    return 1;
}

void DKSTStopKeyboardTap(void) {
    if (dkstEventTap == nil) {
        return;
    }
    CGEventTapEnable(dkstEventTap, false);
    if (dkstRunLoopSource != nil) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), dkstRunLoopSource, kCFRunLoopCommonModes);
        CFRelease(dkstRunLoopSource);
        dkstRunLoopSource = nil;
    }
    CFRelease(dkstEventTap);
    dkstEventTap = nil;
}

void DKSTPostBackspaces(int count) {
    for (int i = 0; i < count; i++) {
        CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)51, true);
        CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)51, false);
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(down);
        CFRelease(up);
        usleep(8000);
    }
}

void DKSTPostKey(int keyCode) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, false);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

void DKSTTypeText(const char *ctext) {
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
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(down);
        CFRelease(up);
        free(chars);
        usleep(1500);
    }];
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
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

void DKSTPasteText(const char *ctext) {
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

int DKSTKoreanInputActive(void) {
    TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
    if (source == NULL) {
        return 0;
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
    return isKorean ? 1 : 0;
}
