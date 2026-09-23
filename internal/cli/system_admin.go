package cli

import "github.com/spf13/cobra"

// systemAdminCommands returns curated REST-API admin + global/per-VDOM settings
// resources (api-user, global, settings). newSystemCmd wires them in.
func systemAdminCommands(a *app) []*cobra.Command {
	apiUser := resource{
		use:     "api-user",
		aliases: []string{"api-admin"},
		short:   "Manage REST API admin accounts (keys generated via execute api-user generate-key)",
		path:    "system/api-user",
		mkey:    "name",
		columns: []column{
			{field: "name"},
			{field: "accprofile"},
		},
		fields: []fieldSpec{
			{name: "accprofile", usage: "access profile ref (permission scope)"},
			{name: "vdom", kind: kindRefList, usage: "accessible VDOM(s), comma-separated"},
			{name: "comments", usage: "free-text comment"},
			{name: "cors-allow-origin", usage: "CORS origin"},
			{name: "peer-auth", usage: "enable|disable (PKI peer auth)"},
			{name: "peer-group", usage: "PKI peer group ref"},
		},
	}

	global := resource{
		use:    "global",
		short:  "Manage global system settings",
		single: true,
		path:   "system/global",
		fields: []fieldSpec{
			{name: "hostname", usage: "device hostname"},
			{name: "admin-sport", usage: "HTTPS admin port"},
			{name: "admin-ssh-port", usage: "SSH admin port"},
			{name: "admintimeout", usage: "idle timeout (minutes)"},
			{name: "timezone", usage: "timezone index/name"},
			{name: "admin-https-redirect", usage: "enable|disable"},
			{name: "language", usage: "GUI language"},
			{name: "gui-theme", usage: "GUI theme"},
		},
	}

	settings := resource{
		use:    "settings",
		short:  "Manage per-VDOM settings",
		single: true,
		path:   "system/settings",
		fields: []fieldSpec{
			{name: "opmode", usage: "nat|transparent"},
			{name: "inspection-mode", usage: "flow|proxy"},
			{name: "central-nat", usage: "enable|disable"},
			{name: "allow-subnet-overlap", usage: "enable|disable"},
		},
	}

	return []*cobra.Command{
		a.newResourceCmd(apiUser),
		a.newResourceCmd(global),
		a.newResourceCmd(settings),
	}
}
