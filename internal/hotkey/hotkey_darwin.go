//go:build darwin

package hotkey

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Carbon -framework AppKit
#include <Carbon/Carbon.h>
#import <AppKit/AppKit.h>

extern void DKSTAIHotkeyPressed(int processID);

static EventHotKeyRef dkstAIHotKeyRef = NULL;
static EventHandlerRef dkstAIHotKeyHandlerRef = NULL;

static int DKSTFrontmostProcessID(void) {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (app == nil) {
        return 0;
    }
    return (int)[app processIdentifier];
}

static OSStatus DKSTAIHotkeyHandler(EventHandlerCallRef nextHandler, EventRef event, void *userData) {
    DKSTAIHotkeyPressed(DKSTFrontmostProcessID());
    return noErr;
}

static int DKSTRegisterAIHotKey(UInt32 keyCode, UInt32 modifiers) {
    if (dkstAIHotKeyRef != NULL) {
        UnregisterEventHotKey(dkstAIHotKeyRef);
        dkstAIHotKeyRef = NULL;
    }
    if (dkstAIHotKeyHandlerRef == NULL) {
        EventTypeSpec eventType;
        eventType.eventClass = kEventClassKeyboard;
        eventType.eventKind = kEventHotKeyPressed;
        OSStatus handlerStatus = InstallEventHandler(
            GetApplicationEventTarget(),
            DKSTAIHotkeyHandler,
            1,
            &eventType,
            NULL,
            &dkstAIHotKeyHandlerRef
        );
        if (handlerStatus != noErr) {
            return 0;
        }
    }

    EventHotKeyID hotKeyID;
    hotKeyID.signature = 'DKAI';
    hotKeyID.id = 1;
    OSStatus status = RegisterEventHotKey(
        keyCode,
        modifiers,
        hotKeyID,
        GetApplicationEventTarget(),
        0,
        &dkstAIHotKeyRef
    );
    return status == noErr ? 1 : 0;
}

static void DKSTUnregisterAIHotKey(void) {
    if (dkstAIHotKeyRef != NULL) {
        UnregisterEventHotKey(dkstAIHotKeyRef);
        dkstAIHotKeyRef = NULL;
    }
}
*/
import "C"

import (
	"fmt"
	"sync"
)

var state struct {
	sync.Mutex
	handler func(int)
}

func Register(value string, handler func(int)) error {
	shortcut, err := Parse(value)
	if err != nil {
		return err
	}
	keyCode, ok := keyCodeFor(shortcut.Key)
	if !ok {
		return fmt.Errorf("unsupported hotkey key: %s", shortcut.Key)
	}

	modifiers := carbonModifiers(shortcut)
	state.Lock()
	state.handler = handler
	state.Unlock()

	if C.DKSTRegisterAIHotKey(C.UInt32(keyCode), C.UInt32(modifiers)) != 1 {
		state.Lock()
		state.handler = nil
		state.Unlock()
		return fmt.Errorf("register hotkey failed: %s", shortcut.Canonical)
	}
	return nil
}

func Unregister() {
	C.DKSTUnregisterAIHotKey()
	state.Lock()
	state.handler = nil
	state.Unlock()
}

//export DKSTAIHotkeyPressed
func DKSTAIHotkeyPressed(processID C.int) {
	state.Lock()
	handler := state.handler
	state.Unlock()
	if handler != nil {
		go handler(int(processID))
	}
}

func carbonModifiers(shortcut Shortcut) uint32 {
	var modifiers uint32
	if shortcut.Command {
		modifiers |= uint32(C.cmdKey)
	}
	if shortcut.Control {
		modifiers |= uint32(C.controlKey)
	}
	if shortcut.Option {
		modifiers |= uint32(C.optionKey)
	}
	if shortcut.Shift {
		modifiers |= uint32(C.shiftKey)
	}
	return modifiers
}

func keyCodeFor(key string) (uint32, bool) {
	codes := map[string]uint32{
		"A": 0, "S": 1, "D": 2, "F": 3, "H": 4, "G": 5, "Z": 6, "X": 7, "C": 8, "V": 9,
		"B": 11, "Q": 12, "W": 13, "E": 14, "R": 15, "Y": 16, "T": 17, "O": 31, "U": 32,
		"I": 34, "P": 35, "L": 37, "J": 38, "K": 40, "N": 45, "M": 46,
		"0": 29, "1": 18, "2": 19, "3": 20, "4": 21, "5": 23, "6": 22, "7": 26, "8": 28, "9": 25,
		"Space": 49, "Tab": 48, "Enter": 36, "Return": 36, "Esc": 53, "Escape": 53,
		"Up": 126, "Down": 125, "Left": 123, "Right": 124,
		"-": 27, "=": 24, "[": 33, "]": 30, "\\": 42, ";": 41, "'": 39, ",": 43, ".": 47, "/": 44, "`": 50,
	}
	code, ok := codes[key]
	return code, ok
}
