package main

// Keycloak — только клиентская интеграция: проверка access token по JWKS и обмен authorization code (PKCE).
// Сам Keycloak и БД разворачиваются отдельно (другая ВМ/сервис); в admin-zone образ Keycloak не входит.
//
// Минимум: KEYCLOAK_ISSUER (как claim iss в JWT), KEYCLOAK_CLIENT_ID (public client в Keycloak).
// Если из контейнера admin-backend недоступен тот же host, что у браузера в iss: задать
// KEYCLOAK_JWKS_URL, KEYCLOAK_TOKEN_URL, KEYCLOAK_AUTHORIZATION_URL.
// Дополнительно: KEYCLOAK_CLIENT_SECRET, KEYCLOAK_REQUIRED_ROLE (realm role), KEYCLOAK_ONLY=1.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logKC = logging.New("admin-backend.keycloak")

func normalizeKeycloakIssuer(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

type oidcDiscoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

func fetchOIDCDiscovery(ctx context.Context, issuer string) (*oidcDiscoveryDoc, error) {
	iss := normalizeKeycloakIssuer(issuer)
	if iss == "" {
		return nil, fmt.Errorf("empty issuer")
	}
	u := iss + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("discovery %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var doc oidcDiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.TokenEndpoint == "" || doc.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("discovery missing endpoints")
	}
	return &doc, nil
}

func keycloakCheckClaims(claims jwt.MapClaims, issuer, clientID, requiredRole string) error {
	iss, _ := claims["iss"].(string)
	if normalizeKeycloakIssuer(iss) != normalizeKeycloakIssuer(issuer) {
		return fmt.Errorf("iss mismatch")
	}
	if clientID != "" {
		ok := false
		if azp, _ := claims["azp"].(string); azp == clientID {
			ok = true
		}
		if !ok {
			switch aud := claims["aud"].(type) {
			case string:
				if aud == clientID {
					ok = true
				}
			case []interface{}:
				for _, x := range aud {
					if s, _ := x.(string); s == clientID {
						ok = true
						break
					}
				}
			}
		}
		if !ok {
			return fmt.Errorf("aud/azp mismatch")
		}
	}
	if requiredRole != "" {
		ra, ok := claims["realm_access"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("realm_access missing")
		}
		roles, _ := ra["roles"].([]interface{})
		found := false
		for _, r := range roles {
			if s, ok := r.(string); ok && s == requiredRole {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing realm role %q", requiredRole)
		}
	}
	return nil
}

func newKeycloakKeyfunc(ctx context.Context, jwksURL string) (keyfunc.Keyfunc, error) {
	u := strings.TrimSpace(jwksURL)
	if u == "" {
		return nil, fmt.Errorf("empty jwks url")
	}
	return keyfunc.NewDefaultCtx(ctx, []string{u})
}

func (h *Handler) validateKeycloakAccessToken(tokenStr string) error {
	if h.KeycloakKeyfunc == nil {
		return fmt.Errorf("keycloak jwks not configured")
	}
	token, err := jwt.Parse(tokenStr, h.KeycloakKeyfunc.Keyfunc, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "PS256"}))
	if err != nil || token == nil || !token.Valid {
		return err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("invalid claims type")
	}
	return keycloakCheckClaims(claims, h.KeycloakIssuer, h.KeycloakClientID, h.KeycloakRequiredRole)
}

func (h *Handler) validateHS256AdminToken(tokenStr string) error {
	_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return h.JWTSecret, nil
	})
	return err
}

func (h *Handler) validateBearerAccessToken(tokenStr string) error {
	if h.KeycloakKeyfunc != nil {
		if err := h.validateKeycloakAccessToken(tokenStr); err == nil {
			return nil
		}
	}
	if h.KeycloakOnly {
		return fmt.Errorf("keycloak only: invalid keycloak token")
	}
	return h.validateHS256AdminToken(tokenStr)
}

func bearerFromRequest(r *http.Request) string {
	tokenStr := r.Header.Get("Authorization")
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		return tokenStr[7:]
	}
	if tokenStr != "" {
		return tokenStr
	}
	return ""
}

// AuthMiddleware проверяет Bearer: JWT Keycloak (RS256/JWKS) или локальный HS256.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := bearerFromRequest(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}
		if err := h.validateBearerAccessToken(tokenStr); err != nil {
			logKC.Warn(r.Context(), "auth failed", logging.KV{"error", err.Error()})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			body := `{"error":"invalid token"}`
			if errors.Is(err, jwt.ErrTokenExpired) {
				body = `{"error":"token_expired"}`
			}
			_, _ = w.Write([]byte(body))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GrafanaAuthMiddleware — как AuthMiddleware, плюс token в query для iframe.
func (h *Handler) GrafanaAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/grafana/public/") || strings.HasPrefix(r.URL.Path, "/api/grafana/img/") {
			next.ServeHTTP(w, r)
			return
		}
		tokenStr := bearerFromRequest(r)
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}
		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing authorization","hint":"Open Grafana from the admin panel (log in first, then open Grafana in the menu)"}`))
			return
		}
		if err := h.validateBearerAccessToken(tokenStr); err != nil {
			logKC.Warn(r.Context(), "grafana auth failed", logging.KV{"error", err.Error()})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			body := `{"error":"invalid token"}`
			if errors.Is(err, jwt.ErrTokenExpired) {
				body = `{"error":"token_expired"}`
			}
			_, _ = w.Write([]byte(body))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// KeycloakPublicConfig GET /api/auth/keycloak — публичные параметры для SPA (без секретов).
func (h *Handler) KeycloakPublicConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if normalizeKeycloakIssuer(h.KeycloakIssuer) == "" || strings.TrimSpace(h.KeycloakClientID) == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	authEp := strings.TrimSpace(h.KeycloakAuthorizationURL)
	if authEp == "" {
		doc, err := fetchOIDCDiscovery(ctx, h.KeycloakIssuer)
		if err != nil {
			logKC.Warn(ctx, "keycloak discovery", logging.KV{"error", err})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
				"error":   "discovery_failed",
			})
			return
		}
		authEp = doc.AuthorizationEndpoint
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":                   true,
		"issuer":                    normalizeKeycloakIssuer(h.KeycloakIssuer),
		"client_id":                 h.KeycloakClientID,
		"authorization_endpoint":    authEp,
		"password_login_disabled":   h.KeycloakOnly,
	})
}

type keycloakCallbackBody struct {
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
}

// KeycloakCallback POST /api/auth/keycloak/callback — обмен code+PKCE на access_token (обход CORS token endpoint).
func (h *Handler) KeycloakCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	if normalizeKeycloakIssuer(h.KeycloakIssuer) == "" || strings.TrimSpace(h.KeycloakClientID) == "" {
		http.Error(w, `{"error":"keycloak disabled"}`, http.StatusNotFound)
		return
	}
	var body keycloakCallbackBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Code) == "" || strings.TrimSpace(body.RedirectURI) == "" || strings.TrimSpace(body.CodeVerifier) == "" {
		http.Error(w, `{"error":"code, redirect_uri, code_verifier required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	tokenEp := strings.TrimSpace(h.KeycloakTokenURL)
	if tokenEp == "" {
		doc, err := fetchOIDCDiscovery(ctx, h.KeycloakIssuer)
		if err != nil {
			logKC.Warn(ctx, "keycloak discovery", logging.KV{"error", err})
			http.Error(w, `{"error":"discovery"}`, http.StatusBadGateway)
			return
		}
		tokenEp = doc.TokenEndpoint
	}
	if tokenEp == "" {
		http.Error(w, `{"error":"no token endpoint"}`, http.StatusBadGateway)
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", h.KeycloakClientID)
	if strings.TrimSpace(h.KeycloakClientSecret) != "" {
		form.Set("client_secret", h.KeycloakClientSecret)
	}
	form.Set("code", strings.TrimSpace(body.Code))
	form.Set("redirect_uri", strings.TrimSpace(body.RedirectURI))
	form.Set("code_verifier", strings.TrimSpace(body.CodeVerifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEp, strings.NewReader(form.Encode()))
	if err != nil {
		http.Error(w, `{"error":"request"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logKC.Warn(ctx, "keycloak token request", logging.KV{"error", err})
		http.Error(w, `{"error":"token request"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		logKC.Warn(ctx, "keycloak token error", logging.KV{"status", resp.StatusCode}, logging.KV{"body", string(raw)})
		http.Error(w, fmt.Sprintf(`{"error":"token_exchange","status":%d}`, resp.StatusCode), http.StatusUnauthorized)
		return
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		http.Error(w, `{"error":"token response"}`, http.StatusBadGateway)
		return
	}
	if err := h.validateKeycloakAccessToken(tok.AccessToken); err != nil {
		logKC.Warn(ctx, "keycloak token invalid after exchange", logging.KV{"error", err})
		http.Error(w, `{"error":"invalid_token_claims"}`, http.StatusUnauthorized)
		return
	}

	rd := keycloakReminderDebug(h, tok.AccessToken)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{Token: tok.AccessToken, ReminderDebug: rd})
}

func keycloakReminderDebug(h *Handler, accessToken string) bool {
	mc := make(jwt.MapClaims)
	_, _, err := jwt.NewParser().ParseUnverified(accessToken, mc)
	if err != nil {
		return false
	}
	if jwtClaimBool(mc, "reminder_debug") {
		return true
	}
	if pref, _ := mc["preferred_username"].(string); pref != "" && h.ReminderSuperAdminSub != "" && pref == h.ReminderSuperAdminSub {
		return true
	}
	if sub, _ := mc["sub"].(string); sub != "" && h.ReminderSuperAdminSub != "" && sub == h.ReminderSuperAdminSub {
		return true
	}
	if ra, ok := mc["realm_access"].(map[string]interface{}); ok {
		if roles, _ := ra["roles"].([]interface{}); roles != nil {
			for _, r := range roles {
				if s, _ := r.(string); s == "reminder_debug" {
					return true
				}
			}
		}
	}
	return false
}
