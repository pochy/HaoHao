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

	shutdownTracing, err := platform.InitTracing(ctx, platform.TracingConfig{
		Enabled:     cfg.OTELTracingEnabled,
		ServiceName: cfg.OTELServiceName,
		AppVersion:  cfg.AppVersion,
		Endpoint:    cfg.OTELExporterOTLPEndpoint,
		Insecure:    cfg.OTELExporterOTLPInsecure,
		SampleRatio: cfg.OTELTraceSampleRatio,
	}, logger)
	if err != nil {
		fatal(logger, "initialize tracing", err)
	}
	defer func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctxWithTimeout); err != nil {
			logger.Warn("shutdown tracing", "error", err)
		}
	}()

	var metrics *platform.Metrics
	if cfg.MetricsEnabled {
		metrics = platform.NewMetrics(cfg.AppVersion)
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
	outboxService := service.NewOutboxService(pool, queries, cfg.OutboxWorkerMaxAttempts)
	idempotencyService := service.NewIdempotencyService(queries, cfg.IdempotencyTTL)
	sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode, cfg.EnableLocalPasswordLogin, auditService)
	authzService := service.NewAuthzService(pool, queries)
	entitlementService := service.NewEntitlementService(queries, auditService)
	var webhookSecretBox *auth.SecretBox
	if cfg.WebhookSecretEncryptionKey != "" {
		webhookSecretBox, err = auth.NewSecretBox(cfg.WebhookSecretEncryptionKey, cfg.WebhookSecretKeyVersion)
		if err != nil {
			fatal(logger, "create webhook secret box", err)
		}
	}
	webhookService := service.NewWebhookService(queries, outboxService, entitlementService, auditService, webhookSecretBox, cfg.WebhookHTTPTimeout)
	tenantAdminService := service.NewTenantAdminService(pool, queries, auditService)
	customerSignalService := service.NewCustomerSignalService(pool, queries, auditService, webhookService)
	todoService := service.NewTodoService(pool, queries, auditService)
	machineClientService := service.NewMachineClientService(pool, queries, cfg.M2MRequiredScopePrefix, auditService)
	notificationService := service.NewNotificationService(queries, auditService)
	tenantSettingsService := service.NewTenantSettingsService(queries, auditService, cfg.TenantDefaultFileQuotaBytes)
	fileStorage := service.NewLocalFileStorage(cfg.FileLocalDir)
	fileService := service.NewFileService(pool, queries, fileStorage, tenantSettingsService, auditService, cfg.FileMaxBytes, cfg.FileAllowedMIMETypes, metrics)
	tenantInvitationService := service.NewTenantInvitationService(pool, queries, outboxService, auditService, cfg.InvitationTTL, cfg.FrontendBaseURL)
	tenantDataExportService := service.NewTenantDataExportService(pool, queries, outboxService, fileService, auditService, cfg.DataExportTTL, entitlementService)
	customerSignalImportService := service.NewCustomerSignalImportService(pool, queries, outboxService, fileService, entitlementService, auditService)
	customerSignalSavedFilterService := service.NewCustomerSignalSavedFilterService(queries, entitlementService, auditService)
	supportAccessService := service.NewSupportAccessService(queries, sessionService, entitlementService, auditService, cfg.SupportAccessMaxDuration)
	emailSender := service.NewLogEmailSender(logger, cfg.EmailFrom)
	outboxHandler := service.NewOutboxHandler(emailSender, notificationService, tenantInvitationService, tenantDataExportService, webhookService, customerSignalImportService)

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
	}, logger, metrics)
	outboxWorker := jobs.NewOutboxWorker(outboxService, outboxHandler, jobs.OutboxWorkerConfig{
		Enabled:   cfg.OutboxWorkerEnabled,
		Interval:  cfg.OutboxWorkerInterval,
		Timeout:   cfg.OutboxWorkerTimeout,
		BatchSize: cfg.OutboxWorkerBatchSize,
	}, logger, metrics)
	dataLifecycleJob := jobs.NewDataLifecycleJob(queries, jobs.DataLifecycleConfig{
		Enabled:               cfg.DataLifecycleEnabled,
		Interval:              cfg.DataLifecycleInterval,
		Timeout:               cfg.DataLifecycleTimeout,
		RunOnStartup:          cfg.DataLifecycleRunOnStartup,
		OutboxRetention:       cfg.OutboxRetention,
		NotificationRetention: cfg.NotificationRetention,
		FileDeletedRetention:  cfg.FileDeletedRetention,
	}, logger, metrics)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg, logger, sessionService, oidcLoginService, delegationService, provisioningService, authzService, auditService, tenantAdminService, customerSignalService, todoService, machineClientService, outboxService, idempotencyService, notificationService, tenantInvitationService, fileService, tenantSettingsService, tenantDataExportService, bearerVerifier, m2mVerifier, redisClient, metrics, entitlementService, webhookService, customerSignalImportService, customerSignalSavedFilterService, supportAccessService)
	app.RegisterHealthRoutes(application.Router, platform.ReadinessChecker{
		PostgresPing:  pool.Ping,
		RedisPing:     func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		ZitadelIssuer: cfg.ZitadelIssuer,
		CheckZitadel:  cfg.ReadinessCheckZitadel,
		HTTPClient:    platform.ReadinessTimeoutClient(cfg.ReadinessTimeout),
		Metrics:       metrics,
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
	go outboxWorker.Start(shutdownCtx)
	go dataLifecycleJob.Start(shutdownCtx)

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
