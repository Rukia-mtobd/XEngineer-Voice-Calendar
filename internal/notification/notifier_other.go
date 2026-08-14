//go:build !windows

package notification

import "errors"

var errUnsupported = errors.New("system notifications are currently supported on Windows only")

type unsupportedNotifier struct{}

// New returns a notifier that reports that the current platform is unsupported.
func New() (Notifier, error) {
	return unsupportedNotifier{}, errUnsupported
}

func (unsupportedNotifier) Send(_, _ string) error {
	return errUnsupported
}
