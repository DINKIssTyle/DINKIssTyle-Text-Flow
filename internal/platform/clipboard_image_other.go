//go:build !darwin && !windows

package platform

import "errors"

func WriteClipboardPNG(_ []byte) error {
	return errors.New("copying images is not supported on this platform")
}
