package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := platform.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	redisClient, err := platform.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	queries := db.New(pool)
	sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode, cfg.EnableLocalPasswordLogin)
	authzService := service.NewAuthzService(pool, queries)
	machineClientService := service.NewMachineClientService(queries, cfg.M2MRequiredScopePrefix)

	var oidcLoginService *service.OIDCLoginService
	var delegationService *service.DelegationService
	var bearerVerifier *auth.BearerVerifier
	var m2mVerifier *auth.M2MVerifier
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			log.Fatal("ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel")
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
			log.Fatal(err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, authzService, sessionService)

		if cfg.DownstreamTokenEncryptionKey != "" {
			refreshTokenStore, err := auth.NewRefreshTokenStore(cfg.DownstreamTokenEncryptionKey, cfg.DownstreamTokenKeyVersion)
			if err != nil {
				log.Fatal(err)
			}

			delegatedOAuthClient, err := auth.NewDelegatedOAuthClient(ctx, cfg.ZitadelIssuer, cfg.ZitadelClientID, cfg.ZitadelClientSecret)
			if err != nil {
				log.Fatal(err)
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
			)
		}
	}

	if cfg.ZitadelIssuer != "" {
		bearerVerifier, err = auth.NewBearerVerifier(ctx, cfg.ZitadelIssuer)
		if err != nil {
			log.Fatal(err)
		}
		m2mVerifier = auth.NewM2MVerifier(bearerVerifier, cfg.M2MExpectedAudience, cfg.M2MRequiredScopePrefix)
	}

	provisioningService := service.NewProvisioningService(pool, queries, sessionService, delegationService, authzService)

	application := app.New(cfg, sessionService, oidcLoginService, delegationService, provisioningService, authzService, machineClientService, bearerVerifier, m2mVerifier)
	if err := backendroot.RegisterFrontendRoutes(application.Router); err != nil {
		log.Printf("frontend routes unavailable: %v", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           application.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on http://127.0.0.1:%d", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-shutdownCtx.Done()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxWithTimeout); err != nil {
		log.Fatal(err)
	}
}
