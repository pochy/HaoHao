package config

import "strings"

func defaultFrontendBaseURL(appBaseURL string, embedded bool) string {
	if embedded {
		return appBaseURL
	}
	return "http://127.0.0.1:5173"
}

func resolveFrontendBaseURL(appBaseURL, configuredFrontendBaseURL string, embedded bool) string {
	frontendBaseURL := strings.TrimRight(configuredFrontendBaseURL, "/")
	if embedded && isViteDevBaseURL(frontendBaseURL) {
		return appBaseURL
	}
	return frontendBaseURL
}

func defaultZitadelPostLogoutRedirectURI(frontendBaseURL string) string {
	return strings.TrimRight(frontendBaseURL, "/") + "/login"
}

func resolveZitadelPostLogoutRedirectURI(frontendBaseURL, configuredPostLogoutRedirectURI string, embedded bool) string {
	postLogoutRedirectURI := strings.TrimRight(configuredPostLogoutRedirectURI, "/")
	if embedded && isViteDevPostLogoutRedirectURI(postLogoutRedirectURI) {
		return defaultZitadelPostLogoutRedirectURI(frontendBaseURL)
	}
	return postLogoutRedirectURI
}

func isViteDevBaseURL(value string) bool {
	return value == "http://127.0.0.1:5173" ||
		value == "http://localhost:5173"
}

func isViteDevPostLogoutRedirectURI(value string) bool {
	return value == "http://127.0.0.1:5173/login" ||
		value == "http://localhost:5173/login"
}
