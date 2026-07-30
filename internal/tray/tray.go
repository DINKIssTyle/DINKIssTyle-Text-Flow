package tray

type Actions struct {
	AskAI          func()
	OCR            func()
	ShowMainWindow func()
	ToggleFlow     func()
	Quit           func()
}

type State struct {
	FlowPaused bool
	Running    bool
	OCREnabled bool
}

func call(action func()) {
	if action != nil {
		action()
	}
}
