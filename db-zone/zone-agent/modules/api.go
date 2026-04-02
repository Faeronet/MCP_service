package modules

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// Run starts the agent HTTP server.
func Run(cfg Config) error {
	cfg.Workdir = strings.TrimSpace(cfg.Workdir)
	if cfg.Workdir == "" {
		return &ConfigError{msg: "ZONE_WORKDIR is required"}
	}
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	if cfg.Secret == "" {
		return &ConfigError{msg: "ZONE_AGENT_SECRET is required"}
	}
	cfg.Listen = strings.TrimSpace(cfg.Listen)
	if cfg.Listen == "" {
		cfg.Listen = "0.0.0.0:19090"
	}

	workdir := cfg.Workdir
	secret := cfg.Secret

	s := &server{
		workdir:        workdir,
		secret:         secret,
		composeProject: strings.TrimSpace(cfg.ComposeProject),
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/meta", s.withAuth(s.handleMeta))
	mux.HandleFunc("/v1/env", s.withAuth(s.handleEnv))
	mux.HandleFunc("/v1/services", s.withAuth(s.handleServices))
	mux.HandleFunc("/v1/rebuild", s.withAuth(s.handleRebuild))

	log.Printf("zone-agent workdir=%s compose_project=%q profiles=%v listen=%s", workdir, s.composeProject, s.composeProfiles, cfg.Listen)

	// Keep the error message stable for logs.
	addr := cfg.Listen
	if strings.HasPrefix(addr, ":") {
		addr = "0.0.0.0" + addr
	}
	// Allow container shutdown signals to stop ListenAndServe.
	return http.ListenAndServe(addr, mux)
}

type ConfigError struct {
	msg string
}

func (e *ConfigError) Error() string {
	return e.msg
}

// If the binary is executed without stdio (rare), keep stderr visible.
func init() {
	_ = os.Stderr
}

