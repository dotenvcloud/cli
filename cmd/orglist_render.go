package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"

	dotenv "github.com/dotenvcloud/sdk-go"
)

// currentOrgULIDFromContext returns the ULID of the active organization for
// the current account, or empty string if none.
func currentOrgULIDFromContext() string {
	account := accountForErrorContext()
	if account == nil {
		return ""
	}
	if account.IsOAuth() && account.CurrentOrganization != "" {
		return account.CurrentOrganization
	}
	if account.Organization != nil {
		return account.Organization.ULID
	}
	return ""
}

// renderOrgList renders a list of organizations in the requested format
// (json/yaml/table). This is shared by `dotenv list organizations` and
// `dotenv org list` to avoid duplicated rendering logic.
func renderOrgList(orgs []*dotenv.Organization, format string) error {
	switch format {
	case formatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(orgs)

	case formatYAML:
		return renderOrgListYAML(orgs)

	default:
		return renderOrgListTable(orgs)
	}
}

func renderOrgListYAML(orgs []*dotenv.Organization) error {
	currentOrgID := currentOrgULIDFromContext()
	for _, org := range orgs {
		// Use ID if ULID is empty (API might return ULID in ID field)
		ulid := org.ULID
		if ulid == "" && org.ID != "" {
			ulid = org.ID
		}

		fmt.Printf("- name: %s\n", org.Name)
		fmt.Printf("  ulid: %s\n", ulid)
		fmt.Printf("  plan: %s\n", org.PlanName)
		fmt.Printf("  status: %s\n", org.Status)
		if org.ID == currentOrgID || org.ULID == currentOrgID {
			fmt.Printf("  current: true\n")
		}
		fmt.Println()
	}
	return nil
}

func renderOrgListTable(orgs []*dotenv.Organization) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "ULID", "PLAN", "STATUS", "CURRENT")

	currentOrgID := currentOrgULIDFromContext()
	for _, org := range orgs {
		current := ""
		if org.ID == currentOrgID || org.ULID == currentOrgID {
			current = "*"
		}

		ulid := org.ULID
		if ulid == "" && org.ID != "" {
			ulid = org.ID
		}

		if err := table.Append([]string{
			org.Name,
			ulid,
			org.PlanName,
			org.Status,
			current,
		}); err != nil {
			return fmt.Errorf("append org row: %w", err)
		}
	}

	if err := table.Render(); err != nil {
		return fmt.Errorf("render org table: %w", err)
	}
	return nil
}
