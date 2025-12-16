# azure-exemption-cli

A Bubble Tea terminal UI that guides you through creating Azure Policy exemptions with the Azure CLI.

## Prerequisites

- Go 1.21 or later
- The [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli) available on your `PATH`
- Permission to list subscriptions, read policy definitions and create exemptions

## What it does

1. Ensures you are logged into Azure (`az login` is started automatically when needed).
2. Retrieves all subscriptions you have access to and lets you pick one.
3. Lists every policy definition in that subscription, offers inline suggestions, and lets you type the exact policy name.
4. Prompts for the ticket number and the requester names.
5. Calls `az policy exemption create` with the collected data and prints the Azure CLI response.

## Usage

```bash
go run .
```

Follow the on-screen instructions. Use `↑/↓` to navigate the subscription list, type to filter policies, and press `q` at any time to quit.
