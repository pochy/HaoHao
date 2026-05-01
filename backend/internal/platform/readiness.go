package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PingFunc func(context.Context) error

type ReadinessChecker struct {
	PostgresPing   PingFunc
	RedisPing      PingFunc
	ClickHousePing PingFunc
	ZitadelIssuer  string
	CheckZitadel   bool
	OpenFGAURL     string
	OpenFGAToken   string
	CheckOpenFGA   bool
	HTTPClient     *http.Client
	Metrics        *Metrics
}

type ReadinessResult struct {
	Status string                    `json:"status"`
	Checks map[string]ReadinessCheck `json:"checks"`
}

type ReadinessCheck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (c ReadinessChecker) Check(ctx context.Context) ReadinessResult {
	result := ReadinessResult{
		Status: "ok",
		Checks: map[string]ReadinessCheck{},
	}

	c.checkPing(ctx, &result, "postgres", c.PostgresPing)
	c.checkPing(ctx, &result, "redis", c.RedisPing)
	if c.ClickHousePing != nil {
		c.checkPing(ctx, &result, "clickhouse", c.ClickHousePing)
	}

	if c.CheckZitadel {
		startedAt := time.Now()
		if err := c.checkZitadel(ctx); err != nil {
			if c.Metrics != nil {
				c.Metrics.ObserveDependencyPing("zitadel", time.Since(startedAt), err)
			}
			result.Status = "error"
			result.Checks["zitadel"] = ReadinessCheck{Status: "error", Error: err.Error()}
		} else {
			if c.Metrics != nil {
				c.Metrics.ObserveDependencyPing("zitadel", time.Since(startedAt), nil)
			}
			result.Checks["zitadel"] = ReadinessCheck{Status: "ok"}
		}
	}

	if c.CheckOpenFGA {
		startedAt := time.Now()
		if err := c.checkOpenFGA(ctx); err != nil {
			if c.Metrics != nil {
				c.Metrics.ObserveDependencyPing("openfga", time.Since(startedAt), err)
			}
			result.Status = "error"
			result.Checks["openfga"] = ReadinessCheck{Status: "error", Error: err.Error()}
		} else {
			if c.Metrics != nil {
				c.Metrics.ObserveDependencyPing("openfga", time.Since(startedAt), nil)
			}
			result.Checks["openfga"] = ReadinessCheck{Status: "ok"}
		}
	}

	return result
}

func (c ReadinessChecker) checkPing(ctx context.Context, result *ReadinessResult, name string, ping PingFunc) {
	startedAt := time.Now()
	if ping == nil {
		err := fmt.Errorf("ping function is not configured")
		if c.Metrics != nil {
			c.Metrics.ObserveDependencyPing(name, time.Since(startedAt), err)
		}
		result.Status = "error"
		result.Checks[name] = ReadinessCheck{Status: "error", Error: err.Error()}
		return
	}

	if err := ping(ctx); err != nil {
		if c.Metrics != nil {
			c.Metrics.ObserveDependencyPing(name, time.Since(startedAt), err)
		}
		result.Status = "error"
		result.Checks[name] = ReadinessCheck{Status: "error", Error: err.Error()}
		return
	}

	if c.Metrics != nil {
		c.Metrics.ObserveDependencyPing(name, time.Since(startedAt), nil)
	}
	result.Checks[name] = ReadinessCheck{Status: "ok"}
}

func (c ReadinessChecker) checkZitadel(ctx context.Context) error {
	issuer := strings.TrimRight(c.ZitadelIssuer, "/")
	if issuer == "" {
		return fmt.Errorf("ZITADEL_ISSUER is empty")
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	if body["issuer"] == nil {
		return fmt.Errorf("discovery response missing issuer")
	}

	return nil
}

func (c ReadinessChecker) checkOpenFGA(ctx context.Context) error {
	baseURL := strings.TrimRight(strings.TrimSpace(c.OpenFGAURL), "/")
	if baseURL == "" {
		return fmt.Errorf("OPENFGA_API_URL is empty")
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	if token := strings.TrimSpace(c.OpenFGAToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("healthz returned %d", resp.StatusCode)
	}

	return nil
}

func ReadinessTimeoutClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
