//go:build darwin

package statusitem

/*
#cgo darwin CFLAGS: -x objective-c -fblocks
#cgo darwin LDFLAGS: -framework Cocoa

void DKSTInstallStatusItem(void);
void DKSTPrepareAccessoryApp(void);
*/
import "C"

import (
	"context"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var state struct {
	sync.Mutex
	ctx context.Context
}

func PrepareAccessoryApp() {
	C.DKSTPrepareAccessoryApp()
}

func Install(ctx context.Context) {
	state.Lock()
	state.ctx = ctx
	state.Unlock()
	C.DKSTInstallStatusItem()
}

//export DKSTStatusItemOpen
func DKSTStatusItemOpen() {
	state.Lock()
	ctx := state.ctx
	state.Unlock()
	if ctx == nil {
		return
	}

	go openMainWindow(ctx)
}

func openMainWindow(ctx context.Context) {
	wailsruntime.WindowSetAlwaysOnTop(ctx, false)
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowSetMinSize(ctx, 900, 560)
	wailsruntime.WindowSetSize(ctx, 900, 560)
	wailsruntime.WindowCenter(ctx)
	wailsruntime.WindowShow(ctx)
	wailsruntime.EventsEmit(ctx, "app:show-main")
}
