package main

//go:generate go run github.com/google/wire/cmd/wire

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/setup"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
)

//go:embed VERSION
var embeddedVersion string

// Build-time variables (can be set by ldflags)
var (
	Version          = ""
	Commit           = "unknown"
	Date             = "unknown"
	BuildType        = "source" // "source" for manual builds, "release" for CI builds (set by ldflags)
	Dirty            = "unknown"
	SourceHash       = "unknown"
	FrontendDistHash = "unknown"
)

func buildVersionLine() string {
	return fmt.Sprintf(
		"Sub2API version=%s commit=%s built=%s build_type=%s dirty=%s source_hash=%s frontend_dist_hash=%s",
		Version,
		Commit,
		Date,
		BuildType,
		Dirty,
		SourceHash,
		FrontendDistHash,
	)
}

func init() {
	// 如果 Version 已通过 ldflags 注入（例如 -X main.Version=...），则不要覆盖。
	if strings.TrimSpace(Version) != "" {
		return
	}

	// 默认从 embedded VERSION 文件读取版本号（编译期打包进二进制）。
	Version = strings.TrimSpace(embeddedVersion)
	if Version == "" {
		Version = "0.0.0-dev"
	}
}

// initLogger configures the default slog handler based on gin.Mode().
// In non-release mode, Debug level logs are enabled.
func main() {
	logger.InitBootstrap()
	exitCode := run()
	logger.Sync()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run() int {
	// Parse command line flags
	setupMode := flag.Bool("setup", false, "Run setup wizard in CLI mode")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Println(buildVersionLine())
		return 0
	}

	// CLI setup mode
	if *setupMode {
		if err := setup.RunCLI(); err != nil {
			log.Printf("Setup failed: %v", err)
			return 1
		}
		return 0
	}

	// Check if setup is needed
	if setup.NeedsSetup() {
		// Check if auto-setup is enabled (for Docker deployment)
		if setup.AutoSetupEnabled() {
			log.Println("Auto setup mode enabled...")
			if err := setup.AutoSetupFromEnv(); err != nil {
				log.Printf("Auto setup failed: %v", err)
				return 1
			}
			// Continue to main server after auto-setup
		} else {
			log.Println("First run detected, starting setup wizard...")
			if err := runSetupServer(); err != nil {
				log.Printf("Setup server failed: %v", err)
				return 1
			}
			return 0
		}
	}

	// Normal server mode
	if err := runMainServer(); err != nil {
		log.Printf("%v", err)
		return 1
	}
	return 0
}

func runSetupServer() error {
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(config.CORSConfig{}))
	r.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true, Policy: config.DefaultCSPPolicy}, nil))

	// Register setup routes
	setup.RegisterRoutes(r)

	// Serve embedded frontend if available
	if web.HasEmbeddedFrontend() {
		r.Use(web.ServeEmbeddedFrontend())
	}

	// Get server address from config.yaml or environment variables (SERVER_HOST, SERVER_PORT)
	// This allows users to run setup on a different address if needed
	addr := config.GetServerAddress()
	log.Printf("Setup wizard available at http://%s", addr)
	log.Println("Complete the setup wizard to configure Sub2API")

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
		Protocols:         protocols,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start setup server: %w", err)
	}
	return nil
}

func runMainServer() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := logger.Init(logger.OptionsFromConfig(cfg.Log)); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	if cfg.RunMode == config.RunModeSimple {
		log.Println("⚠️  WARNING: Running in SIMPLE mode - billing and quota checks are DISABLED")
	}

	buildInfo := handler.BuildInfo{
		Version:   Version,
		BuildType: BuildType,
	}

	app, err := initializeApplication(buildInfo)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	return serveApplication(app, nil)
}

func serveApplication(app *Application, quit <-chan os.Signal) error {
	defer func() {
		service.StopDefaultUserPlatformQuotaDBAggregator()
		app.Cleanup()
	}()

	listener, err := net.Listen("tcp", app.Server.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind server on %s: %w", app.Server.Addr, err)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := app.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	log.Printf("Server started on %s", listener.Addr().String())

	// 等待中断信号
	if quit == nil {
		signalQuit := make(chan os.Signal, 1)
		signal.Notify(signalQuit, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalQuit)
		quit = signalQuit
	}

	select {
	case err := <-serverErrCh:
		return fmt.Errorf("server stopped unexpectedly: %w", err)
	case <-quit:
	}

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout())
	defer cancel()

	if err := app.Server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited")
	return nil
}

func serverShutdownTimeout() time.Duration {
	const defaultTimeout = 10 * time.Second

	raw := strings.TrimSpace(os.Getenv("SERVER_SHUTDOWN_TIMEOUT"))
	if raw == "" {
		return defaultTimeout
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultTimeout
	}
	return timeout
}
