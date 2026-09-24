package domain

// HA (High Availability) value types for the `system ha check` cross-check.
//
// These are grounded in the live FG100F/7.4.12 A-P cluster shapes:
//   - monitor/system/ha-peer     → HAMember (roles/priority per unit)
//   - cmdb/system/ha             → HAConfig (mode, group, heartbeat devices)
//   - monitor/system/ha-checksums → HAChecksum (config-sync state per unit)
//
// The record types (HAMember, HAChecksum) carry the untouched device record in
// Raw for faithful json/yaml passthrough, matching Transceiver/Sensor/LLDP. The
// provider decodes every field defensively — a missing/renamed/retyped field
// leaves the typed value zero, never a panic.

// HAMember is one cluster member from monitor/system/ha-peer. On the primary the
// device reports master/primary true; on a secondary those keys are absent, so
// Primary is false. A standalone unit returns a single member.
type HAMember struct {
	Hostname   string         `json:"hostname"`
	Serial     string         `json:"serial"`
	Priority   int            `json:"priority"`
	Primary    bool           `json:"primary"`
	VclusterID int            `json:"vcluster_id"`
	Raw        map[string]any `json:"-"`
}

// HAConfig is the cluster configuration from the cmdb/system/ha singleton. Mode
// is "standalone"/"a-p"/"a-a"; HeartbeatDevs are the interface names parsed out
// of the FortiOS hbdev string (e.g. `"ha1" 50 "ha2" 50 ` → ["ha1","ha2"]).
type HAConfig struct {
	Mode          string   `json:"mode"`
	GroupName     string   `json:"group_name"`
	HeartbeatDevs []string `json:"heartbeat_devs"`
}

// HAChecksum is one member's config-sync state from monitor/system/ha-checksums.
// All is the whole-config checksum (config is in sync when every member's All
// matches); VDOMs maps each VDOM name to its per-VDOM checksum, so a divergence
// can be named down to the specific VDOM. Raw holds the verbatim device record.
type HAChecksum struct {
	Serial  string            `json:"serial"`
	Primary bool              `json:"primary"`
	All     string            `json:"all"`
	VDOMs   map[string]string `json:"vdoms"`
	Raw     map[string]any    `json:"-"`
}
