//go:build darwin

#import <Cocoa/Cocoa.h>
#import "systemtray_darwin.h"

extern void dkstSystemTrayMenuSelected(int itemID);

@interface DKSTSystemTrayController : NSObject
@property(nonatomic, retain) NSStatusItem *statusItem;
@property(nonatomic, retain) NSMenu *menu;
- (void)statusItemClicked:(id)sender;
- (void)menuItemSelected:(id)sender;
@end

@implementation DKSTSystemTrayController

- (void)statusItemClicked:(id)sender {
    NSEvent *event = NSApp.currentEvent;
    if (event.type == NSEventTypeLeftMouseDown) {
        [self.statusItem popUpStatusItemMenu:self.menu];
    }
}

- (void)menuItemSelected:(NSMenuItem *)sender {
    dkstSystemTrayMenuSelected((int)sender.tag);
}

- (void)dealloc {
    [_menu release];
    [_statusItem release];
    [super dealloc];
}

@end

static NSMenuItem *DKSTMenuItem(NSString *title, NSInteger tag, DKSTSystemTrayController *controller) {
    NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title
                                                  action:@selector(menuItemSelected:)
                                           keyEquivalent:@""] autorelease];
    item.target = controller;
    item.tag = tag;
    return item;
}

void *dkstSystemTrayCreate(const unsigned char *iconBytes, int iconLength) {
    DKSTSystemTrayController *controller = [[DKSTSystemTrayController alloc] init];
    controller.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];

    NSButton *button = controller.statusItem.button;
    button.target = controller;
    button.action = @selector(statusItemClicked:);
    [button sendActionOn:NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown];

    if (iconBytes != NULL && iconLength > 0) {
        NSData *data = [NSData dataWithBytes:iconBytes length:(NSUInteger)iconLength];
        NSImage *image = [[[NSImage alloc] initWithData:data] autorelease];
        image.template = YES;
        image.size = NSMakeSize(24.0, 24.0);
        button.image = image;
    }

    controller.menu = [[[NSMenu alloc] initWithTitle:@"DKST Text Flow"] autorelease];
    [controller.menu addItem:DKSTMenuItem(@"Ask AI", 1, controller)];
    [controller.menu addItem:DKSTMenuItem(@"Main Window", 2, controller)];
    [controller.menu addItem:[NSMenuItem separatorItem]];
    [controller.menu addItem:DKSTMenuItem(@"Quit", 3, controller)];

    return controller;
}

void dkstSystemTrayDestroy(void *tray) {
    DKSTSystemTrayController *controller = (DKSTSystemTrayController *)tray;
    if (controller == nil) return;
    [[NSStatusBar systemStatusBar] removeStatusItem:controller.statusItem];
    [controller release];
}
