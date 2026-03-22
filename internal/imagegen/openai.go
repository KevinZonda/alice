package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"golang.org/x/net/http/httpproxy"

	"github.com/Alice-space/alice/internal/config"
)

type openAIProvider struct {
	httpClient        *http.Client
	client            openai.Client
	apiKey            string
	baseURL           string
	model             openai.ImageModel
	moderation        string
	n                 int
	outputCompression int
	responseFormat    string
	size              string
	quality           string
	background        string
	outputFormat      string
	partialImages     int
	stream            bool
	style             string
	inputFidelity     string
	maskPath          string
}

func newOpenAIProvider(cfg config.ImageGenerationConfig, env map[string]string) (Provider, error) {
	httpClient, err := newHTTPClient(resolveOpenAIProxyConfig(env), time.Duration(cfg.TimeoutSecs)*time.Second)
	if err != nil {
		return nil, err
	}

	opts := []option.RequestOption{
		option.WithHTTPClient(httpClient),
		option.WithRequestTimeout(time.Duration(cfg.TimeoutSecs) * time.Second),
	}
	apiKey := strings.TrimSpace(lookupEnv(env, "OPENAI_API_KEY"))
	if apiKey := lookupEnv(env, "OPENAI_API_KEY"); apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = lookupEnv(env, "OPENAI_BASE_URL")
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	} else {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	client := openai.NewClient(opts...)
	return &openAIProvider{
		httpClient:        httpClient,
		client:            client,
		apiKey:            apiKey,
		baseURL:           baseURL,
		model:             openai.ImageModel(strings.TrimSpace(cfg.Model)),
		moderation:        strings.TrimSpace(cfg.Moderation),
		n:                 cfg.N,
		outputCompression: cfg.OutputCompression,
		responseFormat:    strings.TrimSpace(cfg.ResponseFormat),
		size:              strings.TrimSpace(cfg.Size),
		quality:           strings.TrimSpace(cfg.Quality),
		background:        strings.TrimSpace(cfg.Background),
		outputFormat:      strings.TrimSpace(cfg.OutputFormat),
		partialImages:     cfg.PartialImages,
		stream:            cfg.Stream,
		style:             strings.TrimSpace(cfg.Style),
		inputFidelity:     strings.TrimSpace(cfg.InputFidelity),
		maskPath:          strings.TrimSpace(cfg.MaskPath),
	}, nil
}

type openAIProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	ALLProxy   string
	NoProxy    string
}

func resolveOpenAIProxyConfig(env map[string]string) openAIProxyConfig {
	return openAIProxyConfig{
		HTTPProxy:  lookupEnv(env, "OPENAI_HTTP_PROXY"),
		HTTPSProxy: lookupEnv(env, "OPENAI_HTTPS_PROXY"),
		ALLProxy:   lookupEnv(env, "OPENAI_ALL_PROXY"),
		NoProxy:    lookupEnv(env, "OPENAI_NO_PROXY"),
	}
}

func (p *openAIProvider) Generate(ctx context.Context, req Request) (Result, error) {
	if p == nil {
		return Result{}, errors.New("openai image provider is nil")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return Result{}, errors.New("image prompt is empty")
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		return Result{}, errors.New("image output path is empty")
	}

	var (
		payloads      []imageOutput
		revisedPrompt string
		err           error
	)
	references := compactExistingPaths(req.ReferenceImages, 16)
	if len(references) > 0 {
		payloads, revisedPrompt, err = p.editImage(ctx, req, references)
	} else {
		payloads, revisedPrompt, err = p.generateImage(ctx, req)
	}
	if err != nil {
		return Result{}, err
	}
	localPaths, err := p.writeImageOutputs(ctx, req.OutputPath, payloads)
	if err != nil {
		return Result{}, err
	}
	if len(localPaths) == 0 {
		return Result{}, errors.New("openai image response is empty")
	}
	return Result{
		LocalPath:     localPaths[0],
		LocalPaths:    localPaths,
		RevisedPrompt: revisedPrompt,
	}, nil
}

type imageOutput struct {
	B64JSON       string
	URL           string
	RevisedPrompt string
}

func (p *openAIProvider) generateImage(ctx context.Context, req Request) ([]imageOutput, string, error) {
	params := p.buildGenerateParams(req)
	if p.stream {
		return p.generateImageStreaming(ctx, params)
	}
	resp, err := p.client.Images.Generate(ctx, params)
	if err != nil {
		return nil, "", err
	}
	return extractImageOutputs(resp)
}

func (p *openAIProvider) editImage(ctx context.Context, req Request, references []string) ([]imageOutput, string, error) {
	readers := make([]io.Reader, 0, len(references))
	files := make([]*os.File, 0, len(references))
	for _, path := range references {
		file, err := os.Open(path)
		if err != nil {
			for _, opened := range files {
				_ = opened.Close()
			}
			return nil, "", fmt.Errorf("open reference image %s failed: %w", path, err)
		}
		contentType, err := imageContentTypeForPath(path)
		if err != nil {
			_ = file.Close()
			for _, opened := range files {
				_ = opened.Close()
			}
			return nil, "", err
		}
		files = append(files, file)
		readers = append(readers, openai.File(file, filepath.Base(path), contentType))
	}
	maskFile, maskReader, err := p.openMaskReader()
	if err != nil {
		for _, file := range files {
			_ = file.Close()
		}
		return nil, "", err
	}
	if maskFile != nil {
		files = append(files, maskFile)
	}
	defer func() {
		for _, file := range files {
			_ = file.Close()
		}
	}()

	params := p.buildEditParams(req, readers, maskReader)
	if p.stream {
		return p.editImageStreaming(ctx, params)
	}
	resp, err := p.client.Images.Edit(ctx, params)
	if err != nil {
		return nil, "", err
	}
	return extractImageOutputs(resp)
}

func imageContentTypeForPath(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".webp":
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unsupported reference image format for %s", path)
	}
}

func (p *openAIProvider) buildGenerateParams(req Request) openai.ImageGenerateParams {
	params := openai.ImageGenerateParams{
		Prompt:         req.Prompt,
		Model:          p.model,
		Background:     openai.ImageGenerateParamsBackground(p.background),
		OutputFormat:   openai.ImageGenerateParamsOutputFormat(p.outputFormat),
		Quality:        openai.ImageGenerateParamsQuality(p.quality),
		Size:           openai.ImageGenerateParamsSize(p.size),
		Moderation:     openai.ImageGenerateParamsModeration(p.moderation),
		ResponseFormat: openai.ImageGenerateParamsResponseFormat(p.responseFormat),
		Style:          openai.ImageGenerateParamsStyle(p.style),
	}
	if p.n > 0 {
		params.N = openai.Int(int64(p.n))
	}
	if p.outputCompression >= 0 {
		params.OutputCompression = openai.Int(int64(p.outputCompression))
	}
	if p.partialImages >= 0 {
		params.PartialImages = openai.Int(int64(p.partialImages))
	}
	if strings.TrimSpace(req.UserID) != "" {
		params.User = openai.String(strings.TrimSpace(req.UserID))
	}
	return params
}

func (p *openAIProvider) buildEditParams(req Request, readers []io.Reader, mask io.Reader) openai.ImageEditParams {
	imageUnion := openai.ImageEditParamsImageUnion{}
	switch len(readers) {
	case 1:
		imageUnion.OfFile = readers[0]
	default:
		imageUnion.OfFileArray = readers
	}
	params := openai.ImageEditParams{
		Image:         imageUnion,
		Prompt:        req.Prompt,
		Model:         p.model,
		Background:    openai.ImageEditParamsBackground(p.background),
		InputFidelity: openai.ImageEditParamsInputFidelity(p.inputFidelity),
		OutputFormat:  openai.ImageEditParamsOutputFormat(p.outputFormat),
		Quality:       openai.ImageEditParamsQuality(p.quality),
		Size:          openai.ImageEditParamsSize(p.size),
	}
	if p.n > 0 {
		params.N = openai.Int(int64(p.n))
	}
	if p.outputCompression >= 0 {
		params.OutputCompression = openai.Int(int64(p.outputCompression))
	}
	if p.partialImages >= 0 {
		params.PartialImages = openai.Int(int64(p.partialImages))
	}
	if mask != nil {
		params.Mask = mask
	}
	if strings.TrimSpace(req.UserID) != "" {
		params.User = openai.String(strings.TrimSpace(req.UserID))
	}
	if strings.TrimSpace(p.moderation) != "" {
		params.SetExtraFields(map[string]any{
			"moderation": strings.TrimSpace(p.moderation),
		})
	}
	return params
}

func (p *openAIProvider) generateImageStreaming(
	ctx context.Context,
	params openai.ImageGenerateParams,
) ([]imageOutput, string, error) {
	stream := p.client.Images.GenerateStreaming(ctx, params)
	if stream == nil {
		return nil, "", errors.New("openai image generation stream is nil")
	}
	var completed *openai.ImageGenCompletedEvent
	for stream.Next() {
		switch event := stream.Current().AsAny().(type) {
		case openai.ImageGenCompletedEvent:
			evt := event
			completed = &evt
		}
	}
	if err := stream.Err(); err != nil {
		return nil, "", err
	}
	if completed == nil || strings.TrimSpace(completed.B64JSON) == "" {
		return nil, "", errors.New("openai image generation stream missing completed image")
	}
	return []imageOutput{{B64JSON: strings.TrimSpace(completed.B64JSON)}}, "", nil
}

func (p *openAIProvider) editImageStreaming(
	ctx context.Context,
	params openai.ImageEditParams,
) ([]imageOutput, string, error) {
	params.SetExtraFields(mergeExtraFields(params.ExtraFields(), map[string]any{
		"stream": "true",
	}))
	body, contentType, err := params.MarshalMultipart()
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images/edits", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(p.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.apiKey))
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("openai image edit streaming failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	stream := ssestream.NewStream[openai.ImageEditStreamEventUnion](ssestream.NewDecoder(resp), nil)
	if stream == nil {
		_ = resp.Body.Close()
		return nil, "", errors.New("openai image edit stream is nil")
	}
	var completed *openai.ImageEditCompletedEvent
	for stream.Next() {
		switch event := stream.Current().AsAny().(type) {
		case openai.ImageEditCompletedEvent:
			evt := event
			completed = &evt
		}
	}
	err = stream.Err()
	_ = resp.Body.Close()
	if err != nil {
		return nil, "", err
	}
	if completed == nil || strings.TrimSpace(completed.B64JSON) == "" {
		return nil, "", errors.New("openai image edit stream missing completed image")
	}
	return []imageOutput{{B64JSON: strings.TrimSpace(completed.B64JSON)}}, "", nil
}

func extractImageOutputs(resp *openai.ImagesResponse) ([]imageOutput, string, error) {
	if resp == nil {
		return nil, "", errors.New("openai image response is nil")
	}
	if len(resp.Data) == 0 {
		return nil, "", errors.New("openai image response is empty")
	}
	outputs := make([]imageOutput, 0, len(resp.Data))
	revisedPrompt := ""
	for _, item := range resp.Data {
		output := imageOutput{
			B64JSON:       strings.TrimSpace(item.B64JSON),
			URL:           strings.TrimSpace(item.URL),
			RevisedPrompt: strings.TrimSpace(item.RevisedPrompt),
		}
		if output.B64JSON == "" && output.URL == "" {
			continue
		}
		if revisedPrompt == "" && output.RevisedPrompt != "" {
			revisedPrompt = output.RevisedPrompt
		}
		outputs = append(outputs, output)
	}
	if len(outputs) == 0 {
		return nil, "", errors.New("openai image response missing image data")
	}
	return outputs, revisedPrompt, nil
}

func (p *openAIProvider) writeImageOutputs(
	ctx context.Context,
	outputPath string,
	outputs []imageOutput,
) ([]string, error) {
	if len(outputs) == 0 {
		return nil, errors.New("openai image response is empty")
	}
	localPaths := make([]string, 0, len(outputs))
	for idx, output := range outputs {
		targetPath := indexedOutputPath(outputPath, idx)
		switch {
		case strings.TrimSpace(output.B64JSON) != "":
			if err := writeBase64Image(targetPath, output.B64JSON); err != nil {
				return nil, err
			}
		case strings.TrimSpace(output.URL) != "":
			if err := p.downloadImageURL(ctx, output.URL, targetPath); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("openai image response missing image data")
		}
		localPaths = append(localPaths, targetPath)
	}
	return localPaths, nil
}

func writeBase64Image(outputPath, imageData string) error {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(imageData))
	if err != nil {
		return fmt.Errorf("decode openai image failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("prepare image output dir failed: %w", err)
	}
	if err := os.WriteFile(outputPath, decoded, 0o644); err != nil {
		return fmt.Errorf("write generated image failed: %w", err)
	}
	return nil
}

func (p *openAIProvider) downloadImageURL(ctx context.Context, rawURL, outputPath string) error {
	if p == nil || p.httpClient == nil {
		return errors.New("openai image download client is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download openai image url failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download openai image url failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("prepare image output dir failed: %w", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create generated image failed: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write generated image failed: %w", err)
	}
	return nil
}

func indexedOutputPath(outputPath string, idx int) string {
	if idx <= 0 {
		return outputPath
	}
	ext := filepath.Ext(outputPath)
	base := strings.TrimSuffix(outputPath, ext)
	return fmt.Sprintf("%s-%d%s", base, idx+1, ext)
}

func (p *openAIProvider) openMaskReader() (*os.File, io.Reader, error) {
	maskPath := strings.TrimSpace(p.maskPath)
	if maskPath == "" {
		return nil, nil, nil
	}
	resolvedPath, err := filepath.Abs(maskPath)
	if err != nil {
		return nil, nil, err
	}
	if strings.ToLower(filepath.Ext(resolvedPath)) != ".png" {
		return nil, nil, fmt.Errorf("mask image must be a PNG file: %s", resolvedPath)
	}
	maskFile, err := os.Open(resolvedPath)
	if err != nil {
		return nil, nil, err
	}
	return maskFile, openai.File(maskFile, filepath.Base(resolvedPath), "image/png"), nil
}

func mergeExtraFields(current map[string]any, extra map[string]any) map[string]any {
	if len(current) == 0 && len(extra) == 0 {
		return nil
	}
	merged := make(map[string]any, len(current)+len(extra))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func newHTTPClient(proxyCfg openAIProxyConfig, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyFunc, err := buildProxyFunc(proxyCfg)
	if err != nil {
		return nil, err
	}
	transport.Proxy = proxyFunc
	transport.ForceAttemptHTTP2 = true
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

func buildProxyFunc(proxyCfg openAIProxyConfig) (func(*http.Request) (*url.URL, error), error) {
	proxyCfg = openAIProxyConfig{
		HTTPProxy:  strings.TrimSpace(proxyCfg.HTTPProxy),
		HTTPSProxy: strings.TrimSpace(proxyCfg.HTTPSProxy),
		ALLProxy:   strings.TrimSpace(proxyCfg.ALLProxy),
		NoProxy:    strings.TrimSpace(proxyCfg.NoProxy),
	}
	if proxyCfg.HTTPProxy == "" {
		proxyCfg.HTTPProxy = proxyCfg.ALLProxy
	}
	if proxyCfg.HTTPSProxy == "" {
		proxyCfg.HTTPSProxy = proxyCfg.ALLProxy
	}
	if proxyCfg.HTTPProxy == "" && proxyCfg.HTTPSProxy == "" && proxyCfg.NoProxy == "" {
		return nil, nil
	}
	cfg := &httpproxy.Config{
		HTTPProxy:  proxyCfg.HTTPProxy,
		HTTPSProxy: proxyCfg.HTTPSProxy,
		NoProxy:    proxyCfg.NoProxy,
	}
	proxyForURL := cfg.ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		if req == nil || req.URL == nil {
			return nil, nil
		}
		return proxyForURL(req.URL)
	}, nil
}

func lookupEnv(env map[string]string, key string) string {
	if len(env) == 0 {
		return ""
	}
	return strings.TrimSpace(env[strings.ToUpper(strings.TrimSpace(key))])
}

func compactExistingPaths(paths []string, limit int) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
