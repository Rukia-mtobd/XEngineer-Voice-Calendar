package notification

// Notifier delivers a native operating-system notification.
type Notifier interface {
	Send(title, body string) error
}
