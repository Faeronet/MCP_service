package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"zone-agent/modules"
)

func main() {
	workdir := strings.TrimSpace(os.Getenv("ZONE_WORKDIR"))
	if workdir == "" {
		log.Fatal("ZONE_WORKDIR is required (absolute or bind-mounted path to zone directory)")
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		log.Fatalf("ZONE_WORKDIR abs: %v", err)
	}

	secret := strings.TrimSpace(os.Getenv("ZONE_AGENT_SECRET"))
	if secret == "" {
		secret = "change-me-in-production"
	}

	listen := strings.TrimSpace(os.Getenv("LISTEN"))
	if listen == "" {
		listen = "0.0.0.0:19090"
	}

	composeProject := strings.TrimSpace(os.Getenv("COMPOSE_PROJECT_NAME"))

	if err := modules.Run(modules.Config{
		Workdir:        abs,
		Secret:         secret,
		Listen:         listen,
		ComposeProject: composeProject,
	}); err != nil {
		log.Fatal(err)
	}
}

