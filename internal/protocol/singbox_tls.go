package protocol

import "net"

func usesSelfSignedTLS(info *ServerInfo) bool {
	return info != nil && net.ParseIP(info.Domain) != nil
}

func markClashUDP(proxy map[string]interface{}) {
	if proxy != nil {
		proxy["udp"] = true
	}
}

func markClashSkipCertVerify(proxy map[string]interface{}, info *ServerInfo) {
	if proxy != nil && usesSelfSignedTLS(info) {
		proxy["skip-cert-verify"] = true
	}
}

func markSingBoxTLSInsecure(outbound map[string]interface{}, info *ServerInfo) {
	if outbound == nil || !usesSelfSignedTLS(info) {
		return
	}
	if tls, ok := outbound["tls"].(map[string]interface{}); ok {
		tls["insecure"] = true
	}
}
