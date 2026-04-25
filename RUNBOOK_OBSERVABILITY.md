# Observability Runbook

## HaoHaoScrapeDown

1. `/healthz` と `/readyz` を確認する。
2. process / container が起動しているか確認する。
3. `METRICS_ENABLED` と `METRICS_PATH` を確認する。
4. Prometheus から HaoHao への network / service discovery を確認する。

## HaoHaoHigh5xxRate

1. `haohao_http_requests_total{status_class="5xx"}` を route 別に見る。
2. 同時間帯の structured log を `status>=500` で検索する。
3. log の `request_id` / `trace_id` から trace を確認する。
4. `/readyz` と dependency ping metrics を確認する。

## HaoHaoHighLatency

1. p95 latency が上がっている route を確認する。
2. DB / Redis / Zitadel ping latency を確認する。
3. trace で遅い span が handler / DB / external dependency のどこかを見る。

## HaoHaoReadinessFailure

1. `/readyz` の JSON body で failing dependency を確認する。
2. `haohao_readiness_failures_total` の dependency label を確認する。
3. PostgreSQL / Redis / Zitadel の疎通、credential、network を確認する。

## HaoHaoDependencyPingSlow

1. `dependency` label が postgres / redis / zitadel のどれかを確認する。
2. dependency 側の saturation、connection 数、network latency を確認する。
3. app restart ではなく dependency 側の状態を優先して確認する。

## HaoHaoSCIMReconcileFailure

1. scheduler log の `provisioning reconcile failed` を確認する。
2. `trigger` が startup / interval のどちらか確認する。
3. delegated grant / SCIM mapping / provider availability を確認する。

## HaoHaoAuthFailureSpike

1. `kind` が external_bearer / m2m / scim のどれか確認する。
2. `reason` が missing_token / invalid_scope / invalid_role / tenant_denied / client_not_found などのどれか確認する。
3. provider 側の client / audience / scope / role 設定変更がないか確認する。
4. token や secret は log / issue / chat に貼らない。
