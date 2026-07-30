#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
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

        VNRecognizeTextRequest *request = DKSTTextRequest();
        NSString *languageCode = language != NULL
            ? [NSString stringWithUTF8String:language]
            : @"auto";
        if (languageCode.length == 0) {
            languageCode = @"auto";
        }

        NSError *languageError = nil;
        NSArray<NSString *> *supported = [request supportedRecognitionLanguagesAndReturnError:&languageError];
        if (supported == nil) {
            CGImageRelease(image);
            DKSTSetVisionError(errorMessage, languageError.localizedDescription);
            return NULL;
        }
        if ([languageCode isEqualToString:@"auto"]) {
            if (@available(macOS 13.0, *)) {
                request.automaticallyDetectsLanguage = YES;
            }
        } else if ([supported containsObject:languageCode]) {
            request.recognitionLanguages = @[languageCode];
        } else {
            CGImageRelease(image);
            DKSTSetVisionError(
                errorMessage,
                [NSString stringWithFormat:@"Apple Vision OCR does not support %@ on this Mac.", languageCode]
            );
            return NULL;
        }

        VNImageRequestHandler *handler = [[[VNImageRequestHandler alloc] initWithCGImage:image options:@{}] autorelease];
        NSError *performError = nil;
        BOOL performed = [handler performRequests:@[request] error:&performError];
        CGImageRelease(image);
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
}
