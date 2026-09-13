package setup

import "errors"

// ErrFirewallDeclined is the Administrator prompt being answered No, which is a
// choice rather than a fault: setup is finished and correct, LAN access is not.
var ErrFirewallDeclined = errors.New("the Administrator prompt was declined")

// Variables so tests can drive the declined-prompt path, which is otherwise
// reachable only on Windows. What it does on a decline -- switch listen_host
// back to loopback -- changes the user's config, so it is worth a test that
// runs everywhere rather than one platform's word for it.
var (
	firewallAvailable = firewallAvailableImpl
	openFirewall      = openFirewallImpl
)
