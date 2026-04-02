package modules

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// pruneDockerBuildCacheBeforeResolv сбрасывает кэш Docker BuildKit перед сменой DNS, чтобы сборки заново резолвили реестр.
// Отключить: ZONE_AGENT_SKIP_DOCKER_BUILDER_PRUNE=1
func (s *server) pruneDockerBuildCacheBeforeResolv(ctx context.Context, logs *strings.Builder) {
	if strings.TrimSpace(os.Getenv("ZONE_AGENT_SKIP_DOCKER_BUILDER_PRUNE")) == "1" {
		logs.WriteString("docker builder prune: skip (ZONE_AGENT_SKIP_DOCKER_BUILDER_PRUNE=1)\n")
		return
	}
	out, err := runCmd(ctx, "docker", "builder", "prune", "-af")
	if len(out) > 0 {
		_, _ = logs.Write(out)
		if out[len(out)-1] != '\n' {
			logs.WriteByte('\n')
		}
	}
	if err != nil {
		logs.WriteString("docker builder prune: " + err.Error() + "\n")
		return
	}
	logs.WriteString("docker builder prune: ok\n")
}

// patchHostResolvForBuild edits the host's /etc/resolv.conf (bind-mount that path in compose: /etc/resolv.conf:/etc/resolv.conf:rw).
// Fixes Docker build/BuildKit resolving registry-1.docker.io: loopback stubs (127.0.0.53, 127.0.0.11, ::1), tabs/spaces, optional prepend.
func (s *server) patchHostResolvForBuild(ctx context.Context, logs *strings.Builder) {
	if strings.TrimSpace(os.Getenv("ZONE_AGENT_SKIP_RESOLV_PATCH")) == "1" {
		logs.WriteString("resolv: skip (ZONE_AGENT_SKIP_RESOLV_PATCH=1)\n")
		return
	}

	s.pruneDockerBuildCacheBeforeResolv(ctx, logs)

	path := strings.TrimSpace(os.Getenv("ZONE_AGENT_HOST_RESOLV_PATH"))
	if path == "" {
		path = "/etc/resolv.conf"
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

func nameserverAddrField(line string) (addr string, ok bool) {
	f := strings.Fields(line)
	if len(f) < 2 || strings.ToLower(f[0]) != "nameserver" {
		return "", false
	}
	addr = f[1]
	if i := strings.IndexByte(addr, '%'); i >= 0 {
		addr = addr[:i]
	}
	return addr, true
}

func isLoopbackNameserverLine(line string) bool {
	addr, ok := nameserverAddrField(line)
	if !ok {
		return false
	}
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}

func firstNameserverIndex(lines []string) int {
	for i, ln := range lines {
		f := strings.Fields(strings.TrimSpace(ln))
		if len(f) > 0 && strings.ToLower(f[0]) == "nameserver" {
			return i
		}
	}
	return -1
}

func firstNameserverIs8888(lines []string) bool {
	idx := firstNameserverIndex(lines)
	if idx < 0 {
		return false
	}
	addr, ok := nameserverAddrField(lines[idx])
	return ok && addr == "8.8.8.8"
}

func publicDNSLines() []string {
	return []string{"nameserver 8.8.8.8", "nameserver 8.8.4.4"}
}

func patchResolvFile(hostResolvMount, workdir string, logs *strings.Builder) error {
	rawBytes, err := os.ReadFile(hostResolvMount)
	if err != nil {
		return err
	}
	raw := strings.ReplaceAll(string(rawBytes), "\r\n", "\n")
	lines := strings.Split(raw, "\n")

	var out []string
	removedLoopback := false
	for _, ln := range lines {
		if isLoopbackNameserverLine(ln) {
			removedLoopback = true
			continue
		}
		out = append(out, ln)
	}

	fallbackPrepend := strings.TrimSpace(os.Getenv("ZONE_AGENT_RESOLV_FALLBACK_PREPEND")) != "0"

	if removedLoopback {
		if !firstNameserverIs8888(out) {
			idx := firstNameserverIndex(out)
			pub := publicDNSLines()
			if idx < 0 {
				out = append(out, pub...)
			} else {
				out = append(out[:idx], append(pub, out[idx:]...)...)
			}
			logs.WriteString("resolv: removed loopback stub nameserver(s), prepended 8.8.8.8 / 8.8.4.4\n")
		} else {
			logs.WriteString("resolv: removed loopback stub nameserver(s); first nameserver already 8.8.8.8\n")
		}
	} else if fallbackPrepend && !firstNameserverIs8888(out) {
		idx := firstNameserverIndex(out)
		pub := publicDNSLines()
		if idx < 0 {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			out = append(out, pub...)
			logs.WriteString("resolv: no loopback line in file; appended public DNS (FALLBACK_PREPEND)\n")
		} else {
			out = append(out[:idx], append(pub, out[idx:]...)...)
			logs.WriteString("resolv: prepended public DNS before first nameserver (FALLBACK_PREPEND; BuildKit still used 127.0.0.53)\n")
		}
	} else {
		logs.WriteString("resolv: host /etc/resolv.conf (" + hostResolvMount + ") unchanged (no loopback nameserver; fallback prepend off or 8.8.8.8 already first)\n")
		return nil
	}

	newContent := strings.Join(out, "\n")
	if strings.HasSuffix(raw, "\n") {
		newContent += "\n"
	}
	if newContent == raw {
		logs.WriteString("resolv: no effective change after processing\n")
		return nil
	}

	backup := filepath.Join(workdir, ".zone-agent-resolv.backup")
	if err := os.WriteFile(backup, rawBytes, 0o644); err == nil {
		logs.WriteString("resolv: backup → " + backup + "\n")
	}

	if err := os.WriteFile(hostResolvMount, []byte(newContent), 0o644); err != nil {
		return err
	}
	logs.WriteString("resolv: wrote host /etc/resolv.conf (container path " + hostResolvMount + ")\n")
	return nil
}

func patchResolvViaDocker(ctx context.Context, logs *strings.Builder) error {
	img := strings.TrimSpace(os.Getenv("ZONE_AGENT_RESOLV_PATCH_IMAGE"))
	if img == "" {
		img = "busybox:latest"
	}
	// Drop loopback nameservers; emit public DNS once before first remaining nameserver (BusyBox awk).
	script := `set -e
test -f /r || exit 0
cp /r /r.bak.zoneagent 2>/dev/null || true
awk 'BEGIN{done=0}
$1=="nameserver" && ($2 ~ /^127\./ || $2=="::1") {
  if (!done) { print "nameserver 8.8.8.8"; print "nameserver 8.8.4.4"; done=1 }
  next
}
{ print }
' /r > /tmp/r.new && cat /tmp/r.new > /r
`
	out, err := runCmd(ctx, "docker", "run", "--rm", "--pull=never", "-v", "/etc/resolv.conf:/r:rw", img, "sh", "-c", script)
	if len(out) > 0 {
		_, _ = logs.Write(out)
	}
	return err
}
