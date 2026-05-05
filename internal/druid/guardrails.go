package druid

import (
	"fmt"
	"strings"
)

// ValidateReadOnlyQuery ensures the query is a SELECT statement.
func ValidateReadOnlyQuery(query string) error {
	normalized := strings.TrimSpace(strings.ToUpper(query))

	writeKeywords := []string{
		"INSERT", "REPLACE", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
	}

	for _, kw := range writeKeywords {
		if strings.HasPrefix(normalized, kw) {
			return fmt.Errorf(
				"query appears to be a write operation (starts with %q). Use the druid_ingest tool for INSERT/REPLACE queries",
				kw,
			)
		}
	}

	return nil
}

// ValidateIngestQuery ensures the query is an INSERT or REPLACE with PARTITIONED BY.
func ValidateIngestQuery(query string) error {
	normalized := strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(normalized, "INSERT") && !strings.HasPrefix(normalized, "REPLACE") {
		return fmt.Errorf("ingest queries must start with INSERT or REPLACE. Use druid_query for SELECT queries")
	}

	if !strings.Contains(normalized, "PARTITIONED BY") {
		return fmt.Errorf("ingest queries require a PARTITIONED BY clause (e.g., PARTITIONED BY DAY)")
	}

	return nil
}
