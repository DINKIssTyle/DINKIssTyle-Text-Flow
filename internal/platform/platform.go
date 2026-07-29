package platform

type Status struct {
	AccessibilityTrusted   bool   `json:"accessibilityTrusted"`
	ScreenRecordingGranted bool   `json:"screenRecordingGranted"`
	SecureInputActive      bool   `json:"secureInputActive"`
	ActiveAppName          string `json:"activeAppName"`
	ActiveBundleID         string `json:"activeBundleId"`
	FlowEngineRunning      bool   `json:"flowEngineRunning"`
	FlowPaused             bool   `json:"flowPaused"`
	Message                string `json:"message"`
}

type AppInfo struct {
	Name        string `json:"name"`
	BundleID    string `json:"bundleId"`
	Path        string `json:"path"`
	IconDataURL string `json:"iconDataUrl"`
}

type Controller interface {
	Start() error
	Stop() error
	Status() Status
}

func RequestAccessibilityPermission() bool {
	return requestAccessibilityPermission()
}

func RequestScreenRecordingPermission() bool {
	return requestScreenRecordingPermission()
}

func SelectedText() (string, error) {
	return selectedText()
}

func SelectedTextFromProcess(processID int) (string, error) {
	return selectedTextFromProcess(processID)
}

func ReplaceSelectedTextInProcess(processID int, replacement string, preferPaste bool) error {
	return replaceSelectedTextInProcess(processID, replacement, preferPaste)
}

func ActivateProcess(processID int) error {
	return activateProcess(processID)
}

func AppInfoFromProcess(processID int) AppInfo {
	return appInfoFromProcess(processID)
}

func AppInfoFromBundlePath(path string) AppInfo {
	return appInfoFromBundlePath(path)
}

func ListRunningApps() []AppInfo {
	return listRunningApps()
}

func GetFrontmostPID() int {
	return getFrontmostPID()
}

func IsFocusedElementEditable() bool {
	return isFocusedElementEditableForProcess(GetFrontmostPID())
}

func IsFocusedElementEditableForProcess(processID int) bool {
	return isFocusedElementEditableForProcess(processID)
}
