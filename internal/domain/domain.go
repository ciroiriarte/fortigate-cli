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

// HAMember is one node of an HA cluster as reported by the monitor surface.
// Field names track monitor/system/ha-statistics; json/yaml carries these typed
// fields (the stable contract), while the table view is best-effort.
type HAMember struct {
	Serial   string `json:"serial_no"`
	Hostname string `json:"hostname"`
	Priority int    `json:"priority"`
	CPU      int    `json:"cpu_usage"`
	Memory   int    `json:"mem_usage"`
	Sessions int    `json:"sessions"`
}
