//go:build windows

package notify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// powerShellAUMID is the AppUserModelID of Windows PowerShell, which can always
// show toasts; used when app.ID can't be registered.
const powerShellAUMID = `{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe`

// send shows a toast through the WinRT ToastNotificationManager, from
// PowerShell. A desktop app without a Start menu shortcut needs its
// AppUserModelID registered under HKCU\Software\Classes\AppUserModelId for
// Windows to show its toasts (with app.Name as the sender).
func send(app App, title, body string) error {
	aumid := app.ID
	if err := register(app); err != nil {
		aumid = powerShellAUMID
	}
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$n = $t.GetElementsByTagName('text')
$n.Item(0).AppendChild($t.CreateTextNode(` + psQuote(title) + `)) > $null
$n.Item(1).AppendChild($t.CreateTextNode(` + psQuote(body) + `)) > $null
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier(` + psQuote(aumid) + `).Show([Windows.UI.Notifications.ToastNotification]::new($t))`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("toast: %s", strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}

// register makes app.ID a known AppUserModelID named app.Name.
func register(app App) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\AppUserModelId\`+app.ID, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()
	return k.SetStringValue("DisplayName", app.Name)
}

// psQuote quotes s as a PowerShell single-quoted string literal.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
