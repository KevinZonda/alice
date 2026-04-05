package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// BotSelfInfo holds the identity of the Feishu bot returned by the Bot V3 API.
type BotSelfInfo struct {
	OpenID string
}

// FetchBotSelfInfo calls the Feishu /open-apis/bot/v3/info endpoint and
// returns the bot's identity. On failure the caller decides whether to
// degrade gracefully.
func FetchBotSelfInfo(ctx context.Context, client *lark.Client) (BotSelfInfo, error) {
	if client == nil {
		return BotSelfInfo{}, errors.New("lark client is nil")
	}
	resp, err := client.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return BotSelfInfo{}, err
	}
	if resp.StatusCode/100 != 2 {
		return BotSelfInfo{}, fmt.Errorf("get bot info http status=%d", resp.StatusCode)
	}

	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  *struct {
			OpenID string `json:"open_id"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(resp.RawBody, &body); err != nil {
		return BotSelfInfo{}, fmt.Errorf("parse bot info response: %w", err)
	}
	if body.Code != 0 {
		return BotSelfInfo{}, fmt.Errorf("get bot info api error code=%d msg=%s", body.Code, body.Msg)
	}
	if body.Bot == nil {
		return BotSelfInfo{}, errors.New("get bot info success but bot is empty")
	}
	return BotSelfInfo{
		OpenID: strings.TrimSpace(body.Bot.OpenID),
	}, nil
}
