//go:build windows

package hotkey

func Register(value string, handler func(int)) error {
	_, err := Parse(value)
	return err
}

func Unregister() {}
