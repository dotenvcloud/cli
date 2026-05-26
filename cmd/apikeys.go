package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var apikeysCmd = &cobra.Command{
	Use:   "apikeys",
	Short: "Manage organization API keys",
	Long: `Manage API keys for your organization. API keys provide programmatic
access to the DotEnv API and are used for CI/CD integrations.

API keys have abilities (permissions) that control what operations they can perform.`,

	Example: `  # List all API keys
  dotenv apikeys list

  # Create a new API key
  dotenv apikeys create "CI/CD Key" --abilities="secrets:read,projects:read"

  # Update an API key name
  dotenv apikeys update key-id --name="New Name"

  # Delete an API key
  dotenv apikeys delete key-id

  # Rotate an API key
  dotenv apikeys rotate key-id`,
}

// Subcommands
var (
	apikeysListCmd   *cobra.Command
	apikeysCreateCmd *cobra.Command
	apikeysUpdateCmd *cobra.Command
	apikeysDeleteCmd *cobra.Command
	apikeysRotateCmd *cobra.Command
)

// Flags
var (
	apiKeyName      string
	apiKeyAbilities []string
	apiKeyExpires   string
)

func init() {
	// List command
	apikeysListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		Long:  `List all API keys for the current organization.`,
		RunE:  runAPIKeysList,
	}

	// Create command
	apikeysCreateCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new API key",
		Long: `Create a new API key with specified abilities (permissions).

Available abilities:
  - secrets:read    - Read secrets
  - secrets:write   - Create/update/delete secrets
  - projects:read   - List projects
  - projects:write  - Create/update/delete projects
  - targets:read    - List targets
  - targets:write   - Create/update/delete targets
  - environments:read  - List environments
  - environments:write - Create/update/delete environments
  - encryption:read    - Read encryption keys
  - encryption:write   - Rotate encryption keys
  - apikeys:read      - List API keys
  - apikeys:write     - Create/update/delete API keys`,
		Args: cobra.ExactArgs(1),
		RunE: runAPIKeysCreate,
	}
	apikeysCreateCmd.Flags().StringSliceVar(&apiKeyAbilities, "abilities", []string{},
		"comma-separated list of abilities (required)")
	apikeysCreateCmd.Flags().StringVar(&apiKeyExpires, "expires", "",
		"expiration date (RFC3339 format, e.g. 2024-12-31T23:59:59Z)")
	apikeysCreateCmd.MarkFlagRequired("abilities")

	// Update command
	apikeysUpdateCmd = &cobra.Command{
		Use:   "update [key-id]",
		Short: "Update an API key",
		Long:  `Update an API key's name.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runAPIKeysUpdate,
	}
	apikeysUpdateCmd.Flags().StringVar(&apiKeyName, "name", "",
		"new name for the API key (required)")
	apikeysUpdateCmd.MarkFlagRequired("name")

	// Delete command
	apikeysDeleteCmd = &cobra.Command{
		Use:   "delete [key-id]",
		Short: "Delete an API key",
		Long:  `Delete an API key. This action is irreversible.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runAPIKeysDelete,
	}

	// Rotate command
	apikeysRotateCmd = &cobra.Command{
		Use:   "rotate [key-id]",
		Short: "Rotate an API key",
		Long: `Rotate an API key to generate a new token while keeping the same
permissions and configuration. The old token will be immediately invalidated.`,
		Args: cobra.ExactArgs(1),
		RunE: runAPIKeysRotate,
	}

	// Add subcommands
	apikeysCmd.AddCommand(apikeysListCmd)
	apikeysCmd.AddCommand(apikeysCreateCmd)
	apikeysCmd.AddCommand(apikeysUpdateCmd)
	apikeysCmd.AddCommand(apikeysDeleteCmd)
	apikeysCmd.AddCommand(apikeysRotateCmd)
}

func runAPIKeysList(cmd *cobra.Command, args []string) error {
	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization from config
	org := viper.GetString("organization")
	if org == "" {
		return fmt.Errorf("organization not set in configuration")
	}

	ui.PrintInfo("Listing API keys for organization %s...", org)

	// List API keys
	keys, _, err := client.APIKeys.List(cmd.Context(), org)
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(keys) == 0 {
		ui.PrintWarning("No API keys found")
		return nil
	}

	// Display in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTOKEN PREFIX\tABILITIES\tLAST USED\tCREATED")
	fmt.Fprintln(w, "──\t────\t────────────\t─────────\t─────────\t───────")

	for _, key := range keys {
		lastUsed := "Never"
		if key.LastUsedAt != nil {
			lastUsed = key.LastUsedAt.Format("2006-01-02")
		}

		abilities := strings.Join(key.Abilities, ", ")
		if len(abilities) > 40 {
			abilities = abilities[:37] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s...\t%s\t%s\t%s\n",
			key.ID,
			key.Name,
			key.TokenPrefix,
			abilities,
			lastUsed,
			key.CreatedAt.Format("2006-01-02"),
		)
	}
	w.Flush()

	return nil
}

func runAPIKeysCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization from config
	org := viper.GetString("organization")
	if org == "" {
		return fmt.Errorf("organization not set in configuration")
	}

	// Parse expiration if provided
	var expiresAt *time.Time
	if apiKeyExpires != "" {
		t, err := time.Parse(time.RFC3339, apiKeyExpires)
		if err != nil {
			return fmt.Errorf("invalid expiration date format: %w", err)
		}
		expiresAt = &t
	}

	ui.PrintInfo("Creating API key '%s' with abilities: %s", name, strings.Join(apiKeyAbilities, ", "))

	// Create request
	createReq := dotenv.APIKeyCreateRequest{
		Name:      name,
		Abilities: apiKeyAbilities,
		ExpiresAt: expiresAt,
	}

	// Create API key
	resp, _, err := client.APIKeys.Create(cmd.Context(), org, createReq)
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccess("API key created successfully!")
	fmt.Println()
	ui.PrintWarning("IMPORTANT: Save this token - it will not be shown again!")
	fmt.Println()
	fmt.Printf("ID:    %s\n", resp.ID)
	fmt.Printf("Name:  %s\n", resp.Name)
	fmt.Printf("Token: %s\n", resp.Token)
	fmt.Println()
	ui.PrintInfo("To use this API key:")
	fmt.Println("  export DOTENV_API_KEY=" + resp.Token)
	fmt.Println("  # or")
	fmt.Println("  dotenv --api-key=" + resp.Token + " pull myproject")

	return nil
}

func runAPIKeysUpdate(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization from config
	org := viper.GetString("organization")
	if org == "" {
		return fmt.Errorf("organization not set in configuration")
	}

	ui.PrintInfo("Updating API key %s...", keyID)

	// Update request
	updateReq := dotenv.APIKeyUpdateRequest{
		Name: apiKeyName,
	}

	// Update API key
	key, _, err := client.APIKeys.Update(cmd.Context(), org, keyID, updateReq)
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccess("API key updated successfully!")
	fmt.Printf("ID:   %s\n", key.ID)
	fmt.Printf("Name: %s\n", key.Name)

	return nil
}

func runAPIKeysDelete(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization from config
	org := viper.GetString("organization")
	if org == "" {
		return fmt.Errorf("organization not set in configuration")
	}

	// Confirm deletion
	confirmed, err := ui.Confirm(fmt.Sprintf("Are you sure you want to delete API key %s? This action cannot be undone.", keyID), false)
	if err != nil {
		return err
	}
	if !confirmed {
		ui.PrintInfo("Deletion cancelled")
		return nil
	}

	ui.PrintInfo("Deleting API key %s...", keyID)

	// Delete API key
	_, err = client.APIKeys.Delete(cmd.Context(), org, keyID)
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccess("API key deleted successfully!")

	return nil
}

func runAPIKeysRotate(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization from config
	org := viper.GetString("organization")
	if org == "" {
		return fmt.Errorf("organization not set in configuration")
	}

	// Confirm rotation
	confirmed, err := ui.Confirm(fmt.Sprintf("Are you sure you want to rotate API key %s? The old token will be immediately invalidated.", keyID), false)
	if err != nil {
		return err
	}
	if !confirmed {
		ui.PrintInfo("Rotation cancelled")
		return nil
	}

	ui.PrintInfo("Rotating API key %s...", keyID)

	// Rotate API key
	resp, _, err := client.APIKeys.Rotate(cmd.Context(), org, keyID)
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccess("API key rotated successfully!")
	fmt.Println()
	ui.PrintWarning("IMPORTANT: Save this new token - it will not be shown again!")
	ui.PrintWarning("The old token has been invalidated.")
	fmt.Println()
	fmt.Printf("ID:    %s\n", resp.ID)
	fmt.Printf("Name:  %s\n", resp.Name)
	fmt.Printf("Token: %s\n", resp.Token)
	fmt.Println()
	ui.PrintInfo("To use this API key:")
	fmt.Println("  export DOTENV_API_KEY=" + resp.Token)
	fmt.Println("  # or")
	fmt.Println("  dotenv --api-key=" + resp.Token + " pull myproject")

	return nil
}
