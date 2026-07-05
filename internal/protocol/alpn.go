package protocol

import "strings"

var defaultTLSALPN = []string{"h2", "http/1.1"}

func inboundALPN(params *InboundParams, fallback []string) []string {
	if params != nil && len(params.ALPN) > 0 {
		return cloneStrings(params.ALPN)
	}
	return cloneStrings(fallback)
}

func serverInfoALPN(info *ServerInfo, fallback []string) []string {
	if info != nil && len(info.ALPN) > 0 {
		return cloneStrings(info.ALPN)
	}
	return cloneStrings(fallback)
}

func alpnQueryValue(alpn []string) string {
	return strings.Join(alpn, ",")
}

func inboundTLSALPN(params *InboundParams) []string {
	return tlsCompatibleALPN(inboundALPN(params, defaultTLSALPN))
}

func serverInfoTLSALPN(info *ServerInfo) []string {
	return tlsCompatibleALPN(serverInfoALPN(info, defaultTLSALPN))
}

func tlsCompatibleALPN(values []string) []string {
	filtered := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "h3" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		filtered = append(filtered, value)
	}
	if len(filtered) == 0 {
		return cloneStrings(defaultTLSALPN)
	}
	return filtered
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}
