package tray

type Actions struct {
	AskAI          func()
	ShowMainWindow func()
	ToggleFlow     func()
	Quit           func()
}

type State struct {
	FlowPaused bool
	Running    bool
}

func call(action func()) {
	if action != nil {
		action()
	}
}
