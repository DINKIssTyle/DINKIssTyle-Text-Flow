//go:build !darwin && !windows

package platform

func listRunningApps() []AppInfo {
	return []AppInfo{}
}
