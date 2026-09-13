//go:build !windows

package setup

import "errors"

// Only the Windows installer ships a firewall script. The packages deliberately
// touch nothing outside themselves, so on Linux the LAN answer sets listen_host
// and doctor reports on ufw or firewalld separately.
func firewallAvailableImpl() bool { return false }

func openFirewallImpl() error { return errors.New("no firewall script on this platform") }
