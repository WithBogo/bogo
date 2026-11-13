package main

import (
	"fmt"
	"os"
	"strings"
)

const usage = `Usage: go run . <module-name> <sql-schema-file> [--db=postgres|mysql|sqlite]

Flags:
  --db=<name>   Database to generate code for (default: postgres)

Example: go run . user-service schema.sql --db=mysql`

// parseArgs splits command line arguments into positional arguments and the selected dialect.
// Flags are accepted anywhere, so both "svc schema.sql --db=mysql" and "--db=mysql svc schema.sql" work.
func parseArgs(args []string) (positional []string, selected Dialect, err error) {
	selected = dialects["postgres"]

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--db="):
			selected, err = lookupDialect(strings.TrimPrefix(arg, "--db="))
			if err != nil {
				return nil, selected, err
			}
		case strings.HasPrefix(arg, "--"):
			return nil, selected, fmt.Errorf("unknown flag %s", arg)
		default:
			positional = append(positional, arg)
		}
	}
	return positional, selected, nil
}

func main() {
	positional, selected, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Printf("Error: %v\n\n%s\n", err, usage)
		os.Exit(1)
	}
	if len(positional) != 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	moduleName := positional[0]
	sqlSchemaFile := positional[1]
	dialect = selected

	fmt.Printf("Creating hexagonal architecture for module: %s\n", moduleName)
	fmt.Printf("Using SQL schema from: %s\n", sqlSchemaFile)
	fmt.Printf("Database: %s\n", dialect.Display)

	err = createHexagonalArchitecture(moduleName, sqlSchemaFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created %s!\n", moduleName)
}
