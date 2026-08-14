//go:build windows

package notification

import toast "git.sr.ht/~jackmordaunt/go-toast/v2"

const appID = "XEngineer Voice Calendar"

type windowsNotifier struct{}

// New configures Windows toast notifications for the application.
func New() (Notifier, error) {
	if err := toast.SetAppData(toast.AppData{AppID: appID}); err != nil {
		return nil, err
	}
	return windowsNotifier{}, nil
}

func (windowsNotifier) Send(title, body string) error {
	n := toast.Notification{
		AppID: appID,
		Title: title,
		Body:  body,
		Audio: toast.Default,
	}
	return n.Push()
}
