//go:build !darwin && !windows

package loginitem

func Enabled() bool {
	return false
}

func SetEnabled(enabled bool) error {
	return nil
}
