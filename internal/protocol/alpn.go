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

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}
