package tray

type Actions struct {
	AskAI          func()
	ShowMainWindow func()
	Quit           func()
}

func call(action func()) {
	if action != nil {
		action()
	}
}
