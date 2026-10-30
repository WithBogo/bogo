package main

import (
	"fmt"
	"os"
	"strings"
)

const usage = `Usage: go run . <module-name> <sql-schema-file> [--dry-run] [--force]

Flags:
  --dry-run   Show which files would be created, updated or skipped; write nothing
  --force     Overwrite files even if they were modified after generation

Example: go run . user-service schema.sql --dry-run`

// parseArgs splits command line arguments into positional arguments and options.
// Flags are accepted anywhere, so both "svc schema.sql --dry-run" and "--dry-run svc schema.sql" work.
func parseArgs(args []string) (positional []string, opts genOptions, err error) {
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--force":
			opts.Force = true
		case strings.HasPrefix(arg, "--"):
			return nil, opts, fmt.Errorf("unknown flag %s", arg)
		default:
			positional = append(positional, arg)
		}
	}
	return positional, opts, nil
}

func main() {
	positional, opts, err := parseArgs(os.Args[1:])
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

	fmt.Printf("Creating hexagonal architecture for module: %s\n", moduleName)
	fmt.Printf("Using SQL schema from: %s\n", sqlSchemaFile)

	err = createHexagonalArchitecture(moduleName, sqlSchemaFile, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if opts.DryRun {
		fmt.Printf("Dry run finished for %s, nothing was written.\n", moduleName)
		return
	}
	fmt.Printf("Successfully created %s!\n", moduleName)
}
