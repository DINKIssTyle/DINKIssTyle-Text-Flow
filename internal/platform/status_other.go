//go:build !darwin && !windows

package platform

func CurrentStatus() Status {
	return Status{
		Message: "Platform status is not implemented on this operating system yet.",
	}
}

func requestAccessibilityPermission() bool {
	return false
}

func selectedText() (string, error) {
	return "", nil
}

func selectedTextFromProcess(processID int) (string, error) {
	return selectedText()
}

func replaceSelectedTextInProcess(processID int, replacement string, preferPaste bool) error {
	return nil
}

func activateProcess(processID int) error {
	return nil
}

func appInfoFromProcess(processID int) AppInfo {
	return AppInfo{}
}

func appInfoFromBundlePath(path string) AppInfo {
	return AppInfo{Path: path}
}

func getFrontmostPID() int {
	return 0
}
