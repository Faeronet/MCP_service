package main

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logIPAllow = logging.New("admin-backend.ipallow")

type ipAllowlist struct {
	enabled bool
	nets    []*net.IPNet
	ips     []net.IP
}

func parseIPAllowlist(raw string) *ipAllowlist {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &ipAllowlist{}
	}
	list := &ipAllowlist{enabled: true}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, n, err := net.ParseCIDR(part)
			if err != nil {
				logIPAllow.Warn(context.Background(), "skip invalid CIDR in ADMIN_ALLOWED_IPS", logging.KV{"cidr", part})
				continue
			}
			list.nets = append(list.nets, n)
			continue
		}
		ip := net.ParseIP(part)
		if ip == nil {
			logIPAllow.Warn(context.Background(), "skip invalid IP in ADMIN_ALLOWED_IPS", logging.KV{"ip", part})
			continue
		}
		list.ips = append(list.ips, ip)
	}
	if len(list.nets) == 0 && len(list.ips) == 0 {
		return &ipAllowlist{}
	}
	return list
}

func loadIPAllowlist() *ipAllowlist {
	return parseIPAllowlist(config.LoadString("ADMIN_ALLOWED_IPS", ""))
}

func isPrivateOrLoopback(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 10 {
		return true
	}
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

func clientIP(r *http.Request) net.IP {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteHost)

	// Заголовки доверяем только от reverse-proxy (nginx admin-web-ui) во внутренней сети.
	if isPrivateOrLoopback(remoteIP) {
		if x := strings.TrimSpace(r.Header.Get("X-Real-IP")); x != "" {
			if ip := net.ParseIP(x); ip != nil {
				return ip
			}
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip
			}
		}
	}
	return remoteIP
}

func (a *ipAllowlist) allowed(ip net.IP) bool {
	if !a.enabled {
		return true
	}
	if ip == nil {
		return false
	}
	for _, x := range a.ips {
		if x.Equal(ip) {
			return true
		}
	}
	for _, n := range a.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func ipAllowMiddleware(list *ipAllowlist, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		if !list.allowed(ip) {
			logIPAllow.Warn(r.Context(), "admin access denied by IP allowlist", logging.KV{"ip", ip.String()}, logging.KV{"path", r.URL.Path})
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"forbidden","hint":"your IP is not allowed to access admin"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
