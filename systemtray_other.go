//go:build !darwin

package main

import (
	"dkst-text-flow/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) configureSystemTray(appInst *application.App) {
	a.systray = appInst.SystemTray.New()
	a.systray.SetTemplateIcon(menuIcon)
	a.systray.SetTooltip("DKST Text Flow")

	trayMenu := appInst.NewMenu()
	trayMenu.Add("Ask AI").OnClick(func(ctx *application.Context) {
		a.showAIPrompt(platform.GetFrontmostPID(), false)
	})
	trayMenu.Add("Main Window").OnClick(func(ctx *application.Context) {
		a.showMainWindow()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		appInst.Quit()
	})
	a.systray.SetMenu(trayMenu)
}

func (a *App) destroySystemTray() {}
