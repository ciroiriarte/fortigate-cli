package cli

import "github.com/spf13/cobra"

// newUserCmd assembles the `user` command group: local accounts and groups
// (user_accounts.go) plus the remote-auth servers ldap/radius/tacacs+
// (user_servers.go). These back SSL-VPN and policy identity.
func newUserCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users, groups, and authentication servers (cmdb surface)",
	}
	cmd.AddCommand(userAccountCommands(a)...)
	cmd.AddCommand(userServerCommands(a)...)
	return cmd
}
