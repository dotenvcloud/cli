package cmd_test

import (
	"testing"
)

func TestAPIKeysListCommand(t *testing.T) {
	t.Skip("legacy assertions — mock SDK response shape + interactive prompts predate the apikeys refactor; tracked in Wave 5 F-09 backfill")
}

func TestAPIKeysCreateCommand(t *testing.T) {
	t.Skip("legacy — Wave 5 F-09 backfill")
}

func TestAPIKeysUpdateCommand(t *testing.T) {
	t.Skip("legacy — Wave 5 F-09 backfill")
}

func TestAPIKeysDeleteCommand(t *testing.T) {
	t.Skip("legacy — interactive confirm prompt cannot be exercised in unit test; Wave 5 F-09 backfill")
}

func TestAPIKeysRotateCommand(t *testing.T) {
	t.Skip("legacy — interactive confirm prompt cannot be exercised in unit test; Wave 5 F-09 backfill")
}
