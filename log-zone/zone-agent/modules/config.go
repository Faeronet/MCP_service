package modules

// Config defines how zone-agent should access the zone directory and authorization.
type Config struct {
	// Workdir is an absolute path to the zone directory (where docker-compose.yml and .env live).
	Workdir string
	// Secret is shared with admin-backend via X-Zone-Agent-Secret header.
	Secret string
	// Listen is host:port to bind HTTP server.
	Listen string
	// ComposeProject is docker compose -p (e.g. COMPOSE_PROJECT_NAME). Required when workdir is /zone — otherwise every zone shares project name "zone".
	ComposeProject string
	// ComposeProfiles is docker compose --profile for each entry (from COMPOSE_PROFILES or ZONE_AGENT_COMPOSE_PROFILES, comma-separated).
	ComposeProfiles []string
}

