package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"vasmax/internal/config"
)

// GenerateInboundMessages expands protocols that need multiple runtime
// inbounds. Most protocols keep one inbound; Reality Vision can expose one
// inbound per target so clients can switch away from a broken camouflage target.
func GenerateInboundMessages(p Protocol, params *InboundParams) ([]json.RawMessage, error) {
	if p == nil {
		return nil, fmt.Errorf("protocol is nil")
	}
	if params == nil {
		return nil, fmt.Errorf("inbound params are nil")
	}
	if p.Name() != "vless_reality_vision" || params.Reality == nil || len(params.Reality.Targets) == 0 {
		raw, err := p.GenerateInbound(params)
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{raw}, nil
	}

	targets := params.Reality.EffectiveTargets(params.Port)
	if len(targets) == 0 {
		raw, err := p.GenerateInbound(params)
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{raw}, nil
	}

	inbounds := make([]json.RawMessage, 0, len(targets))
	for _, target := range targets {
		next := *params
		reality := realityForTarget(params.Reality, target)
		next.Reality = &reality
		next.Port = target.Port
		next.Tag = fmt.Sprintf("%s_%s", p.Name(), config.RealityTargetName(target.ServerName))

		raw, err := p.GenerateInbound(&next)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, raw)
	}
	return inbounds, nil
}

func realityForTarget(base *config.RealityConfig, target config.RealityTarget) config.RealityConfig {
	reality := *base
	reality.ServerName = target.ServerName
	reality.Dest = target.Dest
	reality.Port = target.Port
	reality.Targets = nil
	return reality
}

func RealitySubscriptionName(host string, target config.RealityTarget, suffix string) string {
	name := config.RealityTargetName(target.ServerName)
	if strings.TrimSpace(name) == "" {
		name = "target"
	}
	if host == "" {
		return fmt.Sprintf("reality-%s", name)
	}
	if suffix == "" {
		return fmt.Sprintf("%s-reality-%s", host, name)
	}
	return fmt.Sprintf("%s-%s-%s", host, suffix, name)
}
