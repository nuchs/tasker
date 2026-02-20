package main

import (
	"fmt"
	"os"
)

func main() {
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
	os.Exit(1)
}
