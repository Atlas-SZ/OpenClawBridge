package gatewayclient

import "strings"

const (
	gatewayClientID          = "clawbridge-connector"
	gatewayClientDisplayName = "ClawBridge Connector"
	gatewayClientVersion     = "2.0.0"
	gatewayClientPlatform    = "linux"
	gatewayClientMode        = "operator"
)

func normalizeScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	if len(out) == 0 {
		return []string{"operator.read", "operator.write"}
	}
	return out
}
