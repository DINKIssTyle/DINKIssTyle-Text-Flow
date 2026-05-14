#import <Cocoa/Cocoa.h>

extern void DKSTStatusItemOpen(void);

@interface DKSTStatusItemTarget : NSObject
- (void)open:(id)sender;
@end

@implementation DKSTStatusItemTarget
- (void)open:(id)sender {
    DKSTStatusItemOpen();
}
@end

static NSStatusItem *dkstStatusItem = nil;
static DKSTStatusItemTarget *dkstStatusTarget = nil;

void DKSTPrepareAccessoryApp(void) {
}

static void DKSTInstallStatusItemOnMain(void) {
    if (dkstStatusItem != nil) {
        [dkstStatusItem setVisible:YES];
        return;
    }

    dkstStatusTarget = [[DKSTStatusItemTarget alloc] init];
    dkstStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    [dkstStatusItem retain];
    [dkstStatusItem setVisible:YES];

    NSStatusBarButton *button = [dkstStatusItem button];
    NSString *iconPath = [[NSBundle mainBundle] pathForResource:@"menu_icon" ofType:@"pdf"];
    NSImage *icon = iconPath ? [[NSImage alloc] initWithContentsOfFile:iconPath] : nil;
    if (icon != nil) {
        [icon setTemplate:YES];
        [button setImage:icon];
        [button setTitle:@""];
    } else {
        [button setTitle:@"TextFlow"];
    }
    [button setToolTip:@"DKST Text Flow"];
    [button setTarget:dkstStatusTarget];
    [button setAction:@selector(open:)];
    [button setEnabled:YES];
}

void DKSTInstallStatusItem(void) {
    if ([NSThread isMainThread]) {
        DKSTInstallStatusItemOnMain();
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            DKSTInstallStatusItemOnMain();
        });
    }
}
