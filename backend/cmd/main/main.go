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

	clickHouseConn, err := platform.NewClickHouseConn(ctx, platform.ClickHouseConfig{
		Addr:     cfg.ClickHouseAddr,
		Database: cfg.ClickHouseDatabase,
		Username: cfg.ClickHouseUsername,
		Password: cfg.ClickHousePassword,
	})
	if err != nil {
		logger.Warn("clickhouse unavailable; datasets sql studio disabled", "error", err)
	} else {
		defer clickHouseConn.Close()
	}

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
	fileStorage, err := service.NewFileStorage(ctx, service.FileStorageConfig{
		Driver:            cfg.FileStorageDriver,
		LocalDir:          cfg.FileLocalDir,
		S3Endpoint:        cfg.FileS3Endpoint,
		S3Region:          cfg.FileS3Region,
		S3Bucket:          cfg.FileS3Bucket,
		S3AccessKeyID:     cfg.FileS3AccessKeyID,
		S3SecretAccessKey: cfg.FileS3SecretAccessKey,
		S3ForcePathStyle:  cfg.FileS3ForcePathStyle,
	})
	if err != nil {
		fatal(logger, "create file storage", err)
	}
	fileService := service.NewFileService(pool, queries, fileStorage, tenantSettingsService, auditService, cfg.FileMaxBytes, cfg.FileAllowedMIMETypes, metrics)
	var openFGAClient service.OpenFGAClient
	if cfg.OpenFGA.Enabled {
		openFGAClient, err = service.NewOpenFGASDKClient(service.OpenFGAClientConfig{
			APIURL:               cfg.OpenFGA.APIURL,
			StoreID:              cfg.OpenFGA.StoreID,
			AuthorizationModelID: cfg.OpenFGA.AuthorizationModelID,
			APIToken:             cfg.OpenFGA.APIToken,
			Timeout:              cfg.OpenFGA.Timeout,
			Metrics:              metrics,
		})
		if err != nil {
			fatal(logger, "create openfga client", err)
		}
	}
	driveAuthorizationService := service.NewDriveAuthorizationService(openFGAClient, service.DriveAuthorizationConfig{
		Enabled:    cfg.OpenFGA.Enabled,
		FailClosed: cfg.OpenFGA.FailClosed,
		Metrics:    metrics,
	})
	driveService := service.NewDriveService(pool, queries, fileService, fileStorage, driveAuthorizationService, tenantSettingsService, auditService)
	driveService.SetOutboxService(outboxService)
	driveOCRService := service.NewDriveOCRService(pool, queries, driveService, fileStorage, tenantSettingsService, auditService, nil, nil)
	tenantInvitationService := service.NewTenantInvitationService(pool, queries, outboxService, auditService, cfg.InvitationTTL, cfg.FrontendBaseURL)
	tenantDataExportService := service.NewTenantDataExportService(pool, queries, outboxService, fileService, auditService, cfg.DataExportTTL, entitlementService)
	customerSignalImportService := service.NewCustomerSignalImportService(pool, queries, outboxService, fileService, entitlementService, auditService)
	datasetService := service.NewDatasetService(pool, queries, outboxService, fileService, auditService, clickHouseConn, service.DatasetClickHouseConfig{
		Addr:                cfg.ClickHouseAddr,
		Database:            cfg.ClickHouseDatabase,
		Username:            cfg.ClickHouseUsername,
		Password:            cfg.ClickHousePassword,
		TenantPasswordSalt:  cfg.ClickHouseTenantPasswordSalt,
		QueryMaxSeconds:     cfg.ClickHouseQueryMaxSeconds,
		QueryMaxMemoryBytes: cfg.ClickHouseQueryMaxMemoryBytes,
		QueryMaxRowsToRead:  cfg.ClickHouseQueryMaxRowsToRead,
		QueryMaxThreads:     cfg.ClickHouseQueryMaxThreads,
	})
	customerSignalSavedFilterService := service.NewCustomerSignalSavedFilterService(queries, entitlementService, auditService)
	supportAccessService := service.NewSupportAccessService(queries, sessionService, entitlementService, auditService, cfg.SupportAccessMaxDuration)
	emailSender := service.NewLogEmailSender(logger, cfg.EmailFrom)
	outboxHandler := service.NewOutboxHandler(emailSender, notificationService, tenantInvitationService, tenantDataExportService, webhookService, customerSignalImportService, driveOCRService, datasetService)

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
	dataLifecycleJob := jobs.NewDataLifecycleJob(queries, fileService, jobs.DataLifecycleConfig{
		Enabled:               cfg.DataLifecycleEnabled,
		Interval:              cfg.DataLifecycleInterval,
		Timeout:               cfg.DataLifecycleTimeout,
		RunOnStartup:          cfg.DataLifecycleRunOnStartup,
		OutboxRetention:       cfg.OutboxRetention,
		NotificationRetention: cfg.NotificationRetention,
		FileDeletedRetention:  cfg.FileDeletedRetention,
		FilePurgeBatchSize:    cfg.FilePurgeBatchSize,
		FilePurgeLockTimeout:  cfg.FilePurgeLockTimeout,
	}, logger, metrics)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg, logger, sessionService, oidcLoginService, delegationService, provisioningService, authzService, auditService, tenantAdminService, customerSignalService, todoService, machineClientService, outboxService, idempotencyService, notificationService, tenantInvitationService, fileService, tenantSettingsService, tenantDataExportService, bearerVerifier, m2mVerifier, redisClient, metrics, entitlementService, webhookService, customerSignalImportService, customerSignalSavedFilterService, supportAccessService, driveService, driveOCRService, datasetService)
	readiness := platform.ReadinessChecker{
		PostgresPing:  pool.Ping,
		RedisPing:     func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		ZitadelIssuer: cfg.ZitadelIssuer,
		CheckZitadel:  cfg.ReadinessCheckZitadel,
		OpenFGAURL:    cfg.OpenFGA.APIURL,
		OpenFGAToken:  cfg.OpenFGA.APIToken,
		CheckOpenFGA:  cfg.OpenFGA.Enabled,
		HTTPClient:    platform.ReadinessTimeoutClient(cfg.ReadinessTimeout),
		Metrics:       metrics,
	}
	if clickHouseConn != nil {
		readiness.ClickHousePing = func(ctx context.Context) error {
			return clickHouseConn.Ping(ctx)
		}
	}
	app.RegisterHealthRoutes(application.Router, readiness, cfg.ReadinessTimeout)
	if err := backendroot.RegisterFrontendRoutes(application.Router); err != nil {
		if errors.Is(err, backendroot.ErrFrontendNotEmbedded) {
			logger.Info("frontend routes not embedded; serve the frontend dev server separately", "frontend_base_url", cfg.FrontendBaseURL)
		} else {
			logger.Warn("frontend routes unavailable", "error", err)
		}
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
