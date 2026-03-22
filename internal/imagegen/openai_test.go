package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"

	"github.com/Alice-space/alice/internal/config"
)

func TestResolveOpenAIProxyConfig_UsesDedicatedEnvOverrides(t *testing.T) {
	got := resolveOpenAIProxyConfig(map[string]string{
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

func TestResolveOpenAIProxyConfig_EmptyWhenEnvIsEmpty(t *testing.T) {
	got := resolveOpenAIProxyConfig(map[string]string{
		"OPENAI_HTTPS_PROXY": "   ",
	})

	if got != (openAIProxyConfig{}) {
		t.Fatalf("unexpected proxy config: %#v", got)
	}
}

func TestImageContentTypeForPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "refs/base.png", want: "image/png"},
		{path: "refs/base.jpg", want: "image/jpeg"},
		{path: "refs/base.jpeg", want: "image/jpeg"},
		{path: "refs/base.webp", want: "image/webp"},
	}

	for _, tc := range tests {
		got, err := imageContentTypeForPath(tc.path)
		if err != nil {
			t.Fatalf("imageContentTypeForPath(%q) returned error: %v", tc.path, err)
		}
		if got != tc.want {
			t.Fatalf("imageContentTypeForPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestImageContentTypeForPath_RejectsUnsupportedExtension(t *testing.T) {
	_, err := imageContentTypeForPath("refs/base.gif")
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestOpenAIFileWrapperCarriesImageMetadata(t *testing.T) {
	reader := openai.File(strings.NewReader("png-bytes"), "base.png", "image/png")

	if reader.Filename() != "base.png" {
		t.Fatalf("unexpected filename: %q", reader.Filename())
	}

	if reader.ContentType() != "image/png" {
		t.Fatalf("unexpected content type: %q", reader.ContentType())
	}
}

func TestOpenAIProviderEditImage_UsesConfiguredParams(t *testing.T) {
	t.Parallel()

	type uploadedImage struct {
		FormName    string
		FileName    string
		ContentType string
	}
	type requestCapture struct {
		Path   string
		Fields map[string][]string
		Images []uploadedImage
		Masks  []uploadedImage
	}

	captured := requestCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		captured.Path = r.URL.Path
		captured.Fields = map[string][]string{}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() failed: %v", err)
		}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() failed: %v", err)
			}
			body, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Fatalf("ReadAll(%q) failed: %v", part.FormName(), readErr)
			}
			if part.FormName() == "mask" {
				captured.Masks = append(captured.Masks, uploadedImage{
					FormName:    part.FormName(),
					FileName:    part.FileName(),
					ContentType: part.Header.Get("Content-Type"),
				})
				continue
			}
			if strings.HasPrefix(part.FormName(), "image") {
				captured.Images = append(captured.Images, uploadedImage{
					FormName:    part.FormName(),
					FileName:    part.FileName(),
					ContentType: part.Header.Get("Content-Type"),
				})
				continue
			}
			captured.Fields[part.FormName()] = append(captured.Fields[part.FormName()], string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString([]byte("ok")) + `"}]}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "out.png")
	refPath := filepath.Join(tempDir, "ref.png")
	maskPath := filepath.Join(tempDir, "mask.png")
	if err := os.WriteFile(refPath, []byte("fake-png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", refPath, err)
	}
	if err := os.WriteFile(maskPath, []byte("fake-mask"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", maskPath, err)
	}
	provider := mustNewTestOpenAIProvider(t, server.URL, func(cfg *config.ImageGenerationConfig) {
		cfg.Moderation = "low"
		cfg.N = 2
		cfg.OutputCompression = 80
		cfg.Size = "1024x1024"
		cfg.Quality = "medium"
		cfg.Background = "transparent"
		cfg.OutputFormat = "webp"
		cfg.PartialImages = 2
		cfg.InputFidelity = "low"
		cfg.MaskPath = maskPath
	})

	_, err := provider.Generate(context.Background(), Request{
		Prompt:          "draw Mea waving",
		ReferenceImages: []string{refPath},
		OutputPath:      outputPath,
		UserID:          "user-123",
	})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if captured.Path != "/images/edits" {
		t.Fatalf("unexpected path: %q", captured.Path)
	}
	wantFields := map[string]string{
		"background":         "transparent",
		"input_fidelity":     "low",
		"model":              "gpt-image-1.5",
		"moderation":         "low",
		"n":                  "2",
		"output_compression": "80",
		"output_format":      "webp",
		"partial_images":     "2",
		"quality":            "medium",
		"size":               "1024x1024",
		"user":               "user-123",
	}
	for field, want := range wantFields {
		if got := firstField(captured.Fields, field); got != want {
			t.Fatalf("unexpected %s: got %q want %q", field, got, want)
		}
	}
	if len(captured.Images) != 1 {
		t.Fatalf("unexpected image count: %d", len(captured.Images))
	}
	if captured.Images[0].ContentType != "image/png" {
		t.Fatalf("unexpected image content type: %q", captured.Images[0].ContentType)
	}
	if captured.Images[0].FileName != "ref.png" {
		t.Fatalf("unexpected image filename: %q", captured.Images[0].FileName)
	}
	if len(captured.Masks) != 1 {
		t.Fatalf("unexpected mask count: %d", len(captured.Masks))
	}
	if captured.Masks[0].ContentType != "image/png" {
		t.Fatalf("unexpected mask content type: %q", captured.Masks[0].ContentType)
	}
	if captured.Masks[0].FileName != "mask.png" {
		t.Fatalf("unexpected mask filename: %q", captured.Masks[0].FileName)
	}
}

func TestOpenAIProviderGenerateImage_UsesConfiguredParams(t *testing.T) {
	t.Parallel()

	type generateRequest struct {
		Path              string `json:"-"`
		Background        string `json:"background"`
		Moderation        string `json:"moderation"`
		N                 int    `json:"n"`
		OutputCompression int    `json:"output_compression"`
		OutputFormat      string `json:"output_format"`
		PartialImages     int    `json:"partial_images"`
		Prompt            string `json:"prompt"`
		Model             string `json:"model"`
		Quality           string `json:"quality"`
		ResponseFormat    string `json:"response_format"`
		Size              string `json:"size"`
		User              string `json:"user"`
	}

	var captured generateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		captured.Path = r.URL.Path
		if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil {
			t.Fatalf("ParseMediaType() failed: %v", err)
		} else if mediaType != "application/json" {
			t.Fatalf("unexpected content type: %q", mediaType)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("Decode() failed: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString([]byte("ok")) + `"}]}`))
	}))
	defer server.Close()

	provider := mustNewTestOpenAIProvider(t, server.URL, func(cfg *config.ImageGenerationConfig) {
		cfg.Moderation = "low"
		cfg.N = 2
		cfg.OutputCompression = 70
		cfg.OutputFormat = "webp"
		cfg.PartialImages = 2
		cfg.Quality = "medium"
		cfg.ResponseFormat = "b64_json"
		cfg.Size = "1024x1024"
	})
	outputPath := filepath.Join(t.TempDir(), "out.png")

	_, err := provider.Generate(context.Background(), Request{
		Prompt:     "draw Mea waving",
		OutputPath: outputPath,
		UserID:     "user-123",
	})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if captured.Path != "/images/generations" {
		t.Fatalf("unexpected path: %q", captured.Path)
	}
	if captured.Background != "auto" {
		t.Fatalf("unexpected background: %q", captured.Background)
	}
	if captured.Model != "gpt-image-1.5" {
		t.Fatalf("unexpected model: %q", captured.Model)
	}
	if captured.Moderation != "low" {
		t.Fatalf("unexpected moderation: %q", captured.Moderation)
	}
	if captured.N != 2 {
		t.Fatalf("unexpected n: %d", captured.N)
	}
	if captured.OutputCompression != 70 {
		t.Fatalf("unexpected output_compression: %d", captured.OutputCompression)
	}
	if captured.OutputFormat != "webp" {
		t.Fatalf("unexpected output_format: %q", captured.OutputFormat)
	}
	if captured.PartialImages != 2 {
		t.Fatalf("unexpected partial_images: %d", captured.PartialImages)
	}
	if captured.Quality != "medium" {
		t.Fatalf("unexpected quality: %q", captured.Quality)
	}
	if captured.ResponseFormat != "b64_json" {
		t.Fatalf("unexpected response_format: %q", captured.ResponseFormat)
	}
	if captured.Size != "1024x1024" {
		t.Fatalf("unexpected size: %q", captured.Size)
	}
	if captured.User != "user-123" {
		t.Fatalf("unexpected user: %q", captured.User)
	}
}

func TestOpenAIProviderGenerateImage_DownloadsURLResponse(t *testing.T) {
	t.Parallel()

	imageBytes := []byte("downloaded-image")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		switch r.URL.Path {
		case "/images/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"url":"` + server.URL + `/files/out.png"}]}`))
		case "/files/out.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := mustNewTestOpenAIProvider(t, server.URL, func(cfg *config.ImageGenerationConfig) {
		cfg.Model = "dall-e-3"
		cfg.ResponseFormat = "url"
		cfg.Style = "natural"
		cfg.Quality = "hd"
		cfg.Size = "1792x1024"
		cfg.Background = ""
		cfg.OutputFormat = ""
		cfg.Moderation = ""
	})
	outputPath := filepath.Join(t.TempDir(), "out.png")

	result, err := provider.Generate(context.Background(), Request{
		Prompt:     "draw Mea waving",
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if result.LocalPath != outputPath {
		t.Fatalf("unexpected local path: %q", result.LocalPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", outputPath, err)
	}
	if string(data) != string(imageBytes) {
		t.Fatalf("unexpected downloaded image bytes: %q", string(data))
	}
}

func TestOpenAIProviderEditImage_StreamingUsesCompletedEvent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/images/edits" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() failed: %v", err)
		}
		fields := map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() failed: %v", err)
			}
			if strings.HasPrefix(part.FormName(), "image") {
				_, _ = io.Copy(io.Discard, part)
				continue
			}
			body, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Fatalf("ReadAll(%q) failed: %v", part.FormName(), readErr)
			}
			fields[part.FormName()] = string(body)
		}
		if fields["stream"] != "true" {
			t.Fatalf("expected stream=true, got %q", fields["stream"])
		}
		if fields["moderation"] != "low" {
			t.Fatalf("expected moderation=low, got %q", fields["moderation"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: image_edit.completed\n")
		_, _ = io.WriteString(w, `data: {"type":"image_edit.completed","b64_json":"`+base64.StdEncoding.EncodeToString([]byte("streamed-ok"))+`","background":"auto","created_at":1,"output_format":"png","quality":"high","size":"1024x1024","usage":{"input_tokens":1,"input_tokens_details":{"image_tokens":0,"text_tokens":1},"output_tokens":1,"total_tokens":2}}`+"\n\n")
	}))
	defer server.Close()

	tempDir := t.TempDir()
	refPath := filepath.Join(tempDir, "ref.png")
	if err := os.WriteFile(refPath, []byte("fake-png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", refPath, err)
	}
	provider := mustNewTestOpenAIProvider(t, server.URL, func(cfg *config.ImageGenerationConfig) {
		cfg.Stream = true
		cfg.Moderation = "low"
	})
	outputPath := filepath.Join(tempDir, "out.png")

	result, err := provider.Generate(context.Background(), Request{
		Prompt:          "draw Mea waving",
		ReferenceImages: []string{refPath},
		OutputPath:      outputPath,
	})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", outputPath, err)
	}
	if string(data) != "streamed-ok" {
		t.Fatalf("unexpected streamed image bytes: %q", string(data))
	}
	if result.LocalPath != outputPath {
		t.Fatalf("unexpected local path: %q", result.LocalPath)
	}
}

func mustNewTestOpenAIProvider(
	t *testing.T,
	baseURL string,
	override func(*config.ImageGenerationConfig),
) *openAIProvider {
	t.Helper()

	cfg := config.ImageGenerationConfig{
		Provider:              "openai",
		Model:                 "gpt-image-1.5",
		BaseURL:               baseURL,
		TimeoutSecs:           5,
		Size:                  "1024x1536",
		Quality:               "high",
		Background:            "auto",
		OutputFormat:          "png",
		InputFidelity:         "high",
		UseCurrentAttachments: true,
		OutputCompression:     -1,
		PartialImages:         -1,
	}
	if override != nil {
		override(&cfg)
	}
	provider, err := newOpenAIProvider(cfg, map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	if err != nil {
		t.Fatalf("newOpenAIProvider() failed: %v", err)
	}
	typed, ok := provider.(*openAIProvider)
	if !ok {
		t.Fatalf("unexpected provider type: %T", provider)
	}
	return typed
}

func firstField(fields map[string][]string, key string) string {
	values := fields[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
