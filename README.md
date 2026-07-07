# azure-exemption-cli

A Bubble Tea terminal UI that guides you through creating Azure Policy exemptions with the Azure CLI.

## Installation

Download the latest binary for your platform from
[GitHub Releases](https://github.com/Lukas-Klein/azure-exemption-cli/releases/latest).

### macOS / Linux

```bash
# Set the OS and architecture (examples: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64)
OS=darwin
ARCH=arm64

curl -sL "https://github.com/Lukas-Klein/azure-exemption-cli/releases/latest/download/azure-exemption-cli_${OS}_${ARCH}.tar.gz" \
  | tar xz azure-exemption-cli
sudo mv azure-exemption-cli /usr/local/bin/
```

### Windows (PowerShell)

```powershell
$arch = "amd64"  # or "arm64"
$zip  = "azure-exemption-cli_windows_${arch}.zip"
$dest = "$env:LOCALAPPDATA\Programs\azure-exemption-cli"

Invoke-WebRequest `
  "https://github.com/Lukas-Klein/azure-exemption-cli/releases/latest/download/$zip" `
  -OutFile "$env:TEMP\$zip"
Expand-Archive -Path "$env:TEMP\$zip" -DestinationPath $dest -Force
Remove-Item "$env:TEMP\$zip"

# Add to PATH for the current user (persistent across sessions)
$path = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($path -notlike "*$dest*") {
  [Environment]::SetEnvironmentVariable('Path', "$path;$dest", 'User')
  $env:Path += ";$dest"
}
```

Restart your terminal, then run `azure-exemption-cli`.

### Build from source

Requires Go 1.21 or later:

```bash
go install github.com/Lukas-Klein/azure-exemption-cli@latest
```

## Prerequisites

- The [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli) available on your `PATH`
- Permission to list subscriptions, read policy definitions and create exemptions

## What it does

1. **Authentication**: Ensures you are logged into Azure (`az login` is started automatically when needed).
2. **Subscription Selection**: Retrieves all subscriptions you have access to and lets you pick one.
3. **Assignment Selection**: Lists all policy assignments in the selected subscription.
4. **Definition Selection**: If the assignment is a Policy Set (Initiative), allows you to exempt the entire assignment or specific definitions within it.
5. **Scope Selection**: Choose to apply the exemption at the Subscription level or select a specific Resource Group.
6. **Details**: Prompts for a tracking ticket number and requester names.
7. **Expiration**: Optionally set an expiration date for the exemption.
8. **Creation**: Calls `az policy exemption create` with the collected data and prints the Azure CLI response.

## Usage

```bash
# Run directly
go run main.go

# Or build and run
go build -o azure-exemption-cli main.go
./azure-exemption-cli
```

Follow the on-screen instructions. Use `↑/↓` to navigate lists, `Space` to toggle selections, and `Enter` to confirm. Press `q` at any time to quit.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑/↓` or `k/j` | Navigate lists |
| `Enter` | Confirm selection |
| `Space` | Toggle selection (in multi-select lists) |
| `Backspace` | Go back to previous step |
| `q` | Quit the application |
| Type characters | Search/filter subscriptions |
| `Esc` | Clear search |

## Configuration

The CLI supports an optional configuration file to customize behavior. The config file is searched in the following locations (first match wins):

1. `./config.yaml` (current directory)
2. `./config.yml`
3. `~/.azure-exemption-cli/config.yaml`
4. `~/.azure-exemption-cli/config.yml`
5. `$XDG_CONFIG_HOME/azure-exemption-cli/config.yaml` (or `~/.config/azure-exemption-cli/config.yaml`)

See `config.yaml.example` for a sample configuration file.

### Blocking Policy Definitions

You can configure a list of policy definitions that cannot be exempted. This is useful for enforcing compliance by preventing exemptions on critical security or governance policies.

```yaml
blocked_policy_definition_ids:
  - /providers/Microsoft.Authorization/policyDefinitions/e56962a6-4747-49cd-b67b-bf8b01975c4c
```

**How it works:**

- The blocked list uses **policy definition IDs** (not assignment IDs)
- A single policy definition can be used by multiple policy assignments across your environment
- When you block a policy definition, **all assignments using that definition** will be blocked
- Blocked assignments appear greyed out with a `[-]` marker and `[blocked]` label
- Attempting to select a blocked assignment shows an error message

**Example:** If you block the "Inherit a tag from the subscription" policy definition, any policy assignment that uses this definition will be blocked - whether it's a standalone assignment or part of a policy set (initiative).

**Finding Policy Definition IDs:**

You can find policy definition IDs using the Azure CLI:

```bash
# List all policy definitions
az policy definition list --query "[].{name:name, displayName:displayName, id:id}" -o table

# Find a specific policy by display name
az policy definition list --query "[?contains(displayName, 'Inherit a tag')].{displayName:displayName, id:id}" -o table

# Get the definition ID from a policy assignment
az policy assignment show --name <assignment-name> --query "policyDefinitionId" -o tsv
```

## Project Structure

The project follows a standard Go project layout:

- `main.go`: Application entry point.
- `/azure`: Azure CLI interaction logic and types.
- `/tui`: Bubble Tea UI model, views, and update logic.
- `/config`: Configuration loading and parsing.
