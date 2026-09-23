package cli

import "github.com/spf13/cobra"

// newLogCmd builds the `log` command group over the log/* singletons (dotted
// categories log.syslogd, log.fortianalyzer). The caller wires it into root.
func newLogCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "log", Short: "Manage logging configuration (cmdb surface)"}

	setting := resource{
		use: "setting", short: "Manage global logging settings", single: true,
		path: "log/setting",
		fields: []fieldSpec{
			{name: "resolve-ip", usage: "enable|disable (resolve IPs in logs)"},
			{name: "resolve-port", usage: "enable|disable (resolve ports in logs)"},
			{name: "fwpolicy-implicit-log", usage: "enable|disable"},
			{name: "local-in-allow", usage: "enable|disable"},
			{name: "local-out", usage: "enable|disable"},
		},
	}

	syslogdSetting := resource{
		use: "setting", short: "Manage syslog daemon settings", single: true,
		path: "log.syslogd/setting",
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "server", usage: "syslog server IP or FQDN"},
			{name: "port", usage: "syslog port (default 514)"},
			{name: "mode", usage: "udp|legacy-reliable|reliable"},
			{name: "facility", usage: "local0..local7|kernel|user|... (syslog facility)"},
			{name: "format", usage: "default|csv|cef|rfc5424"},
			{name: "source-ip", usage: "source IP for syslog"},
			{name: "enc-algorithm", usage: "high|low|disable|high-medium (TLS)"},
			{name: "interface-select-method", usage: "auto|sdwan|specify"},
			{name: "interface", usage: "egress interface (when specify)"},
			{name: "certificate", usage: "client certificate for reliable/TLS"},
		},
	}

	syslogdFilter := resource{
		use: "filter", short: "Manage syslog daemon filters", single: true,
		path: "log.syslogd/filter",
		fields: []fieldSpec{
			{name: "severity", usage: "emergency|alert|critical|error|warning|notification|information|debug"},
			{name: "forward-traffic", usage: "enable|disable"},
			{name: "local-traffic", usage: "enable|disable"},
			{name: "sniffer-traffic", usage: "enable|disable"},
			{name: "anomaly", usage: "enable|disable"},
			{name: "voip", usage: "enable|disable"},
			{name: "multicast-traffic", usage: "enable|disable"},
		},
	}

	fortianalyzerSetting := resource{
		use: "setting", short: "Manage FortiAnalyzer settings", single: true,
		path: "log.fortianalyzer/setting",
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "server", usage: "FortiAnalyzer IP"},
			{name: "upload-option", usage: "5-minute|1-minute|realtime"},
			{name: "source-ip", usage: "source IP"},
			{name: "enc-algorithm", usage: "high|low|disable|high-medium"},
			{name: "certificate", usage: "client certificate"},
			{name: "serial", usage: "FortiAnalyzer serial"},
		},
	}

	syslogd := &cobra.Command{Use: "syslogd", Aliases: []string{"syslog"}, Short: "Manage syslog daemon configuration"}
	syslogd.AddCommand(a.newResourceCmd(syslogdSetting), a.newResourceCmd(syslogdFilter))

	fortianalyzer := &cobra.Command{Use: "fortianalyzer", Aliases: []string{"faz"}, Short: "Manage FortiAnalyzer configuration"}
	fortianalyzer.AddCommand(a.newResourceCmd(fortianalyzerSetting))

	cmd.AddCommand(a.newResourceCmd(setting), syslogd, fortianalyzer)
	return cmd
}
