package main

import (
	"fmt"
	"os"

	"github.com/nuchs/tasker/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tracker: %v\n", err)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		err = cli.RunInit(wd, args)
	case "create":
		err = cli.RunCreate(wd, args, os.Stdout)
	case "show":
		err = cli.RunShow(wd, args, os.Stdout)
	case "list":
		err = cli.RunList(wd, args, os.Stdout)
	case "ready":
		err = cli.RunReady(wd, args, os.Stdout)
	case "update":
		err = cli.RunUpdate(wd, args, os.Stdout)
	case "comment":
		err = cli.RunComment(wd, args, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "tracker: unknown command %q\n\n", cmd)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "tracker: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `tracker — local issue tracker for AI agents

Usage:
  tracker init --prefix PROJ
  tracker create --title "..." --description "..." [--priority high] [--type issue]
  tracker show <id> [--events] [--json]
  tracker list [--status open] [--priority high] [--type task] [--json]
  tracker ready [--json]
  tracker update <id> --field value [--field value ...]
  tracker comment <id> "message"
  tracker claim <id> --agent <agent-id> --session <session-id>
  tracker release <id>
  tracker rebuild`)
}
