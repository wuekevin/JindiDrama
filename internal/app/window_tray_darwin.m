#import <AppKit/AppKit.h>
#import <stdint.h>
#import "_cgo_export.h"

@interface JindiStatusMenuTarget : NSObject
@property(nonatomic) uintptr_t handle;
@property(nonatomic, assign) NSWindow *window;
- (void)restoreWindow:(id)sender;
- (void)quitApplication:(id)sender;
@end

static NSStatusItem *JindiStatusItem;
static JindiStatusMenuTarget *JindiStatusTarget;

@implementation JindiStatusMenuTarget
- (void)removeStatusItem {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    uintptr_t handle = self.handle;
    [[NSStatusBar systemStatusBar] removeStatusItem:JindiStatusItem];
    [JindiStatusItem release];
    JindiStatusItem = nil;
    [JindiStatusTarget autorelease];
    JindiStatusTarget = nil;
    JindiStatusItemAction(handle, false);
}

- (void)restoreWindow:(id)sender {
    NSWindow *window = self.window;
    [self removeStatusItem];
    if (@available(macOS 14.0, *)) {
        [NSApp activate];
    } else {
        [NSApp activateIgnoringOtherApps:YES];
    }
    [window makeKeyAndOrderFront:nil];
}

- (void)windowBecameKey:(NSNotification *)notification {
    [self removeStatusItem];
}

- (void)quitApplication:(id)sender {
    JindiStatusItemAction(self.handle, true);
}
@end

static bool hideWindowToStatusItem(uintptr_t handle) {
    NSWindow *window = NSApp.mainWindow ?: NSApp.keyWindow;
    if (JindiStatusItem != nil || window == nil || !window.visible || window.attachedSheet != nil || NSApp.modalWindow != nil) {
        return false;
    }
    NSStatusItem *item = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
    if (item.button == nil) {
        if (item != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:item];
            [item release];
        }
        return false;
    }
    NSImage *image = [NSImage imageWithSystemSymbolName:@"play.rectangle" accessibilityDescription:@"金帝短剧"];
    if (image != nil) {
        image.template = YES;
        item.button.image = image;
    } else {
        item.button.title = @"金帝短剧";
    }
    item.button.toolTip = @"金帝短剧";
    [item.button setAccessibilityLabel:@"金帝短剧"];

    JindiStatusMenuTarget *target = [[JindiStatusMenuTarget alloc] init];
    target.handle = handle;
    target.window = window;
    NSMenu *menu = [[NSMenu alloc] init];
    NSMenuItem *restore = [menu addItemWithTitle:@"显示窗口" action:@selector(restoreWindow:) keyEquivalent:@""];
    restore.target = target;
    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *quit = [menu addItemWithTitle:@"退出金帝短剧" action:@selector(quitApplication:) keyEquivalent:@""];
    quit.target = target;
    item.menu = menu;
    [menu release];
    JindiStatusItem = item;
    JindiStatusTarget = target;
    [[NSNotificationCenter defaultCenter] addObserver:target selector:@selector(windowBecameKey:)
                                                 name:NSWindowDidBecomeKeyNotification object:window];
    [window orderOut:nil];
    return true;
}

bool JindiHideWindowToStatusItem(uintptr_t handle) {
    if ([NSThread isMainThread]) {
        return hideWindowToStatusItem(handle);
    }
    __block bool success;
    dispatch_sync(dispatch_get_main_queue(), ^{
        success = hideWindowToStatusItem(handle);
    });
    return success;
}
