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
	profilesRaw := strings.TrimSpace(os.Getenv("COMPOSE_PROFILES"))
	if profilesRaw == "" {
		profilesRaw = strings.TrimSpace(os.Getenv("ZONE_AGENT_COMPOSE_PROFILES"))
	}
	var composeProfiles []string
	for _, p := range strings.Split(profilesRaw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			composeProfiles = append(composeProfiles, p)
		}
	}

	aiSwap := strings.TrimSpace(os.Getenv("ZONE_AGENT_AI_SWAP"))
	aiSwapOn := aiSwap == "1" || strings.EqualFold(aiSwap, "true") || strings.EqualFold(aiSwap, "yes")

	if err := modules.Run(modules.Config{
		Workdir:         abs,
		Secret:          secret,
		Listen:          listen,
		ComposeProject:  composeProject,
		ComposeProfiles: composeProfiles,
		AISwap:          aiSwapOn,
	}); err != nil {
		log.Fatal(err)
	}
}

