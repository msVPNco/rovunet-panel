package actions

func init() {
	Register(&Action{
		ID:       SystemInstall,
		Name:     "Install",
		Category: "system",
		Inputs:   []InputField{{Key: "custom", Label: "Select a custom subset instead of the complete setup", Bool: true, CLIOnly: true}},
	})

	Register(&Action{
		ID:       SystemUninstall,
		Name:     "Uninstall",
		Category: "system",
	})

	Register(&Action{
		ID:       SystemUpdate,
		Name:     "Update",
		Category: "system",
	})

	Register(&Action{
		ID:       SystemRestart,
		Name:     "Restart",
		Category: "system",
	})

	Register(&Action{
		ID:       SystemUsers,
		Name:     "Users",
		Category: "system",
		Inputs: []InputField{
			{Key: "action", Label: "Action", Required: true, Options: []SelectOption{
				{Value: "add", Label: "Add user"},
				{Value: "bulk_add", Label: "Add multiple users (random creds)"},
				{Value: "edit_password", Label: "Edit user password"},
				{Value: "rotate_link", Label: "Replace subscription link (keep connection credentials)"},
				{Value: "renew", Label: "Renew a stopped user (expired or traffic used up)"},
				{Value: "remove", Label: "Remove user"},
				{Value: "list", Label: "List users"},
				{Value: "configs", Label: "Show all configs (every tunnel and user)"},
			}},
			{Key: "username", Label: "Username", DependsOn: "action", DependsOnValues: []string{"add", "edit_password", "rotate_link", "remove", "renew"}},
			{Key: "password", Label: "New password (leave blank to generate)", DependsOn: "action", DependsOnValues: []string{"edit_password"}},
			{Key: "count", Label: "How many users", DependsOn: "action", DependsOnValues: []string{"bulk_add"}},
			{Key: "prefix", Label: "Username prefix", DependsOn: "action", DependsOnValues: []string{"bulk_add"}},
			{Key: "days", Label: "Days until the user expires (blank or 0 = unlimited)", DependsOn: "action", DependsOnValues: []string{"add", "bulk_add", "renew"}},
			{Key: "gb", Label: "Traffic limit in GB (blank or 0 = unlimited)", DependsOn: "action", DependsOnValues: []string{"add", "bulk_add", "renew"}},
			{Key: "operation-id", Label: "Rotation request ID (32 lowercase hex); reuse for retries", CLIOnly: true},
		},
	})

	Register(&Action{
		ID:       SystemStats,
		Name:     "Stats",
		Category: "system",
	})

	Register(&Action{
		ID:       SystemDiag,
		Name:     "Diagnostics",
		Category: "system",
	})
	Register(&Action{ID: SystemCertificates, Name: "Certificates: check / recover", Category: "system", Inputs: []InputField{
		{Key: "mode", Label: "Certificate operation", Default: "check", Options: []SelectOption{{Value: "check", Label: "Check certificates"}, {Value: "recover", Label: "Recover an expired certificate"}}},
		{Key: "tag", Label: "TLS front or Slipstream tunnel tag (recovery only)", CLIOnly: true},
	}})

	Register(&Action{
		ID:       SystemMTU,
		Name:     "Set MTU (DNSTT/NoizDNS/VayDNS)",
		Category: "system",
		Inputs: []InputField{
			{Key: "mtu", Label: "MTU"},
		},
	})

	Register(&Action{
		ID:       SystemBackup,
		Name:     "Backup (config, keys, users)",
		Category: "system",
		Inputs: []InputField{
			{Key: "file", Label: `File to write the backup to ("-" for stdout)`, CLIOnly: true},
			{Key: "recipient", Label: "Age X25519 public recipient (not a secret)", CLIOnly: true},
			{Key: "passphrase-file", Label: "Owner-only passphrase file", CLIOnly: true},
			{Key: "plaintext", Label: "Explicitly export an unencrypted backup", CLIOnly: true, Bool: true},
		},
	})

	Register(&Action{
		ID:       SystemRestore,
		Name:     "Restore from backup",
		Category: "system",
		Inputs: []InputField{
			{Key: "file", Label: `Backup file to restore ("-" for stdin)`, CLIOnly: true},
			{Key: "yes", Label: "Restore without asking for confirmation", CLIOnly: true, Bool: true},
			{Key: "identity-file", Label: "Owner-only age private identity file", CLIOnly: true},
			{Key: "passphrase-file", Label: "Owner-only passphrase file", CLIOnly: true},
		},
	})

	Register(&Action{
		ID:       QuickWizard,
		Name:     "Quick Wizard",
		Category: "system",
		Inputs:   []InputField{{Key: "custom", Label: "Use the selective tunnel wizard instead of the complete setup", Bool: true, CLIOnly: true}},
	})

	Register(&Action{
		ID:       WarpToggle,
		Name:     "WARP",
		Category: "system",
	})

	Register(&Action{
		ID:       ConfigExport,
		Name:     "Export Config",
		Category: "config",
	})

	Register(&Action{
		ID:       ConfigImport,
		Name:     "Import Config",
		Category: "config",
		Inputs: []InputField{
			{Key: "path", Label: "Config file path", Required: true},
		},
	})

	Register(&Action{
		ID:       ConfigAddress,
		Name:     "Server address for client configs",
		Category: "config",
		Inputs: []InputField{
			{Key: "address", Label: `Public IP or hostname ("auto" to detect)`, CLIOnly: true},
		},
	})
}
