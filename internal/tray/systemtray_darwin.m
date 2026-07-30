// go:build darwin

#import "systemtray_darwin.h"
#import <Cocoa/Cocoa.h>

extern void dkstSystemTrayMenuSelected(int itemID);

@interface DKSTSystemTrayController : NSObject
@property(nonatomic, retain) NSStatusItem *statusItem;
@property(nonatomic, retain) NSMenu *menu;
@property(nonatomic, retain) NSMenuItem *flowToggleItem;
@property(nonatomic, retain) NSMenuItem *ocrItem;
@property(nonatomic, retain) NSImage *activeImage;
@property(nonatomic, retain) NSImage *pausedImage;
- (void)statusItemClicked:(id)sender;
- (void)menuItemSelected:(id)sender;
- (void)updateFlowPaused:(BOOL)flowPaused
                 running:(BOOL)running
              ocrEnabled:(BOOL)ocrEnabled;
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

- (void)updateFlowPaused:(BOOL)flowPaused
                 running:(BOOL)running
              ocrEnabled:(BOOL)ocrEnabled {
  NSImage *image = running ? self.activeImage : self.pausedImage;
  if (image == nil)
    image = self.activeImage;
  self.statusItem.button.image = image;
  self.statusItem.button.toolTip =
      running ? @"DKST Text Flow" : @"DKST Text Flow — Paused";
  self.flowToggleItem.title = flowPaused ? @"Resume Flow" : @"Pause Flow";
  self.ocrItem.hidden = !ocrEnabled;
}

- (void)dealloc {
  [_pausedImage release];
  [_activeImage release];
  [_flowToggleItem release];
  [_ocrItem release];
  [_menu release];
  [_statusItem release];
  [super dealloc];
}

@end

static NSMenuItem *DKSTMenuItem(NSString *title, NSInteger tag,
                                DKSTSystemTrayController *controller) {
  NSMenuItem *item =
      [[[NSMenuItem alloc] initWithTitle:title
                                  action:@selector(menuItemSelected:)
                           keyEquivalent:@""] autorelease];
  item.target = controller;
  item.tag = tag;
  return item;
}

static NSImage *DKSTTemplateImage(const unsigned char *iconBytes,
                                  int iconLength) {
  if (iconBytes == NULL || iconLength <= 0)
    return nil;
  NSData *data = [NSData dataWithBytes:iconBytes length:(NSUInteger)iconLength];
  NSImage *image = [[[NSImage alloc] initWithData:data] autorelease];
  image.template = YES;
  image.size = NSMakeSize(26.0, 26.0);
  return image;
}

void *dkstSystemTrayCreate(const unsigned char *activeIconBytes,
                           int activeIconLength,
                           const unsigned char *pausedIconBytes,
                           int pausedIconLength) {
  DKSTSystemTrayController *controller =
      [[DKSTSystemTrayController alloc] init];
  controller.statusItem = [[NSStatusBar systemStatusBar]
      statusItemWithLength:NSSquareStatusItemLength];
  controller.activeImage = DKSTTemplateImage(activeIconBytes, activeIconLength);
  controller.pausedImage = DKSTTemplateImage(pausedIconBytes, pausedIconLength);

  NSButton *button = controller.statusItem.button;
  button.target = controller;
  button.action = @selector(statusItemClicked:);
  [button sendActionOn:NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown];
  button.image = controller.activeImage;

  controller.menu =
      [[[NSMenu alloc] initWithTitle:@"DKST Text Flow"] autorelease];
  [controller.menu addItem:DKSTMenuItem(@"Ask AI", 1, controller)];
  controller.ocrItem = DKSTMenuItem(@"OCR", 5, controller);
  controller.ocrItem.hidden = YES;
  [controller.menu addItem:controller.ocrItem];
  [controller.menu addItem:DKSTMenuItem(@"Main Window", 2, controller)];
  [controller.menu addItem:[NSMenuItem separatorItem]];
  controller.flowToggleItem = DKSTMenuItem(@"Pause Flow", 3, controller);
  [controller.menu addItem:controller.flowToggleItem];
  [controller.menu addItem:[NSMenuItem separatorItem]];
  [controller.menu addItem:DKSTMenuItem(@"Quit", 4, controller)];

  return controller;
}

void dkstSystemTrayUpdateState(void *tray, int flowPaused, int running,
                               int ocrEnabled) {
  DKSTSystemTrayController *controller = (DKSTSystemTrayController *)tray;
  if (controller == nil)
    return;
  dispatch_async(dispatch_get_main_queue(), ^{
    [controller updateFlowPaused:(flowPaused == 1)
                         running:(running == 1)
                      ocrEnabled:(ocrEnabled == 1)];
  });
}

void dkstSystemTrayDestroy(void *tray) {
  DKSTSystemTrayController *controller = (DKSTSystemTrayController *)tray;
  if (controller == nil)
    return;
  [[NSStatusBar systemStatusBar] removeStatusItem:controller.statusItem];
  [controller release];
}
