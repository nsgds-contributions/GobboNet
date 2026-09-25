//go:build windows

package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// lanScript is setup-lan.bat beside the running binary, or "" when there is
// none. A .deb has no such file, and neither does a `go run` tree.
func lanScript() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(exe), "setup-lan.bat")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return ""
	}
	return p
}

func firewallAvailableImpl() bool { return lanScript() != "" }

// openFirewall runs setup-lan.bat elevated, so the LAN answer covers both
// halves. Measured symptom of covering only one: the phone cannot connect,
// reports a timeout, and nothing on the machine says why.
//
// "runas" rather than powershell -Verb RunAs: same elevation, one process, and
// none of the cmd -> powershell shape installer/README.md warns about.
func openFirewallImpl() error {
	script := lanScript()
	if script == "" {
		return fmt.Errorf("setup-lan.bat not found beside the executable")
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(script)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(script))
	if err != nil {
		return err
	}
	// SW_SHOWNORMAL: the script prints what it changed and how to undo it,
	// and hiding that would make an elevation prompt the only visible trace.
	err = windows.ShellExecute(0, verb, file, nil, cwd, 1)
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return ErrFirewallDeclined
	}
	return err
}
