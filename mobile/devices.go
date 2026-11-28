package mobile

// Devices lists this device first, then the other devices the library holds
// settings of ([]services.DeviceView), as JSON. A device marked sameName is
// likely this one before the app was reinstalled.
func Devices() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.sync.Devices(), nil)
}

// ForgetDevice removes another device's settings from the library, on every
// device. Its cloud saves and backups stay.
func ForgetDevice(id string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.sync.ForgetDevice(id)
}

// ReuseDevice makes this device take over the settings of device id (an
// earlier install of the app on this phone), and id is forgotten. Refused
// while a sync or game session runs.
func ReuseDevice(id string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.sync.ReuseDevice(id)
}
