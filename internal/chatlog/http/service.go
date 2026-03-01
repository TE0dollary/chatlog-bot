package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/database"
	"github.com/TE0dollary/chatlog-bot/internal/errors"
)

// Service 是 HTTP/MCP 服务器，基于 Gin 框架。
// 提供以下能力：
//   - REST API（/api/v1/*）：查询聊天记录、联系人、群聊、会话，支持 text/csv/json 格式
//   - 媒体文件服务（/image/*, /video/*, /voice/*, /file/*）：通过 MD5 或文件名获取媒体
//   - MCP 协议（/mcp, /sse, /message）：支持 SSE 和 Streamable HTTP 两种传输方式
//   - 内嵌 Web 前端（/static/*）
//   - 健康检查（/health）
type Service struct {
	conf Config
	db   *database.Service

	router *gin.Engine  // Gin 路由引擎
	server *http.Server // 底层 HTTP 服务器

	mcpServer           *server.MCPServer           // MCP 协议核心服务器
	mcpSSEServer        *server.SSEServer           // MCP SSE 传输
	mcpStreamableServer *server.StreamableHTTPServer // MCP Streamable HTTP 传输
}

type Config interface {
	GetHTTPAddr() string
	GetDataDir() string
	GetSaveDecryptedMedia() bool
}

// NewService 创建 HTTP 服务实例，初始化 Gin 路由、中间件和 MCP 服务器。
// 中间件包括：错误恢复、统一错误处理、请求日志、CORS 跨域支持。
func NewService(conf Config, db *database.Service) *Service {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Handle error from SetTrustedProxies
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Err(err).Msg("Failed to set trusted proxies")
	}

	// Middleware
	router.Use(
		errors.RecoveryMiddleware(),
		errors.ErrorHandlerMiddleware(),
		gin.LoggerWithWriter(log.Logger, "/health"),
		corsMiddleware(),
	)

	s := &Service{
		conf:   conf,
		db:     db,
		router: router,
	}

	s.initMCPServer()
	s.initRouter()
	return s
}

// Start 非阻塞地启动 HTTP 服务器（用于 TUI 模式，在后台 goroutine 中运行）。
func (s *Service) Start() error {

	s.server = &http.Server{
		Addr:    s.conf.GetHTTPAddr(),
		Handler: s.router,
	}

	go func() {
		// Handle error from Run
		if err := s.server.ListenAndServe(); err != nil {
			log.Err(err).Msg("Failed to start HTTP server")
		}
	}()

	log.Info().Msg("Starting HTTP server on " + s.conf.GetHTTPAddr())

	return nil
}

// ListenAndServe 阻塞式启动 HTTP 服务器（用于 server 命令模式）。
func (s *Service) ListenAndServe() error {

	s.server = &http.Server{
		Addr:    s.conf.GetHTTPAddr(),
		Handler: s.router,
	}

	log.Info().Msg("Starting HTTP server on " + s.conf.GetHTTPAddr())
	return s.server.ListenAndServe()
}

// Stop 优雅关闭 HTTP 服务器（2 秒超时）。
func (s *Service) Stop() error {

	if s.server == nil {
		return nil
	}

	// 使用超时上下文优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Debug().Err(err).Msg("Failed to shutdown HTTP server")
		return nil
	}

	log.Info().Msg("HTTP server stopped")
	return nil
}

