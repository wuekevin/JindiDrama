//go:build darwin && cgo && !bindings

#import <AVFoundation/AVFoundation.h>
#import <ImageIO/ImageIO.h>
#include <stdlib.h>
#include <string.h>

void *jindiDemoFrame(const char *path, double seconds, size_t *length) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        AVURLAsset *asset = [AVURLAsset URLAssetWithURL:url options:nil];
        AVAssetImageGenerator *generator = [AVAssetImageGenerator assetImageGeneratorWithAsset:asset];
        generator.appliesPreferredTrackTransform = YES;
        generator.requestedTimeToleranceBefore = kCMTimeZero;
        generator.requestedTimeToleranceAfter = kCMTimeZero;

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        CGImageRef image = [generator copyCGImageAtTime:CMTimeMakeWithSeconds(seconds, 600)
                                            actualTime:NULL error:NULL];
#pragma clang diagnostic pop
        if (image == NULL) return NULL;

        NSMutableData *png = [NSMutableData data];
        CGImageDestinationRef destination = CGImageDestinationCreateWithData((CFMutableDataRef)png,
                                                                            CFSTR("public.png"), 1, NULL);
        void *result = NULL;
        if (destination != NULL) {
            CGImageDestinationAddImage(destination, image, NULL);
            if (CGImageDestinationFinalize(destination) && png.length <= (8u << 20)) {
                result = malloc(png.length);
                if (result != NULL) {
                    memcpy(result, png.bytes, png.length);
                    *length = png.length;
                }
            }
            CFRelease(destination);
        }
        CGImageRelease(image);
        return result;
    }
}
