package cli

import "github.com/spf13/cobra"

// systemAutomationCommands returns the `automation` group (trigger, action,
// stitch). newSystemCmd wires it in.
func systemAutomationCommands(a *app) []*cobra.Command {
	trigger := resource{
		use: "trigger", short: "Manage automation triggers",
		path: "system/automation-trigger", mkey: "name",
		columns: []column{
			{field: "name"},
			{field: "trigger-type"},
			{field: "event-type"},
		},
		fields: []fieldSpec{
			{name: "trigger-type", usage: "event-based|scheduled"},
			{name: "event-type", usage: "event/log trigger type, e.g. event-log|reboot"},
			{name: "logid", usage: "log ID(s) to match"},
			{name: "trigger-frequency", usage: "hourly|daily|weekly|monthly"},
			{name: "trigger-hour", usage: "hour of day (0-23)"},
			{name: "trigger-minute", usage: "minute of hour (0-59)"},
			{name: "description", usage: "free-text description"},
		},
	}

	action := resource{
		use: "action", short: "Manage automation actions",
		path: "system/automation-action", mkey: "name",
		columns: []column{
			{field: "name"},
			{field: "action-type"},
		},
		fields: []fieldSpec{
			{name: "action-type", usage: "email|alert|disable-ssid|system-actions|webhook|cli-script|aws-lambda|azure-function|..."},
			{name: "email-to", kind: kindRefList, usage: "recipient name(s), comma-separated"},
			{name: "email-subject", usage: "email subject"},
			{name: "message", usage: "message body"},
			{name: "method", usage: "post|put|get|patch|delete (webhook)"},
			{name: "uri", usage: "webhook URI"},
			{name: "http-body", usage: "webhook body"},
			{name: "minimum-interval", usage: "rate-limit seconds"},
			{name: "description", usage: "free-text description"},
		},
	}

	stitch := resource{
		use: "stitch", short: "Manage automation stitches",
		path: "system/automation-stitch", mkey: "name",
		columns: []column{
			{field: "name"},
			{field: "status"},
			{field: "trigger"},
		},
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "trigger", usage: "trigger name"},
			{name: "description", usage: "free-text description"},
		},
	}

	automation := &cobra.Command{Use: "automation", Aliases: []string{"auto"}, Short: "Manage automation triggers, actions, and stitches"}
	automation.AddCommand(a.newResourceCmd(trigger), a.newResourceCmd(action), a.newResourceCmd(stitch))
	return []*cobra.Command{automation}
}
