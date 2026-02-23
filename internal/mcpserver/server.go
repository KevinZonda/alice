package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"gitee.com/alicespace/alice/internal/mcpbridge"
)

const (
	ToolSendImage = "send_image"
	ToolSendFile  = "send_file"
)

type Sender interface {
	SendText(ctx context.Context, receiveIDType, receiveID, text string) error
	SendImage(ctx context.Context, receiveIDType, receiveID, imageKey string) error
	SendFile(ctx context.Context, receiveIDType, receiveID, fileKey string) error
	UploadImage(ctx context.Context, localPath string) (string, error)
	UploadFile(ctx context.Context, localPath, fileName string) (string, error)
}

type replyTextSender interface {
	ReplyText(ctx context.Context, sourceMessageID, text string) (string, error)
}

type replyImageSender interface {
	ReplyImage(ctx context.Context, sourceMessageID, imageKey string) (string, error)
}

type replyFileSender interface {
	ReplyFile(ctx context.Context, sourceMessageID, fileKey string) (string, error)
}

type service struct {
	sender Sender
	getenv func(string) string
}

func New(sender Sender, getenv func(string) string) (*server.MCPServer, error) {
	if sender == nil {
		return nil, errors.New("sender is nil")
	}
	if getenv == nil {
		getenv = os.Getenv
	}

	svc := &service{sender: sender, getenv: getenv}
	mcpServer := server.NewMCPServer(
		"alice-feishu-tools",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	mcpServer.AddTool(mcp.NewTool(
		ToolSendImage,
		mcp.WithDescription("向当前会话发送图片。优先使用会话上下文，若缺失可显式传 receive_id_type/receive_id 回退。"),
		mcp.WithString("image_key", mcp.Description("已存在的飞书 image_key")),
		mcp.WithString("path", mcp.Description("本地绝对路径，仅允许资源目录白名单路径")),
		mcp.WithString("caption", mcp.Description("可选文字说明，发送在图片之后")),
		mcp.WithString("receive_id_type", mcp.Description("可选回退：当会话上下文缺失时，显式传当前会话接收ID类型，如 chat_id/open_id")),
		mcp.WithString("receive_id", mcp.Description("可选回退：当会话上下文缺失时，显式传当前会话接收ID")),
		mcp.WithString("source_message_id", mcp.Description("可选回退：当会话上下文缺失时，显式传当前线程源消息ID，用于在 thread 内回复")),
	), svc.handleSendImage)

	mcpServer.AddTool(mcp.NewTool(
		ToolSendFile,
		mcp.WithDescription("向当前会话发送文件。优先使用会话上下文，若缺失可显式传 receive_id_type/receive_id 回退。"),
		mcp.WithString("file_key", mcp.Description("已存在的飞书 file_key")),
		mcp.WithString("path", mcp.Description("本地绝对路径，仅允许资源目录白名单路径")),
		mcp.WithString("file_name", mcp.Description("可选文件名，path上传时生效")),
		mcp.WithString("caption", mcp.Description("可选文字说明，发送在文件之后")),
		mcp.WithString("receive_id_type", mcp.Description("可选回退：当会话上下文缺失时，显式传当前会话接收ID类型，如 chat_id/open_id")),
		mcp.WithString("receive_id", mcp.Description("可选回退：当会话上下文缺失时，显式传当前会话接收ID")),
		mcp.WithString("source_message_id", mcp.Description("可选回退：当会话上下文缺失时，显式传当前线程源消息ID，用于在 thread 内回复")),
	), svc.handleSendFile)

	return mcpServer, nil
}

func (s *service) handleSendImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionContext, err := s.loadSessionContext(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	imageKey := strings.TrimSpace(request.GetString("image_key", ""))
	path := strings.TrimSpace(request.GetString("path", ""))
	caption := strings.TrimSpace(request.GetString("caption", ""))
	if imageKey == "" && path == "" {
		return mcp.NewToolResultError("send_image requires image_key or path"), nil
	}
	if imageKey == "" {
		if err := validatePathUnderRoot(path, sessionContext.ResourceRoot); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		uploadedImageKey, uploadErr := s.sender.UploadImage(ctx, path)
		if uploadErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("upload image failed: %v", uploadErr)), nil
		}
		imageKey = strings.TrimSpace(uploadedImageKey)
	}

	if sendErr := s.dispatchImage(ctx, sessionContext, imageKey); sendErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("send image failed: %v", sendErr)), nil
	}
	if caption != "" {
		if captionErr := s.dispatchText(ctx, sessionContext, caption); captionErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("send image caption failed: %v", captionErr)), nil
		}
	}

	return mcp.NewToolResultStructured(map[string]any{
		"status":    "ok",
		"type":      "image",
		"image_key": imageKey,
		"caption":   caption,
	}, "image sent"), nil
}

func (s *service) handleSendFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionContext, err := s.loadSessionContext(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	fileKey := strings.TrimSpace(request.GetString("file_key", ""))
	path := strings.TrimSpace(request.GetString("path", ""))
	fileName := strings.TrimSpace(request.GetString("file_name", ""))
	caption := strings.TrimSpace(request.GetString("caption", ""))
	if fileKey == "" && path == "" {
		return mcp.NewToolResultError("send_file requires file_key or path"), nil
	}
	if fileKey == "" {
		if err := validatePathUnderRoot(path, sessionContext.ResourceRoot); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		uploadedFileKey, uploadErr := s.sender.UploadFile(ctx, path, fileName)
		if uploadErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("upload file failed: %v", uploadErr)), nil
		}
		fileKey = strings.TrimSpace(uploadedFileKey)
	}

	if sendErr := s.dispatchFile(ctx, sessionContext, fileKey); sendErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("send file failed: %v", sendErr)), nil
	}
	if caption != "" {
		if captionErr := s.dispatchText(ctx, sessionContext, caption); captionErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("send file caption failed: %v", captionErr)), nil
		}
	}

	return mcp.NewToolResultStructured(map[string]any{
		"status":   "ok",
		"type":     "file",
		"file_key": fileKey,
		"caption":  caption,
	}, "file sent"), nil
}

func (s *service) loadSessionContext(request mcp.CallToolRequest) (mcpbridge.SessionContext, error) {
	sessionContext := mcpbridge.SessionContextFromEnv(s.getenv)
	if strings.TrimSpace(sessionContext.ReceiveIDType) == "" {
		sessionContext.ReceiveIDType = strings.TrimSpace(request.GetString("receive_id_type", ""))
	}
	if strings.TrimSpace(sessionContext.ReceiveID) == "" {
		sessionContext.ReceiveID = strings.TrimSpace(request.GetString("receive_id", ""))
	}
	if strings.TrimSpace(sessionContext.SourceMessageID) == "" {
		sessionContext.SourceMessageID = strings.TrimSpace(request.GetString("source_message_id", ""))
	}
	if err := sessionContext.Validate(); err != nil {
		return mcpbridge.SessionContext{}, fmt.Errorf(
			"mcp session context invalid: %w (fallback: provide receive_id_type and receive_id in tool arguments)",
			err,
		)
	}
	return sessionContext, nil
}

func (s *service) dispatchText(ctx context.Context, sessionContext mcpbridge.SessionContext, text string) error {
	sourceMessageID := strings.TrimSpace(sessionContext.SourceMessageID)
	if sourceMessageID != "" {
		if replySender, ok := s.sender.(replyTextSender); ok {
			_, err := replySender.ReplyText(ctx, sourceMessageID, text)
			return err
		}
	}
	return s.sender.SendText(ctx, sessionContext.ReceiveIDType, sessionContext.ReceiveID, text)
}

func (s *service) dispatchImage(ctx context.Context, sessionContext mcpbridge.SessionContext, imageKey string) error {
	sourceMessageID := strings.TrimSpace(sessionContext.SourceMessageID)
	if sourceMessageID != "" {
		if replySender, ok := s.sender.(replyImageSender); ok {
			_, err := replySender.ReplyImage(ctx, sourceMessageID, imageKey)
			return err
		}
	}
	return s.sender.SendImage(ctx, sessionContext.ReceiveIDType, sessionContext.ReceiveID, imageKey)
}

func (s *service) dispatchFile(ctx context.Context, sessionContext mcpbridge.SessionContext, fileKey string) error {
	sourceMessageID := strings.TrimSpace(sessionContext.SourceMessageID)
	if sourceMessageID != "" {
		if replySender, ok := s.sender.(replyFileSender); ok {
			_, err := replySender.ReplyFile(ctx, sourceMessageID, fileKey)
			return err
		}
	}
	return s.sender.SendFile(ctx, sessionContext.ReceiveIDType, sessionContext.ReceiveID, fileKey)
}

func validatePathUnderRoot(path string, root string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is empty")
	}
	if strings.TrimSpace(root) == "" {
		return nil
	}

	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	pathAbs = filepath.Clean(pathAbs)
	rootAbs = filepath.Clean(rootAbs)
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path out of allowed root: %s", rootAbs)
	}
	return nil
}
