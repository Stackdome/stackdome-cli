# CLI Context Command Design

## Goal

Add a discoverable `stackdome ctx` command that reports the active Stackdome
identity and scope, including the selected stack's name and ID. Make the current
stack explicit in both table and structured output from `stackdome get stacks`
and its `stackdome list stacks` alias.

## Command Surface

`stackdome ctx` is a root command in the **Authentication and Context** help
group. It accepts the global `--output table|json|yaml` options and no positional
arguments.

The table output is:

```text
User:     Ashish <ashish@example.com>
Server:   https://stackdome.io
Org:      c1348df5-ef90-481c-b59b-ad089a74090c
Project:  default
Stack:    n8n (c87a8e72-6a96-47ce-b606-537b1bdab59a)
Auth:     API token (from config file)
```

When there is no selected stack, the command prints `Stack: none` followed by
`Select one with: stackdome use stack <stack>` in table mode. Structured output
uses a stable context object:

```json
{
  "user": "Ashish",
  "email": "ashish@example.com",
  "server_url": "https://stackdome.io",
  "organization_id": "c1348df5-ef90-481c-b59b-ad089a74090c",
  "project": "default",
  "current_stack": {
    "name": "n8n",
    "id": "c87a8e72-6a96-47ce-b606-537b1bdab59a"
  },
  "auth_method": "api token",
  "token_source": "config file"
}
```

`current_stack` is omitted when no stack is selected. Credentials and token
fragments are never included.

## Context Resolution

The command requires authentication and uses the existing scope middleware, so
missing organization/project values are discovered consistently with other CLI
commands. It fetches the current user for live identity information.

When `Config.CurrentStack` is non-empty, the command lists stacks once and
matches the selection by full ID or name. Name matching preserves compatibility
with older config files; current versions continue to persist full stack IDs.
If the saved selection cannot be resolved, the command returns the existing
stack-not-found error and exit code 3 rather than presenting stale context as
valid.

Authentication method and token-source formatting will be shared with
`whoami`, keeping both commands consistent without changing the existing
`whoami` output contract.

## Stack List Output

Table output from both `stackdome get stacks` and `stackdome list stacks` uses
an explicit leading column:

```text
CURRENT   NAME   ID   STATE
*         n8n    ...  Released
```

Exactly the selected stack receives `*`; other rows are blank. Matching accepts
either the stored full ID or a legacy stored name.

JSON and YAML output retain all existing stack fields and add a top-level
`current: true|false` field to every list item. This makes the selection
machine-readable without forcing agents to compare configuration IDs manually.

## Error Handling

- Missing or invalid credentials retain the standard authentication error and
  exit code 2.
- An unresolved saved stack selection returns stack-not-found and exit code 3.
- `ctx` rejects positional arguments with the standard usage error and exit
  code 4.
- Environment-controlled contexts continue to clear the file-backed stack
  selection in memory, so `ctx` reports no selected stack rather than leaking a
  stack from another server, organization, or project.

## Testing

Tests will cover:

- root registration, help group, descriptions, examples, and zero-argument
  grammar for `ctx`;
- table and JSON/YAML context output with a selected stack;
- the no-selected-stack output and structured omission behavior;
- legacy name-based stack resolution and unresolved-selection exit behavior;
- explicit `CURRENT` table headers and markers for both stack-list aliases;
- `current` booleans in JSON/YAML stack lists;
- preservation of credential redaction and the existing `whoami` contract.

Implementation follows test-driven development: each behavior is introduced by
a failing test before production code changes.
