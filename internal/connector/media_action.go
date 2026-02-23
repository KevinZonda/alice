package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	mediaActionTypeSendImage = "send_image"
	mediaActionTypeSendFile  = "send_file"
)

var mediaActionCodeFencePattern = regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)[ \t]*\\n(.*?)```")

type mediaActionEnvelope struct {
	Actions []mediaAction `json:"actions"`
	Reply   string        `json:"reply,omitempty"`
}

type mediaAction struct {
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
	FileKey  string `json:"file_key,omitempty"`
	FileName string `json:"file_name,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

func (p *Processor) executeMediaActions(
	ctx context.Context,
	receiveIDType string,
	receiveID string,
	reply string,
) (string, bool, error) {
	cleanedReply, actions, handled, err := parseMediaActionsReply(reply)
	if !handled {
		return strings.TrimSpace(reply), false, nil
	}
	if err != nil {
		return buildMediaActionFailureReply(cleanedReply, err), true, err
	}

	for idx, action := range actions {
		if sendErr := p.sendMediaAction(ctx, receiveIDType, receiveID, action); sendErr != nil {
			return buildMediaActionFailureReply(
				cleanedReply,
				fmt.Errorf("action[%d] %s failed: %w", idx, action.Type, sendErr),
			), true, sendErr
		}
	}
	return cleanedReply, true, nil
}

func (p *Processor) sendMediaAction(
	ctx context.Context,
	receiveIDType string,
	receiveID string,
	action mediaAction,
) error {
	var err error
	switch action.Type {
	case mediaActionTypeSendImage:
		imageKey := strings.TrimSpace(action.ImageKey)
		if imageKey == "" {
			imageKey, err = p.sender.UploadImage(ctx, action.Path)
			if err != nil {
				return err
			}
		}
		if err = p.sender.SendImage(ctx, receiveIDType, receiveID, imageKey); err != nil {
			return err
		}
	case mediaActionTypeSendFile:
		fileKey := strings.TrimSpace(action.FileKey)
		if fileKey == "" {
			fileKey, err = p.sender.UploadFile(ctx, action.Path, action.FileName)
			if err != nil {
				return err
			}
		}
		if err = p.sender.SendFile(ctx, receiveIDType, receiveID, fileKey); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported media action type: %s", action.Type)
	}

	caption := strings.TrimSpace(action.Caption)
	if caption == "" {
		return nil
	}
	return p.sender.SendText(ctx, receiveIDType, receiveID, caption)
}

func parseMediaActionsReply(reply string) (string, []mediaAction, bool, error) {
	raw := strings.TrimSpace(reply)
	if raw == "" {
		return "", nil, false, nil
	}

	if cleaned, actions, handled, err := parseMediaActionsFromCodeFence(reply); handled {
		return cleaned, actions, handled, err
	}

	if isLikelyJSONPayload(raw) {
		replyText, actions, parseErr := decodeMediaActionPayload(raw)
		if parseErr != nil {
			if strings.Contains(raw, "\"actions\"") {
				return "", nil, true, parseErr
			}
			return strings.TrimSpace(reply), nil, false, nil
		}
		return strings.TrimSpace(replyText), actions, true, nil
	}

	return strings.TrimSpace(reply), nil, false, nil
}

func parseMediaActionsFromCodeFence(reply string) (string, []mediaAction, bool, error) {
	matches := mediaActionCodeFencePattern.FindAllStringSubmatchIndex(reply, -1)
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(reply[match[2]:match[3]]))
		body := strings.TrimSpace(reply[match[4]:match[5]])
		if body == "" {
			continue
		}
		if !isMediaActionFenceLabel(label, body) {
			continue
		}

		replyText, actions, err := decodeMediaActionPayload(body)
		if err != nil {
			return "", nil, true, err
		}
		cleaned := strings.TrimSpace(reply[:match[0]] + reply[match[1]:])
		if cleaned == "" {
			cleaned = strings.TrimSpace(replyText)
		}
		return cleaned, actions, true, nil
	}
	return strings.TrimSpace(reply), nil, false, nil
}

func isMediaActionFenceLabel(label string, body string) bool {
	switch label {
	case "alice_action", "alice_actions", "alice_media", "alice_media_actions":
		return true
	case "", "json":
		return strings.Contains(body, "\"actions\"")
	default:
		return false
	}
}

func isLikelyJSONPayload(raw string) bool {
	return strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[")
}

func decodeMediaActionPayload(payload string) (string, []mediaAction, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return "", nil, errors.New("media action payload is empty")
	}

	var envelope mediaActionEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err == nil {
		actions, normalizeErr := normalizeMediaActions(envelope.Actions)
		if normalizeErr == nil {
			return strings.TrimSpace(envelope.Reply), actions, nil
		}
	}

	var actions []mediaAction
	if err := json.Unmarshal([]byte(payload), &actions); err == nil {
		normalized, normalizeErr := normalizeMediaActions(actions)
		if normalizeErr == nil {
			return "", normalized, nil
		}
	}

	var single mediaAction
	if err := json.Unmarshal([]byte(payload), &single); err == nil {
		normalized, normalizeErr := normalizeMediaActions([]mediaAction{single})
		if normalizeErr == nil {
			return "", normalized, nil
		}
	}

	return "", nil, errors.New("invalid media action payload")
}

func normalizeMediaActions(actions []mediaAction) ([]mediaAction, error) {
	if len(actions) == 0 {
		return nil, errors.New("media actions are empty")
	}

	normalized := make([]mediaAction, 0, len(actions))
	for idx, action := range actions {
		item := mediaAction{
			Type:     normalizeMediaActionType(action.Type),
			Path:     strings.TrimSpace(action.Path),
			ImageKey: strings.TrimSpace(action.ImageKey),
			FileKey:  strings.TrimSpace(action.FileKey),
			FileName: strings.TrimSpace(action.FileName),
			Caption:  strings.TrimSpace(action.Caption),
		}
		if item.Type == "" {
			return nil, fmt.Errorf("action[%d] type is required", idx)
		}

		switch item.Type {
		case mediaActionTypeSendImage:
			if item.ImageKey == "" && item.Path == "" {
				return nil, fmt.Errorf("action[%d] send_image requires image_key or path", idx)
			}
		case mediaActionTypeSendFile:
			if item.FileKey == "" && item.Path == "" {
				return nil, fmt.Errorf("action[%d] send_file requires file_key or path", idx)
			}
		default:
			return nil, fmt.Errorf("action[%d] unsupported type: %s", idx, item.Type)
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func normalizeMediaActionType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "send_image", "image", "sendimage":
		return mediaActionTypeSendImage
	case "send_file", "file", "sendfile":
		return mediaActionTypeSendFile
	default:
		return ""
	}
}

func buildMediaActionFailureReply(cleanedReply string, err error) string {
	prefix := strings.TrimSpace(cleanedReply)
	if err == nil {
		return prefix
	}
	errText := "多媒体发送失败：" + strings.TrimSpace(err.Error())
	if prefix == "" {
		return errText
	}
	return strings.TrimSpace(prefix + "\n\n" + errText)
}

func stripMediaActionFromReply(reply string) string {
	cleaned, _, handled, err := parseMediaActionsReply(reply)
	if !handled || err != nil {
		return strings.TrimSpace(reply)
	}
	return strings.TrimSpace(cleaned)
}
