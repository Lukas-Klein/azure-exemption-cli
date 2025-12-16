# azure-exemption-cli

A Bubble Tea terminal UI that guides you through creating Azure Policy exemptions with the Azure CLI.

## Prerequisites

- Go 1.21 or later
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

## Project Structure

The project follows a standard Go project layout:

- `main.go`: Application entry point.
- `internal/azure`: Azure CLI interaction logic and types.
- `internal/tui`: Bubble Tea UI model, views, and update logic.
