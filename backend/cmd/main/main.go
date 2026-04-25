package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	backendroot "example.com/haohao/backend"
	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/jobs"
	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger := platform.NewLogger(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	slog.SetDefault(logger)
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	pool, err := platform.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal(logger, "connect postgres", err)
	}
	defer pool.Close()

	redisClient, err := platform.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		fatal(logger, "connect redis", err)
	}
	defer redisClient.Close()

	queries := db.New(pool)
	auditService := service.NewAuditService(queries)
	sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode, cfg.EnableLocalPasswordLogin, auditService)
	authzService := service.NewAuthzService(pool, queries)
	todoService := service.NewTodoService(pool, queries, auditService)
	machineClientService := service.NewMachineClientService(pool, queries, cfg.M2MRequiredScopePrefix, auditService)

	var oidcLoginService *service.OIDCLoginService
	var delegationService *service.DelegationService
	var bearerVerifier *auth.BearerVerifier
	var m2mVerifier *auth.M2MVerifier
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			fatal(logger, "ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel", nil)
		}

		oidcClient, err := auth.NewOIDCClient(
			ctx,
			cfg.ZitadelIssuer,
			cfg.ZitadelClientID,
			cfg.ZitadelClientSecret,
			cfg.ZitadelRedirectURI,
			cfg.ZitadelScopes,
		)
		if err != nil {
			fatal(logger, "create oidc client", err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, authzService, sessionService)

		if cfg.DownstreamTokenEncryptionKey != "" {
			refreshTokenStore, err := auth.NewRefreshTokenStore(cfg.DownstreamTokenEncryptionKey, cfg.DownstreamTokenKeyVersion)
			if err != nil {
				fatal(logger, "create refresh token store", err)
			}

			delegatedOAuthClient, err := auth.NewDelegatedOAuthClient(ctx, cfg.ZitadelIssuer, cfg.ZitadelClientID, cfg.ZitadelClientSecret)
			if err != nil {
				fatal(logger, "create delegated oauth client", err)
			}

			delegationStateStore := auth.NewDelegationStateStore(redisClient, cfg.LoginStateTTL)
			delegationService = service.NewDelegationService(
				queries,
				delegatedOAuthClient,
				delegationStateStore,
				refreshTokenStore,
				cfg.AppBaseURL,
				cfg.DownstreamDefaultScopes,
				cfg.DownstreamRefreshTokenTTL,
				cfg.DownstreamAccessTokenSkew,
				auditService,
			)
		}
	}

	if cfg.ZitadelIssuer != "" {
		bearerVerifier, err = auth.NewBearerVerifier(ctx, cfg.ZitadelIssuer)
		if err != nil {
			fatal(logger, "create bearer verifier", err)
		}
		m2mVerifier = auth.NewM2MVerifier(bearerVerifier, cfg.M2MExpectedAudience, cfg.M2MRequiredScopePrefix)
	}

	provisioningService := service.NewProvisioningService(pool, queries, sessionService, delegationService, authzService)
	reconcileJob := jobs.NewProvisioningReconcileJob(queries, sessionService, delegationService)
	reconcileScheduler := jobs.NewReconcileScheduler(reconcileJob, jobs.ReconcileSchedulerConfig{
		Enabled:      cfg.SCIMReconcileEnabled,
		Interval:     cfg.SCIMReconcileInterval,
		Timeout:      cfg.SCIMReconcileTimeout,
		RunOnStartup: cfg.SCIMReconcileRunOnStartup,
	}, logger)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg, logger, sessionService, oidcLoginService, delegationService, provisioningService, authzService, auditService, todoService, machineClientService, bearerVerifier, m2mVerifier)
	app.RegisterHealthRoutes(application.Router, platform.ReadinessChecker{
		PostgresPing:  pool.Ping,
		RedisPing:     func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		ZitadelIssuer: cfg.ZitadelIssuer,
		CheckZitadel:  cfg.ReadinessCheckZitadel,
		HTTPClient:    platform.ReadinessTimeoutClient(cfg.ReadinessTimeout),
	}, cfg.ReadinessTimeout)
	if err := backendroot.RegisterFrontendRoutes(application.Router); err != nil {
		logger.Warn("frontend routes unavailable", "error", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           application.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go reconcileScheduler.Start(shutdownCtx)

	go func() {
		logger.Info("listening", "url", fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(logger, "serve http", err)
		}
	}()

	<-shutdownCtx.Done()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxWithTimeout); err != nil {
		fatal(logger, "shutdown http server", err)
	}
}

func fatal(logger *slog.Logger, message string, err error) {
	if err != nil {
		logger.Error(message, "error", err)
	} else {
		logger.Error(message)
	}
	os.Exit(1)
}
