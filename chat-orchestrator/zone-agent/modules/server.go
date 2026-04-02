package modules

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type server struct {
	workdir string
	secret  string
}

func (s *server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("X-Zone-Agent-Secret")) != s.secret {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (s *server) envPath() string {
	return filepath.Join(s.workdir, ".env")
}

func (s *server) composeFile() (string, error) {
	for _, name := range []string{"docker-compose.yml", "compose.yml"} {
		p := filepath.Join(s.workdir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("no docker-compose.yml or compose.yml in zone")
}

func (s *server) dockerComposeBaseArgs(composePath string) []string {
	// Always run against the mounted zone directory (not container cwd).
	return []string{"compose", "--project-directory", s.workdir, "-f", composePath}
}

