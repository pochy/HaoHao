package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func ExternalCORS(pathPrefix string, allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) {
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && originAllowed(origin, allowed) {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID")
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || !originAllowed(origin, allowed) {
				writeProblem(c, http.StatusForbidden, "origin is not allowed")
				return
			}

			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}

func ExternalAuth(pathPrefix string, verifier *auth.BearerVerifier, authzService *service.AuthzService, providerName, expectedAudience, requiredScopePrefix, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil || authzService == nil {
			writeProblem(c, http.StatusServiceUnavailable, "external bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, requiredScopePrefix)
		if err != nil {
			status := http.StatusUnauthorized
			switch {
			case err == auth.ErrInvalidBearerScope:
				status = http.StatusForbidden
			case err == auth.ErrInvalidBearerAudience, err == auth.ErrInvalidBearerIssuer, err == auth.ErrMissingBearerToken:
				status = http.StatusUnauthorized
			}
			writeBearerProblem(c, status, err.Error())
			return
		}

		authCtx, err := authzService.AuthContextFromBearerWithTenant(c.Request.Context(), providerName, claims, c.GetHeader("X-Tenant-ID"))
		if err != nil {
			if err == service.ErrUnauthorized {
				writeBearerProblem(c, http.StatusForbidden, "tenant access denied")
				return
			}
			writeProblem(c, http.StatusInternalServerError, "failed to build auth context")
			return
		}
		if !authCtx.HasProviderRole(requiredRole) {
			writeBearerProblem(c, http.StatusForbidden, auth.ErrInvalidBearerRole.Error())
			return
		}

		c.Request = c.Request.WithContext(service.ContextWithAuthContext(c.Request.Context(), authCtx))
		c.Next()
	}
}

func SCIMAuth(pathPrefix string, verifier *auth.BearerVerifier, expectedAudience, requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil {
			writeProblem(c, http.StatusServiceUnavailable, "scim bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, "")
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}
		if !claims.HasScope(requiredScope) {
			writeBearerProblem(c, http.StatusForbidden, auth.ErrInvalidBearerScope.Error())
			return
		}

		c.Next()
	}
}

func M2MAuth(pathPrefix string, verifier *auth.M2MVerifier, machineClientService *service.MachineClientService, providerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil || machineClientService == nil {
			writeProblem(c, http.StatusServiceUnavailable, "m2m bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblemForRealm(c, "haohao-m2m", http.StatusUnauthorized, err.Error())
			return
		}

		principal, err := verifier.Verify(c.Request.Context(), rawToken)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, auth.ErrInvalidBearerScope) || errors.Is(err, auth.ErrHumanBearerToken) {
				status = http.StatusForbidden
			}
			writeBearerProblemForRealm(c, "haohao-m2m", status, err.Error())
			return
		}

		machineCtx, err := machineClientService.AuthenticateM2M(c.Request.Context(), providerName, principal)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrMachineClientNotFound) ||
				errors.Is(err, service.ErrMachineClientInactive) ||
				errors.Is(err, service.ErrMachineClientScopeDenied) {
				status = http.StatusForbidden
			}
			writeBearerProblemForRealm(c, "haohao-m2m", status, err.Error())
			return
		}

		c.Request = c.Request.WithContext(service.ContextWithMachineClient(c.Request.Context(), machineCtx))
		c.Next()
	}
}

func DocsAuth(required bool, sessionService *service.SessionService, authzService *service.AuthzService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !required || !isDocsPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		if sessionService == nil || authzService == nil {
			writeProblem(c, http.StatusServiceUnavailable, "docs auth is not configured")
			return
		}

		sessionCookie, err := c.Request.Cookie(auth.SessionCookieName)
		if err != nil || strings.TrimSpace(sessionCookie.Value) == "" {
			writeProblem(c, http.StatusUnauthorized, "missing or expired session")
			return
		}

		current, err := sessionService.CurrentSession(c.Request.Context(), sessionCookie.Value)
		if err != nil {
			writeProblem(c, http.StatusUnauthorized, "missing or expired session")
			return
		}
		authCtx, err := authzService.BuildBrowserContext(c.Request.Context(), current.User, current.ActiveTenantID)
		if err != nil {
			if errors.Is(err, service.ErrUnauthorized) {
				writeProblem(c, http.StatusForbidden, "docs access denied")
				return
			}
			writeProblem(c, http.StatusInternalServerError, "failed to build auth context")
			return
		}
		if !authCtx.HasRole("docs_reader") {
			writeProblem(c, http.StatusForbidden, "docs_reader role is required")
			return
		}

		c.Next()
	}
}

func bearerTokenFromHeader(header string) (string, error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return "", auth.ErrMissingBearerToken
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", fmt.Errorf("%w: authorization header must use Bearer", auth.ErrInvalidBearerToken)
	}

	token := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if token == "" {
		return "", auth.ErrMissingBearerToken
	}

	return token, nil
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return false
	}
	_, ok := allowed[origin]
	return ok
}

func writeBearerProblem(c *gin.Context, status int, detail string) {
	writeBearerProblemForRealm(c, "haohao-external", status, detail)
}

func writeBearerProblemForRealm(c *gin.Context, realm string, status int, detail string) {
	c.Header("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
	writeProblem(c, status, detail)
}

func writeProblem(c *gin.Context, status int, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	})
}

func isDocsPath(path string) bool {
	return path == "/docs" ||
		strings.HasPrefix(path, "/docs/") ||
		path == "/openapi.json" ||
		path == "/openapi.yaml"
}
