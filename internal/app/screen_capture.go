package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"dkst-text-flow/internal/ocr"
	"dkst-text-flow/internal/platform"
	"dkst-text-flow/internal/windowing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type ScreenRegionSelection struct {
	ScreenID       string  `json:"screenId"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	ViewportWidth  float64 `json:"viewportWidth"`
	ViewportHeight float64 `json:"viewportHeight"`
}

func (a *App) BeginScreenRegionCapture() error {
	return a.beginScreenRegionCapture(false, 0)
}

func (a *App) beginOCRScreenRegionCapture(sourceProcessID int) error {
	if !a.tryBeginOCRProcessing() {
		return errors.New("Apple Vision OCR is already processing")
	}
	settings, err := a.GetGeneralSettings()
	if err != nil {
		a.finishOCRProcessing()
		return err
	}
	if !settings.AppleVisionOCREnabled {
		a.finishOCRProcessing()
		return errors.New("Apple Vision OCR is disabled")
	}
	if err := a.beginScreenRegionCapture(true, sourceProcessID); err != nil {
		a.finishOCRProcessing()
		a.showOCRWindow(OCRInvocation{
			SourceProcessID: sourceProcessID,
			Error:           err.Error(),
		})
		return err
	}
	return nil
}

func (a *App) beginScreenRegionCapture(forOCR bool, sourceProcessID int) error {
	a.screenCaptureMu.Lock()
	if a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return errors.New("screen capture is already active")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.screenCaptureActive = true
	a.screenCaptureCompleting = false
	a.screenCaptureContext = ctx
	a.screenCaptureCancel = cancel
	a.screenCaptureForOCR = forOCR
	a.screenCaptureSourcePID = sourceProcessID
	a.screenCaptureMu.Unlock()

	appInst := application.Get()
	if aiWindow, ok := appInst.Window.GetByName("ai"); ok {
		application.InvokeSync(func() {
			aiWindow.Hide()
		})
	}
	if ocrWindow, ok := appInst.Window.GetByName("ocr"); ok {
		application.InvokeSync(func() {
			ocrWindow.Hide()
		})
	}
	if err := a.beginPlatformScreenRegionCapture(ctx); err != nil {
		a.finishScreenRegionCapture(platform.ScreenCaptureResult{}, err)
		return err
	}
	return nil
}

func (a *App) CompleteScreenRegionCapture(selection ScreenRegionSelection) error {
	return a.completePlatformScreenRegionCapture(selection)
}

func (a *App) CancelScreenRegionCapture() {
	a.cancelScreenRegionCapture(true)
}

func (a *App) cancelScreenRegionCapture(restoreHUD bool) {
	a.screenCaptureMu.Lock()
	if !a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return
	}
	cancel := a.screenCaptureCancel
	forOCR := a.screenCaptureForOCR
	if !restoreHUD {
		a.screenCaptureActive = false
		a.screenCaptureCompleting = false
		a.screenCaptureContext = nil
		a.screenCaptureCancel = nil
		a.screenCaptureForOCR = false
		a.screenCaptureSourcePID = 0
	}
	windows := append([]application.Window(nil), a.screenCaptureWindows...)
	if !restoreHUD {
		a.screenCaptureWindows = nil
	}
	a.screenCaptureMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if !restoreHUD && forOCR {
		a.finishOCRProcessing()
	}
	if restoreHUD {
		a.finishScreenRegionCapture(platform.ScreenCaptureResult{Canceled: true}, nil)
		return
	}
	for _, captureWindow := range windows {
		captureWindow.Close()
	}
}

func (a *App) finishScreenRegionCapture(result platform.ScreenCaptureResult, captureErr error) {
	a.screenCaptureMu.Lock()
	if !a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return
	}
	cancel := a.screenCaptureCancel
	a.screenCaptureActive = false
	a.screenCaptureCompleting = false
	a.screenCaptureContext = nil
	a.screenCaptureCancel = nil
	forOCR := a.screenCaptureForOCR
	sourceProcessID := a.screenCaptureSourcePID
	a.screenCaptureForOCR = false
	a.screenCaptureSourcePID = 0
	windows := a.screenCaptureWindows
	a.screenCaptureWindows = nil
	a.screenCaptureMu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, captureWindow := range windows {
		captureWindow.Close()
	}
	if forOCR {
		a.finishOCRScreenRegionCapture(result, captureErr, sourceProcessID)
		return
	}

	appInst := application.Get()
	application.InvokeSync(func() {
		if aiWindow, ok := appInst.Window.GetByName("ai"); ok {
			aiWindow.SetAlwaysOnTop(true)
			aiWindow.UnMinimise()
			aiWindow.Show()
			windowing.ActivateForInput(aiWindow)
			aiWindow.Focus()
		}
	})
	switch {
	case captureErr != nil:
		appInst.Event.Emit("ai:screenshot-error", captureErr.Error())
	case result.Canceled:
		appInst.Event.Emit("ai:screenshot-canceled")
	default:
		appInst.Event.Emit("ai:screenshot-captured", result)
	}
}

type OCRInvocation struct {
	Text            string `json:"text"`
	SourceProcessID int    `json:"sourceProcessId"`
	Error           string `json:"error,omitempty"`
	Loading         bool   `json:"loading,omitempty"`
}

func (a *App) finishOCRScreenRegionCapture(
	result platform.ScreenCaptureResult,
	captureErr error,
	sourceProcessID int,
) {
	defer a.finishOCRProcessing()

	if result.Canceled {
		if sourceProcessID > 0 {
			_ = platform.ActivateProcess(sourceProcessID)
		}
		return
	}
	if captureErr != nil {
		a.showOCRWindow(OCRInvocation{
			SourceProcessID: sourceProcessID,
			Error:           captureErr.Error(),
		})
		return
	}

	settings, err := a.GetGeneralSettings()
	if err != nil {
		a.showOCRWindow(OCRInvocation{SourceProcessID: sourceProcessID, Error: err.Error()})
		return
	}
	a.showOCRWindow(OCRInvocation{
		SourceProcessID: sourceProcessID,
		Loading:         true,
	})
	recognized, err := ocr.RecognizePNG(result.PNGData, settings.OCRRecognitionLanguage)
	if err != nil {
		a.showOCRWindow(OCRInvocation{SourceProcessID: sourceProcessID, Error: err.Error()})
		return
	}
	if strings.TrimSpace(recognized.Text) == "" {
		a.showOCRWindow(OCRInvocation{
			SourceProcessID: sourceProcessID,
			Error:           "Apple Vision OCR did not recognize any text",
		})
		return
	}

	if settings.OCRResultAction == ocr.ResultActionClipboard {
		if err := copyTextToClipboard(recognized.Text); err != nil {
			a.showOCRWindow(OCRInvocation{SourceProcessID: sourceProcessID, Error: err.Error()})
			return
		}
		hideOCRWindow()
		if sourceProcessID > 0 {
			_ = platform.ActivateProcess(sourceProcessID)
		}
		application.Get().Event.Emit("ocr:copied", recognized.Text)
		return
	}

	a.showOCRWindow(OCRInvocation{
		Text:            recognized.Text,
		SourceProcessID: sourceProcessID,
	})
}

func (a *App) tryBeginOCRProcessing() bool {
	a.ocrProcessingMu.Lock()
	defer a.ocrProcessingMu.Unlock()
	if a.ocrProcessing {
		return false
	}
	a.ocrProcessing = true
	return true
}

func (a *App) finishOCRProcessing() {
	a.ocrProcessingMu.Lock()
	a.ocrProcessing = false
	a.ocrProcessingMu.Unlock()
}

func copyTextToClipboard(text string) error {
	command := exec.Command("/usr/bin/pbcopy")
	command.Stdin = strings.NewReader(text)
	if err := command.Run(); err != nil {
		return fmt.Errorf("failed to copy OCR text to the clipboard: %w", err)
	}
	return nil
}

func hideOCRWindow() {
	appInst := application.Get()
	if appInst == nil {
		return
	}
	application.InvokeSync(func() {
		if ocrWindow, ok := appInst.Window.GetByName("ocr"); ok {
			ocrWindow.Hide()
		}
	})
}

func (a *App) showOCRWindow(invocation OCRInvocation) {
	appInst := application.Get()
	if appInst == nil {
		return
	}
	application.InvokeSync(func() {
		if mainWindow, ok := appInst.Window.GetByName("main"); ok {
			mainWindow.Hide()
		}
		if aiWindow, ok := appInst.Window.GetByName("ai"); ok {
			aiWindow.Hide()
		}
		if ocrWindow, ok := appInst.Window.GetByName("ocr"); ok {
			ocrWindow.SetMinSize(460, 120)
			ocrWindow.SetSize(460, 220)
			ocrWindow.Center()
			ocrWindow.SetAlwaysOnTop(true)
			ocrWindow.UnMinimise()
			ocrWindow.Show()
			windowing.ActivateForInput(ocrWindow)
			ocrWindow.Focus()
		}
	})
	appInst.Event.Emit("ocr:result", invocation)
}

func (a *App) hideScreenCaptureWindows() {
	a.screenCaptureMu.Lock()
	windows := append([]application.Window(nil), a.screenCaptureWindows...)
	a.screenCaptureMu.Unlock()
	application.InvokeSync(func() {
		for _, captureWindow := range windows {
			captureWindow.Hide()
		}
	})
}
