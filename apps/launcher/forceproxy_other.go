//go:build !windows && !darwin

package main

// openAppChromium is a per-app force-proxy no-op on Linux; there is no
// process-level proxy handle, so the launch must fail rather than touch the OS
// system proxy.
func openAppChromium(app catalogApp) error { return errNoPerProcessProxy }
