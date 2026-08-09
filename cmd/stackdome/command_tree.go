package main

import (
	"fmt"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/spf13/cobra"
)

type commandDocs struct {
	Use     string
	Short   string
	Long    string
	Example string
}

func documentCommand(cmd *cobra.Command, docs commandDocs) *cobra.Command {
	cmd.Use = docs.Use
	cmd.Short = docs.Short
	cmd.Long = docs.Long
	cmd.Example = docs.Example
	cmd.Aliases = nil
	return cmd
}

func newVerbCommand(use, short, long, example string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			if correction := nounCorrection(cmd.Name(), args[0]); correction != "" {
				return clierrors.ValidationError(fmt.Sprintf("unknown resource %q for %q; use `%s`", args[0], cmd.Name(), correction))
			}
			return clierrors.ValidationError(fmt.Sprintf("unknown resource %q for %q; run `stackdome %s --help` to list supported resources", args[0], cmd.Name(), cmd.Name()))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}

func nounCorrection(verb, noun string) string {
	corrections := map[string]map[string]string{
		"list": {
			"stack": "stackdome list stacks", "build": "stackdome list builds",
			"release": "stackdome list releases", "secret": "stackdome list secrets",
			"volume": "stackdome list volumes", "postgres-addon": "stackdome list postgres-addons",
			"token": "stackdome list tokens", "token-scope": "stackdome list token-scopes",
		},
		"describe": {
			"stacks": "stackdome describe stack <stack>", "builds": "stackdome describe build <build-id>",
			"releases": "stackdome describe release <release-id>", "secrets": "stackdome describe secret <name>",
			"postgres-addons": "stackdome describe postgres-addon <name>",
		},
	}
	return corrections[verb][noun]
}

func collectionDocs(verb, resource, description, scopeExample string) commandDocs {
	equivalent := "list"
	if verb == "list" {
		equivalent = "get"
	}
	return commandDocs{
		Use:   resource,
		Short: description,
		Long: fmt.Sprintf("%s\n\nThis command is equivalent to `stackdome %s %s`.",
			description+".", equivalent, resource),
		Example: fmt.Sprintf("  stackdome %s %s\n  stackdome %s %s %s",
			verb, resource, verb, resource, scopeExample),
	}
}

func detailDocs(verb, resource, argument, description, example string) commandDocs {
	equivalent := "describe"
	if verb == "describe" {
		equivalent = "get"
	}
	return commandDocs{
		Use:   fmt.Sprintf("%s <%s>", resource, argument),
		Short: description,
		Long: fmt.Sprintf("%s.\n\nThis command is equivalent to `stackdome %s %s <%s>`.",
			description, equivalent, resource, argument),
		Example: fmt.Sprintf("  stackdome %s %s %s", verb, resource, example),
	}
}

func operationDocs(use, short, long, example string) commandDocs {
	return commandDocs{Use: use, Short: short, Long: long, Example: "  " + example}
}

func newCommandTree() []*cobra.Command {
	get := newVerbCommand(
		"get",
		"Get Stackdome resources",
		"Get a resource collection or one identified resource. Use plural resource names for collections and singular names with an identifier for details.",
		"  stackdome get builds\n  stackdome get build <build-id>",
	)
	list := newVerbCommand(
		"list",
		"List Stackdome resource collections",
		"List a Stackdome resource collection. Every list command is equivalent to its `stackdome get <plural-resource>` form.",
		"  stackdome list stacks\n  stackdome list builds --stack demo",
	)
	describe := newVerbCommand(
		"describe",
		"Describe one Stackdome resource",
		"Show details for one identified resource. Every describe command is equivalent to its `stackdome get <singular-resource> <identifier>` form.",
		"  stackdome describe stack demo\n  stackdome describe build <build-id>",
	)

	get.AddCommand(
		documentCommand(newStackListCmd(), collectionDocs("get", "stacks", "List all stacks", "-o json")),
		documentCommand(newStackInfoCmd(), detailDocs("get", "stack", "stack", "Show one stack by name or ID", "demo")),
		documentCommand(newBuildListCmd(), collectionDocs("get", "builds", "List builds for the selected or specified stack", "--stack demo -o json")),
		documentCommand(newBuildInfoCmd(), detailDocs("get", "build", "build-id", "Show one build by ID or ID prefix", "<build-id>")),
		documentCommand(newReleaseListCmd(), collectionDocs("get", "releases", "List releases for the selected or specified stack", "--stack demo -o json")),
		documentCommand(newReleaseInfoCmd(), detailDocs("get", "release", "release-id", "Show one release by ID or ID prefix", "<release-id>")),
		documentCommand(newReleaseEventsCmd(), operationDocs("release-events <release-id>", "List or follow release events", "List events for one release. Pass --follow to stream new events until the stream ends. This command is equivalent to `stackdome list release-events <release-id>`.", "stackdome get release-events <release-id> --follow")),
		documentCommand(newSecretListCmd(), collectionDocs("get", "secrets", "List project secrets without revealing values", "-o json")),
		documentCommand(newSecretInfoCmd(), detailDocs("get", "secret", "name", "Show secret metadata without revealing values", "api-key")),
		documentCommand(newVolumeListCmd(), collectionDocs("get", "volumes", "List volumes for the selected or specified stack", "--stack demo")),
		documentCommand(newPostgresListCmd(), collectionDocs("get", "postgres-addons", "List PostgreSQL addons", "-o json")),
		documentCommand(newPostgresInfoCmd(), detailDocs("get", "postgres-addon", "name", "Show one PostgreSQL addon", "database")),
		documentCommand(newPostgresBackupsCmd(), operationDocs("postgres-backups <postgres-addon>", "List PostgreSQL backups", "List backups belonging to one PostgreSQL addon. This command is equivalent to `stackdome list postgres-backups <postgres-addon>`.", "stackdome get postgres-backups database")),
		documentCommand(newPostgresCredentialsCmd(), operationDocs("postgres-credentials <postgres-addon> <database>", "Get just-in-time PostgreSQL credentials", "Return sensitive, short-lived credentials for one database. Treat all output as secret material.", "stackdome get postgres-credentials database app -o json")),
		documentCommand(newTokenListCmd(), collectionDocs("get", "tokens", "List API tokens without revealing token values", "-o json")),
		documentCommand(newTokenScopesCmd(), collectionDocs("get", "token-scopes", "List valid API token scopes", "-o json")),
		documentCommand(newConfigViewCmd(), operationDocs("config", "Show effective CLI configuration", "Show the active server, authentication method, project, and stack with stored secrets redacted.", "stackdome get config -o yaml")),
		documentCommand(newStackfileSchemaCmd(), operationDocs("stackfile-schema", "Print the canonical Stackfile schema", "Print the embedded canonical Stackfile JSON Schema without contacting the server.", "stackdome get stackfile-schema -o json")),
	)

	list.AddCommand(
		documentCommand(newStackListCmd(), collectionDocs("list", "stacks", "List all stacks", "-o json")),
		documentCommand(newBuildListCmd(), collectionDocs("list", "builds", "List builds for the selected or specified stack", "--stack demo -o json")),
		documentCommand(newReleaseListCmd(), collectionDocs("list", "releases", "List releases for the selected or specified stack", "--stack demo -o json")),
		documentCommand(newReleaseEventsCmd(), operationDocs("release-events <release-id>", "List or follow release events", "List events for one release. This command is equivalent to `stackdome get release-events <release-id>`.", "stackdome list release-events <release-id>")),
		documentCommand(newSecretListCmd(), collectionDocs("list", "secrets", "List project secrets without revealing values", "-o json")),
		documentCommand(newVolumeListCmd(), collectionDocs("list", "volumes", "List volumes for the selected or specified stack", "--stack demo")),
		documentCommand(newPostgresListCmd(), collectionDocs("list", "postgres-addons", "List PostgreSQL addons", "-o json")),
		documentCommand(newPostgresBackupsCmd(), operationDocs("postgres-backups <postgres-addon>", "List PostgreSQL backups", "List backups belonging to one PostgreSQL addon. This command is equivalent to `stackdome get postgres-backups <postgres-addon>`.", "stackdome list postgres-backups database")),
		documentCommand(newTokenListCmd(), collectionDocs("list", "tokens", "List API tokens without revealing token values", "-o json")),
		documentCommand(newTokenScopesCmd(), collectionDocs("list", "token-scopes", "List valid API token scopes", "-o json")),
	)

	describe.AddCommand(
		documentCommand(newStackInfoCmd(), detailDocs("describe", "stack", "stack", "Show one stack by name or ID", "demo")),
		documentCommand(newBuildInfoCmd(), detailDocs("describe", "build", "build-id", "Show one build by ID or ID prefix", "<build-id>")),
		documentCommand(newReleaseInfoCmd(), detailDocs("describe", "release", "release-id", "Show one release by ID or ID prefix", "<release-id>")),
		documentCommand(newSecretInfoCmd(), detailDocs("describe", "secret", "name", "Show secret metadata without revealing values", "api-key")),
		documentCommand(newPostgresInfoCmd(), detailDocs("describe", "postgres-addon", "name", "Show one PostgreSQL addon", "database")),
	)

	create := newVerbCommand("create", "Create Stackdome resources", "Create one Stackdome resource. Resource-specific help documents required flags, output, and side effects.", "  stackdome create secret api-key --data KEY=value\n  stackdome create volume data --size 5Gi")
	create.AddCommand(
		documentCommand(newReleaseCreateCmd(), operationDocs("release", "Create a release from saved stack state", "Create a release from the saved definition of the selected or specified stack. This command does not apply a Stackfile. Pass --wait to follow the created release and --timeout to bound the wait.", "stackdome create release --stack demo --wait")),
		documentCommand(newSecretCreateCmd(), operationDocs("secret <name>", "Create a secret", "Create a project secret. Secret values are sensitive and are never shown by later get or describe commands.", "stackdome create secret api-key --data KEY=value")),
		documentCommand(newVolumeCreateCmd(), operationDocs("volume <name>", "Create a stack volume", "Create a volume in the selected or specified stack.", "stackdome create volume data --size 5Gi")),
		documentCommand(newPostgresCreateCmd(), operationDocs("postgres-addon <name>", "Create a PostgreSQL addon", "Provision a PostgreSQL addon. Pass --wait to wait for it to become ready.", "stackdome create postgres-addon database --wait")),
		documentCommand(newTokenCreateCmd(), operationDocs("token <name>", "Create an API token", "Create an API token. Its sensitive value is printed once and cannot be retrieved later.", "stackdome create token ci --scope 'stacks:*' -o json")),
	)

	update := newVerbCommand("update", "Update Stackdome resources", "Update one existing Stackdome resource.", "  stackdome update secret api-key --data KEY=value")
	update.AddCommand(documentCommand(newSecretSetCmd(), operationDocs("secret <name>", "Update a secret", "Update an existing secret's values. Values remain hidden from later reads.", "stackdome update secret api-key --data KEY=value")))

	deleteCmd := newVerbCommand("delete", "Delete Stackdome resources", "Permanently delete one Stackdome resource. Destructive commands prompt unless --yes is supplied.", "  stackdome delete stack demo --yes\n  stackdome delete secret api-key --yes")
	deleteCmd.AddCommand(
		documentCommand(newStackDeleteCmd(), operationDocs("stack <stack>", "Delete a stack", "Permanently delete one stack. The command prompts unless --yes is supplied.", "stackdome delete stack demo --yes")),
		documentCommand(newSecretDeleteCmd(), operationDocs("secret <name>", "Delete a secret", "Permanently delete one secret. The command prompts unless --yes is supplied.", "stackdome delete secret api-key --yes")),
		documentCommand(newVolumeDeleteCmd(), operationDocs("volume <name>", "Delete a stack volume", "Permanently delete one volume from the selected or specified stack. The command prompts unless --yes is supplied.", "stackdome delete volume data --yes")),
		documentCommand(newPostgresDeleteCmd(), operationDocs("postgres-addon <name>", "Delete a PostgreSQL addon", "Permanently delete one PostgreSQL addon. The command prompts unless --yes is supplied.", "stackdome delete postgres-addon database --yes")),
		documentCommand(newTokenDeleteCmd(), operationDocs("token <token-id>", "Delete an API token", "Revoke one API token. The command prompts unless --yes is supplied.", "stackdome delete token <token-id> --yes")),
	)

	use := newVerbCommand("use", "Select CLI context", "Select the Stackdome server or default stack used by later commands.", "  stackdome use stack demo\n  stackdome use context https://api.stackdome.example")
	use.AddCommand(
		documentCommand(newStackUseCmd(), operationDocs("stack <stack>", "Select the default stack", "Resolve a stack by name or ID and persist it as the default for stack-scoped commands.", "stackdome use stack demo")),
		documentCommand(newConfigSetContextCmd(), operationDocs("context <url>", "Select the Stackdome server", "Persist a different Stackdome server URL. Authenticate again after switching servers.", "stackdome use context https://api.stackdome.example")),
	)

	cancel := newVerbCommand("cancel", "Cancel a pending operation", "Cancel a pending Stackdome operation.", "  stackdome cancel release <release-id>")
	cancel.AddCommand(documentCommand(newReleaseCancelCmd(), operationDocs("release <release-id>", "Cancel a pending release", "Resolve and cancel one pending release in the selected or specified stack.", "stackdome cancel release <release-id>")))

	rollback := newVerbCommand("rollback", "Roll back a resource", "Create new desired state from historical Stackdome state.", "  stackdome rollback release <release-id> --wait")
	rollback.AddCommand(documentCommand(newReleaseRollbackCmd(), operationDocs("release <release-id>", "Create a release from historical state", "Create a new release from a historical release. Pass --wait to observe it to a terminal state.", "stackdome rollback release <release-id> --wait")))

	backup := newVerbCommand("backup", "Back up a resource", "Trigger an immediate backup of a supported Stackdome resource.", "  stackdome backup postgres-addon database")
	backup.AddCommand(documentCommand(newPostgresBackupCmd(), operationDocs("postgres-addon <name>", "Back up a PostgreSQL addon", "Trigger an immediate backup of one PostgreSQL addon.", "stackdome backup postgres-addon database --description manual")))

	export := newVerbCommand("export", "Export Stackdome resources", "Export a Stackdome resource into a local, portable representation.", "  stackdome export stackfile demo")
	export.AddCommand(documentCommand(newStackfileExportCmd(), operationDocs("stackfile <stack>", "Export a canonical Stackfile", "Export a saved stack as canonical Stackfile YAML or JSON.", "stackdome export stackfile demo --output-file stackfile.yaml")))

	return []*cobra.Command{get, list, describe, create, update, deleteCmd, use, cancel, rollback, backup, export}
}

func configureRootHelpGroups(root *cobra.Command) {
	root.AddGroup(
		&cobra.Group{ID: "read", Title: "Read Resources:"},
		&cobra.Group{ID: "change", Title: "Change Resources:"},
		&cobra.Group{ID: "deploy", Title: "Deploy and Release:"},
		&cobra.Group{ID: "observe", Title: "Observe and Operate:"},
		&cobra.Group{ID: "auth", Title: "Authentication and Context:"},
		&cobra.Group{ID: "tooling", Title: "Local Tooling:"},
	)

	groups := map[string]string{
		"get": "read", "list": "read", "describe": "read",
		"create": "change", "update": "change", "delete": "change",
		"apply": "deploy", "deploy": "deploy", "cancel": "deploy", "rollback": "deploy",
		"status": "observe", "logs": "observe", "restart": "observe", "open": "observe", "backup": "observe",
		"ctx": "auth", "login": "auth", "logout": "auth", "signup": "auth", "whoami": "auth", "use": "auth",
		"init": "tooling", "validate": "tooling", "export": "tooling", "doctor": "tooling",
		"api": "tooling", "completion": "tooling", "version": "tooling",
	}
	for _, cmd := range root.Commands() {
		cmd.GroupID = groups[cmd.Name()]
	}
}
