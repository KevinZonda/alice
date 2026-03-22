package imagegen

import (
	"testing"

	"github.com/Alice-space/alice/internal/config"
)

func TestResolveOpenAIProxyConfig_UsesDedicatedEnvOverrides(t *testing.T) {
	got := resolveOpenAIProxyConfig(config.ProxyConfig{
		HTTPProxy:  "http://config-http:8080",
		HTTPSProxy: "http://config-https:8080",
		ALLProxy:   "socks5://config-all:1080",
		NoProxy:    "config.internal",
	}, map[string]string{
		"OPENAI_HTTP_PROXY":  "  http://env-http:8080  ",
		"OPENAI_HTTPS_PROXY": "  http://env-https:8080  ",
		"OPENAI_ALL_PROXY":   "  socks5://env-all:1080  ",
		"OPENAI_NO_PROXY":    "  open.feishu.cn,.example.com  ",
	})

	if got.HTTPProxy != "http://env-http:8080" {
		t.Fatalf("unexpected http proxy: %q", got.HTTPProxy)
	}
	if got.HTTPSProxy != "http://env-https:8080" {
		t.Fatalf("unexpected https proxy: %q", got.HTTPSProxy)
	}
	if got.ALLProxy != "socks5://env-all:1080" {
		t.Fatalf("unexpected all proxy: %q", got.ALLProxy)
	}
	if got.NoProxy != "open.feishu.cn,.example.com" {
		t.Fatalf("unexpected no proxy: %q", got.NoProxy)
	}
}

func TestResolveOpenAIProxyConfig_KeepsConfigWhenEnvIsEmpty(t *testing.T) {
	want := config.ProxyConfig{
		HTTPProxy:  "http://config-http:8080",
		HTTPSProxy: "http://config-https:8080",
		ALLProxy:   "socks5://config-all:1080",
		NoProxy:    "config.internal",
	}
	got := resolveOpenAIProxyConfig(want, map[string]string{
		"OPENAI_HTTPS_PROXY": "   ",
	})

	if got != want {
		t.Fatalf("unexpected proxy config: want=%#v got=%#v", want, got)
	}
}
