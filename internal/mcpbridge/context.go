package mcpbridge

import (
	"errors"
	"strings"
)

const (
	EnvReceiveIDType   = "ALICE_MCP_RECEIVE_ID_TYPE"
	EnvReceiveID       = "ALICE_MCP_RECEIVE_ID"
	EnvResourceRoot    = "ALICE_MCP_RESOURCE_ROOT"
	EnvSourceMessageID = "ALICE_MCP_SOURCE_MESSAGE_ID"
)

type SessionContext struct {
	ReceiveIDType   string
	ReceiveID       string
	ResourceRoot    string
	SourceMessageID string
}

func (c SessionContext) Validate() error {
	if strings.TrimSpace(c.ReceiveIDType) == "" {
		return errors.New("missing receive id type")
	}
	if strings.TrimSpace(c.ReceiveID) == "" {
		return errors.New("missing receive id")
	}
	return nil
}

func (c SessionContext) ToEnv() map[string]string {
	env := make(map[string]string, 4)
	env[EnvReceiveIDType] = strings.TrimSpace(c.ReceiveIDType)
	env[EnvReceiveID] = strings.TrimSpace(c.ReceiveID)
	if root := strings.TrimSpace(c.ResourceRoot); root != "" {
		env[EnvResourceRoot] = root
	}
	if sourceMessageID := strings.TrimSpace(c.SourceMessageID); sourceMessageID != "" {
		env[EnvSourceMessageID] = sourceMessageID
	}
	return env
}

func SessionContextFromEnv(getenv func(key string) string) SessionContext {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return SessionContext{
		ReceiveIDType:   strings.TrimSpace(getenv(EnvReceiveIDType)),
		ReceiveID:       strings.TrimSpace(getenv(EnvReceiveID)),
		ResourceRoot:    strings.TrimSpace(getenv(EnvResourceRoot)),
		SourceMessageID: strings.TrimSpace(getenv(EnvSourceMessageID)),
	}
}
