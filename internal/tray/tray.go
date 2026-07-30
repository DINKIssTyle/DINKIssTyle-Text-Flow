package tray

type Actions struct {
	AskAI          func()
	OCR            func()
	PinShot        func()
	ShowMainWindow func()
	ToggleFlow     func()
	Quit           func()
}

type State struct {
	FlowPaused     bool
	Running        bool
	PinShotEnabled bool
	OCREnabled     bool
}

func call(action func()) {
	if action != nil {
		action()
	}
}
