// Package domain holds backend-agnostic value types the CLI renders. Providers
// translate raw FortiOS JSON into these; the cli layer never touches HTTP.
package domain

// Interface is a FortiOS system interface (physical, VLAN, aggregate, ...).
type Interface struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	IP     string `json:"ip"`
	Status string `json:"status"`
	VDOM   string `json:"vdom"`
	Alias  string `json:"alias"`
}

// DeviceStatus is the "what am I talking to" probe (monitor/system/status):
// model/serial/FortiOS version, used for identity display and version gating.
type DeviceStatus struct {
	Hostname string `json:"hostname"`
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	Version  string `json:"version"`
	Build    int    `json:"build"`
}

// ManagedSwitch is a FortiLink-managed FortiSwitch as seen by the FortiGate's
// switch-controller.
type ManagedSwitch struct {
	Serial   string `json:"serial"`
	Name     string `json:"name"`
	Model    string `json:"switch-id"`
	Status   string `json:"status"`
	State    string `json:"state"`
	Firmware string `json:"os_version"`
}
