//go:build !darwin

package tray

import "github.com/wailsapp/wails/v3/pkg/application"

type Manager struct {
	systemTray *application.SystemTray
	toggleItem *application.MenuItem
	activeIcon []byte
	pausedIcon []byte
}

func New(app *application.App, activeIcon []byte, pausedIcon []byte, actions Actions) *Manager {
	systemTray := app.SystemTray.New()
	systemTray.SetIcon(activeIcon)
	systemTray.SetTooltip("DKST Text Flow")

	menu := app.NewMenu()
	menu.Add("Ask AI").OnClick(func(*application.Context) {
		call(actions.AskAI)
	})
	menu.Add("Main Window").OnClick(func(*application.Context) {
		call(actions.ShowMainWindow)
	})
	menu.AddSeparator()
	toggleItem := menu.Add("Pause Flow").OnClick(func(*application.Context) {
		call(actions.ToggleFlow)
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		call(actions.Quit)
	})
	systemTray.SetMenu(menu)

	return &Manager{
		systemTray: systemTray,
		toggleItem: toggleItem,
		activeIcon: append([]byte(nil), activeIcon...),
		pausedIcon: append([]byte(nil), pausedIcon...),
	}
}

func (m *Manager) UpdateState(state State) {
	if m == nil || m.systemTray == nil {
		return
	}
	icon := m.activeIcon
	tooltip := "DKST Text Flow"
	if !state.Running {
		icon = m.pausedIcon
		tooltip = "DKST Text Flow — Paused"
	}
	if len(icon) > 0 {
		m.systemTray.SetIcon(icon)
	}
	m.systemTray.SetTooltip(tooltip)
	if m.toggleItem != nil {
		label := "Pause Flow"
		if state.FlowPaused {
			label = "Resume Flow"
		}
		application.InvokeSync(func() {
			m.toggleItem.SetLabel(label)
		})
	}
}

func (m *Manager) Destroy() {
	if m == nil {
		return
	}
	m.systemTray = nil
	m.toggleItem = nil
}
