// Package main is the LLMHub operations CLI.
//
// Provides administrative commands: account onboarding, credential
// rotation, manual reconciliation runs, and database seeding.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("llmhub-cli v0.1.0")
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`llmhub-cli - LLMHub operations tool

Usage:
  llmhub-cli <command> [flags]

Commands:
  version            Print the CLI version
  help               Show this help

Planned (TODO):
  pool add           Register an upstream account into the pool
  pool rotate-key    Rotate an account credential in Vault
  recon run          Trigger daily reconciliation for a provider
  seed dev           Load development seed data`)
}
