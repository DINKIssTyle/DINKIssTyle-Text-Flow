//go:build !darwin

package tray

import "github.com/wailsapp/wails/v3/pkg/application"

type Manager struct {
	systemTray *application.SystemTray
}

func New(app *application.App, icon []byte, actions Actions) *Manager {
	systemTray := app.SystemTray.New()
	systemTray.SetTemplateIcon(icon)
	systemTray.SetTooltip("DKST Text Flow")

	menu := app.NewMenu()
	menu.Add("Ask AI").OnClick(func(*application.Context) {
		call(actions.AskAI)
	})
	menu.Add("Main Window").OnClick(func(*application.Context) {
		call(actions.ShowMainWindow)
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		call(actions.Quit)
	})
	systemTray.SetMenu(menu)

	return &Manager{systemTray: systemTray}
}

func (m *Manager) Destroy() {
	if m == nil {
		return
	}
	m.systemTray = nil
}
