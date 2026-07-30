#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
#import <CoreText/CoreText.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

static char *DKSTCopyUTF8String(NSString *value) {
    const char *utf8 = [value UTF8String];
    return strdup(utf8 != NULL ? utf8 : "");
}

static void DKSTSetVisionError(char **errorMessage, NSString *message) {
    if (errorMessage == NULL) {
        return;
    }
    *errorMessage = DKSTCopyUTF8String(message ?: @"Apple Vision OCR failed.");
}

static VNRecognizeTextRequest *DKSTTextRequest(void) {
    VNRecognizeTextRequest *request = [[[VNRecognizeTextRequest alloc] init] autorelease];
    request.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
    request.usesLanguageCorrection = YES;
    if (@available(macOS 13.0, *)) {
        request.revision = VNRecognizeTextRequestRevision3;
    }
    return request;
}

static BOOL DKSTConfigureTextRequest(
    VNRecognizeTextRequest *request,
    const char *language,
    char **errorMessage
) {
    NSString *languageCode = language != NULL
        ? [NSString stringWithUTF8String:language]
        : @"auto";
    if (languageCode.length == 0) {
        languageCode = @"auto";
    }

    NSError *languageError = nil;
    NSArray<NSString *> *supported = [request supportedRecognitionLanguagesAndReturnError:&languageError];
    if (supported == nil) {
        DKSTSetVisionError(errorMessage, languageError.localizedDescription);
        return NO;
    }
    if ([languageCode isEqualToString:@"auto"]) {
        if (@available(macOS 13.0, *)) {
            request.automaticallyDetectsLanguage = YES;
        }
        return YES;
    }
    if ([supported containsObject:languageCode]) {
        request.recognitionLanguages = @[languageCode];
        return YES;
    }

    DKSTSetVisionError(
        errorMessage,
        [NSString stringWithFormat:@"Apple Vision OCR does not support %@ on this Mac.", languageCode]
    );
    return NO;
}

static char *DKSTRecognizeCGImage(
    CGImageRef image,
    const char *language,
    char **errorMessage
) {
    if (image == NULL) {
        DKSTSetVisionError(errorMessage, @"Apple Vision could not create an image for OCR.");
        return NULL;
    }

    VNRecognizeTextRequest *request = DKSTTextRequest();
    if (!DKSTConfigureTextRequest(request, language, errorMessage)) {
        return NULL;
    }

    VNImageRequestHandler *handler = [[[VNImageRequestHandler alloc] initWithCGImage:image options:@{}] autorelease];
    NSError *performError = nil;
    BOOL performed = [handler performRequests:@[request] error:&performError];
    if (!performed) {
        DKSTSetVisionError(errorMessage, performError.localizedDescription);
        return NULL;
    }

    NSArray<VNRecognizedTextObservation *> *observations = request.results ?: @[];
    NSArray<VNRecognizedTextObservation *> *ordered = [observations sortedArrayUsingComparator:
        ^NSComparisonResult(VNRecognizedTextObservation *left, VNRecognizedTextObservation *right) {
            CGFloat leftTop = CGRectGetMaxY(left.boundingBox);
            CGFloat rightTop = CGRectGetMaxY(right.boundingBox);
            if (fabs(leftTop - rightTop) > 0.015) {
                return leftTop > rightTop ? NSOrderedAscending : NSOrderedDescending;
            }
            CGFloat leftX = CGRectGetMinX(left.boundingBox);
            CGFloat rightX = CGRectGetMinX(right.boundingBox);
            if (leftX < rightX) {
                return NSOrderedAscending;
            }
            if (leftX > rightX) {
                return NSOrderedDescending;
            }
            return NSOrderedSame;
        }
    ];

    NSMutableArray<NSString *> *lines = [NSMutableArray arrayWithCapacity:ordered.count];
    for (VNRecognizedTextObservation *observation in ordered) {
        VNRecognizedText *candidate = [[observation topCandidates:1] firstObject];
        if (candidate.string.length > 0) {
            [lines addObject:candidate.string];
        }
    }
    return DKSTCopyUTF8String([lines componentsJoinedByString:@"\n"]);
}

static CGImageRef DKSTCreateWarmUpImage(void) {
    const size_t width = 640;
    const size_t height = 96;
    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGContextRef context = CGBitmapContextCreate(
        NULL,
        width,
        height,
        8,
        width * 4,
        colorSpace,
        (CGBitmapInfo)kCGImageAlphaPremultipliedLast
    );
    CGColorSpaceRelease(colorSpace);
    if (context == NULL) {
        return NULL;
    }

    CGContextSetRGBFillColor(context, 1.0, 1.0, 1.0, 1.0);
    CGContextFillRect(context, CGRectMake(0, 0, width, height));

    CTFontRef font = CTFontCreateWithName(CFSTR("Helvetica"), 28.0, NULL);
    CGColorRef textColor = CGColorCreateGenericRGB(0.05, 0.05, 0.05, 1.0);
    const void *keys[] = { kCTFontAttributeName, kCTForegroundColorAttributeName };
    const void *values[] = { font, textColor };
    CFDictionaryRef attributes = CFDictionaryCreate(
        kCFAllocatorDefault,
        keys,
        values,
        2,
        &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks
    );
    CFAttributedStringRef attributed = CFAttributedStringCreate(
        kCFAllocatorDefault,
        CFSTR("Text Flow OCR 123  한국어  中文  日本語"),
        attributes
    );
    CTLineRef line = CTLineCreateWithAttributedString(attributed);
    CGContextSetTextPosition(context, 16.0, 34.0);
    CTLineDraw(line, context);

    CGImageRef image = CGBitmapContextCreateImage(context);
    CFRelease(line);
    CFRelease(attributed);
    CFRelease(attributes);
    CGColorRelease(textColor);
    CFRelease(font);
    CGContextRelease(context);
    return image;
}

char *DKSTVisionSupportedLanguages(char **errorMessage) {
    @autoreleasepool {
        if (errorMessage != NULL) {
            *errorMessage = NULL;
        }
        VNRecognizeTextRequest *request = DKSTTextRequest();
        NSError *error = nil;
        NSArray<NSString *> *languages = [request supportedRecognitionLanguagesAndReturnError:&error];
        if (languages == nil) {
            DKSTSetVisionError(errorMessage, error.localizedDescription);
            return NULL;
        }
        NSArray<NSString *> *sorted = [languages sortedArrayUsingSelector:@selector(localizedCaseInsensitiveCompare:)];
        return DKSTCopyUTF8String([sorted componentsJoinedByString:@"\n"]);
    }
}

char *DKSTVisionRecognizeText(
    const unsigned char *bytes,
    size_t length,
    const char *language,
    char **errorMessage
) {
    @autoreleasepool {
        if (errorMessage != NULL) {
            *errorMessage = NULL;
        }
        if (bytes == NULL || length == 0) {
            DKSTSetVisionError(errorMessage, @"OCR image data is empty.");
            return NULL;
        }

        NSData *data = [NSData dataWithBytes:bytes length:length];
        CGImageSourceRef source = CGImageSourceCreateWithData((__bridge CFDataRef)data, NULL);
        if (source == NULL) {
            DKSTSetVisionError(errorMessage, @"Apple Vision could not decode the captured image.");
            return NULL;
        }
        CGImageRef image = CGImageSourceCreateImageAtIndex(source, 0, NULL);
        CFRelease(source);
        if (image == NULL) {
            DKSTSetVisionError(errorMessage, @"Apple Vision could not create an image for OCR.");
            return NULL;
        }

        char *recognized = DKSTRecognizeCGImage(image, language, errorMessage);
        CGImageRelease(image);
        return recognized;
    }
}

int DKSTVisionWarmUp(const char *language, char **errorMessage) {
    @autoreleasepool {
        if (errorMessage != NULL) {
            *errorMessage = NULL;
        }
        CGImageRef image = DKSTCreateWarmUpImage();
        if (image == NULL) {
            DKSTSetVisionError(errorMessage, @"Apple Vision OCR could not create its warm-up image.");
            return 0;
        }
        char *recognized = DKSTRecognizeCGImage(image, language, errorMessage);
        CGImageRelease(image);
        if (recognized == NULL) {
            return 0;
        }
        free(recognized);
        return 1;
    }
}
