package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// patchHostResolvForBuild replaces systemd-resolved stub nameserver so docker build can reach registry-1.docker.io.
// Host file is edited via bind mount (ZONE_AGENT_HOST_RESOLV_PATH) or a docker run helper (see rebuild flow).
func (s *server) patchHostResolvForBuild(ctx context.Context, logs *strings.Builder) {
	if strings.TrimSpace(os.Getenv("ZONE_AGENT_SKIP_RESOLV_PATCH")) == "1" {
		logs.WriteString("resolv: skip (ZONE_AGENT_SKIP_RESOLV_PATCH=1)\n")
		return
	}

	path := strings.TrimSpace(os.Getenv("ZONE_AGENT_HOST_RESOLV_PATH"))
	if path == "" {
		path = "/host-etc-resolv.conf"
	}

	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		if err := patchResolvFile(path, s.workdir, logs); err != nil {
			logs.WriteString("resolv: file patch: " + err.Error() + "\n")
		}
		return
	}

	if err := patchResolvViaDocker(ctx, logs); err != nil {
		logs.WriteString("resolv: docker helper: " + err.Error() + "\n")
	}
}

func patchResolvFile(hostResolvMount, workdir string, logs *strings.Builder) error {
	rawBytes, err := os.ReadFile(hostResolvMount)
	if err != nil {
		return err
	}
	raw := strings.ReplaceAll(string(rawBytes), "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	var out []string
	changed := false
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "nameserver 127.0.0.53" {
			out = append(out, "nameserver 8.8.8.8", "nameserver 8.8.4.4")
			changed = true
		} else {
			out = append(out, ln)
		}
	}
	if !changed {
		logs.WriteString("resolv: no nameserver 127.0.0.53 in " + hostResolvMount + ", unchanged\n")
		return nil
	}

	backup := filepath.Join(workdir, ".zone-agent-resolv.backup")
	if err := os.WriteFile(backup, rawBytes, 0o644); err == nil {
		logs.WriteString("resolv: backup → " + backup + "\n")
	}

	newContent := strings.Join(out, "\n")
	if strings.HasSuffix(raw, "\n") {
		newContent += "\n"
	}
	if err := os.WriteFile(hostResolvMount, []byte(newContent), 0o644); err != nil {
		return err
	}
	logs.WriteString("resolv: patched " + hostResolvMount + " (127.0.0.53 → 8.8.8.8 / 8.8.4.4)\n")
	return nil
}

func patchResolvViaDocker(ctx context.Context, logs *strings.Builder) error {
	img := strings.TrimSpace(os.Getenv("ZONE_AGENT_RESOLV_PATCH_IMAGE"))
	if img == "" {
		img = "busybox:latest"
	}
	// BusyBox awk: replace stub resolver line with public DNS (build can pull base images).
	script := `set -e
test -f /r || exit 0
grep -qx 'nameserver 127.0.0.53' /r || exit 0
cp /r /r.bak.zoneagent 2>/dev/null || true
awk '/^nameserver 127\.0\.0\.53$/ { print "nameserver 8.8.8.8"; print "nameserver 8.8.4.4"; next } { print }' /r > /tmp/r.new && cat /tmp/r.new > /r
`
	out, err := runCmd(ctx, "docker", "run", "--rm", "--pull=never", "-v", "/etc/resolv.conf:/r:rw", img, "sh", "-c", script)
	if len(out) > 0 {
		_, _ = logs.Write(out)
	}
	return err
}
