# P11 tenant settings rate limit runtime 連動チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **tenant settings の rate limit runtime 連動** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P7 では `tenant_settings` に rate limit override の入口を作り、Tenant Admin UI から `rateLimitBrowserApiPerMinute` を保存できるようにしました。一方で、runtime の rate limit middleware はまだ `RATE_LIMIT_BROWSER_API_PER_MINUTE` などの config 固定値だけを見ています。

P11 では、このずれを閉じます。既存の Tenant Settings API と UI はそのまま使い、`browser_api` policy の rate limit decision だけを runtime で tenant settings に連動させます。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P7_WEB_SERVICE_COMMON.md` / `TUTORIAL_P10_CROSS_CUTTING_EXTENSIONS.md` と同じように、対象ファイル、主要コード方針、確認コマンド、失敗時の見方まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- P7 の `0012_web_service_common` migration が適用済み
- `tenant_settings` に `rate_limit_login_per_minute`、`rate_limit_browser_api_per_minute`、`rate_limit_external_api_per_minute` がある
- `TenantSettingsService.Get` / `Update` がある
- `GET /api/v1/admin/tenants/{tenantSlug}/settings` と `PUT /api/v1/admin/tenants/{tenantSlug}/settings` がある
- Tenant Admin detail UI で `Browser API limit / minute` を表示 / 更新できる
- `backend/internal/middleware/rate_limit.go` に Redis fixed window rate limit middleware がある
- rate limit middleware は `login`、`browser_api`、`external_api` の policy を判定できる
- 現在の middleware は `RateLimitConfig` の config 値だけを limit として使っている
- `/metrics` に `haohao_rate_limit_total{policy,result}` が出る
- P8 により `openapi/browser.yaml` が frontend generated SDK の入力になっている
- P9 の Playwright E2E が single binary に対して動く
- P10 の support access / impersonation が入っている場合は、`CurrentSession.ActorUser` と `CurrentSession.SupportAccess` が使える

この P11 では、DB schema、OpenAPI schema、frontend generated SDK は原則変更しません。既存 API の意味を runtime に接続する作業です。

## 完成条件

このチュートリアルの完了条件は次です。

- `browser_api` policy の rate limit decision が active tenant の `rateLimitBrowserApiPerMinute` を反映する
- override が `null` または tenant settings row がない場合は `RATE_LIMIT_BROWSER_API_PER_MINUTE` の config default を使う
- active tenant がない request、未ログイン request、session 解決に失敗した request は config default を使う
- `login` policy と `external_api` policy は P11 では config default のままにする
- browser API の rate limit bucket は `tenant + requesting actor/user` 単位で分離される
- support access 中は impersonated user ではなく support actor user を requester として bucket を作る
- tenant settings lookup 失敗時は config default に fallback する
- Redis failure は既存通り fail-open にする
- rate limit 超過時は `429` と `Retry-After` header を返す
- metrics label は `policy` と `result` だけを維持し、tenant id、user id、email、session id、IP、route tenant slug、idempotency key を入れない
- unit test で override、fallback、bucket 分離、support access actor、Redis failure を確認できる
- smoke で tenant settings 更新後に runtime の `429` が出ることを確認できる
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make smoke-common-services`、`make smoke-rate-limit-runtime`、`make e2e` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | P11 境界 | 対象 policy と public interface を固定する |
| Step 2 | `TenantSettingsService` | effective rate limit resolver を追加する |
| Step 3 | `backend/internal/middleware/rate_limit.go` | config 固定から resolver 対応へ拡張する |
| Step 4 | `backend/internal/app/app.go` | runtime resolver を wire する |
| Step 5 | bucket key | `tenant + requester` 単位の key にする |
| Step 6 | fallback | settings lookup と Redis failure の扱いを固定する |
| Step 7 | unit test / smoke | 自動確認を追加する |
| Step 8 | 生成と確認 | build、smoke、E2E、失敗時の見方を確認する |

## 先に決める方針

### Public interface は増やさない

P11 では API route や request / response schema を増やしません。

既存の public interface は次のままです。

```text
GET /api/v1/admin/tenants/{tenantSlug}/settings
PUT /api/v1/admin/tenants/{tenantSlug}/settings
```

P11 で変わるのは、`PUT` で保存済みの `rateLimitBrowserApiPerMinute` が runtime の `browser_api` rate limit decision に反映されることだけです。

このため、通常は次の差分は出ません。

- DB migration
- sqlc query
- OpenAPI schema
- frontend generated SDK
- Vue UI

`make gen` は実行しますが、P11 の本質は generated artifact を増やすことではなく、既存設定を middleware に接続することです。

### 対象は `browser_api` に絞る

P11 で tenant settings override を runtime 反映する対象は `browser_api` だけにします。

```text
login        -> config default
browser_api  -> tenant settings override, fallback config default
external_api -> config default
```

理由は、`login` は未認証で tenant がないことが多く、`external_api` は bearer / M2M / SCIM の tenant 解決方式が browser session と異なるためです。これらは後続で個別に設計します。

### Bucket は `tenant + requester` 単位にする

P7 初期版では IP hash だけでも最小の防御として機能します。しかし tenant settings override を入れる P11 では、同一 user が複数 tenant を切り替える状況と、複数 user が同一 NAT の後ろにいる状況を分ける必要があります。

P11 の browser API bucket は次の意味にします。

```text
policy=browser_api
tenant=active tenant id
requester=current user id, or support actor user id
window=minute
```

Redis key は raw id を直接含めず、hash 済みの bucket key にします。

例:

```text
rate_limit:browser_api:tenant_user:{sha256(tenantID + requesterID)}:{yyyymmddhhmm}
```

Redis key は metrics label ではありませんが、運用中に Redis key を見ることがあるため、tenant id や user id を平文では入れません。

### support access は actor を requester にする

P10 の support access 中は、`CurrentSession.User` が impersonated user になり、`CurrentSession.ActorUser` に support agent が入ります。

rate limit の bucket は、操作している本人である support actor user を使います。

```go
requester := current.User
if current.ActorUser != nil {
    requester = *current.ActorUser
}
```

これにより、support agent が複数 user を切り替えても、rate limit を impersonated user ごとに分散して回避できません。

### settings lookup 失敗は config default に fallback する

tenant settings lookup に失敗しても request 全体を `500` にしません。rate limit は防御層なので、settings DB lookup 障害時は config default に戻して処理を続けます。

一方で、Redis が落ちている場合は既存通り fail-open にします。Redis failure で全 browser API を止めると、rate limit 障害が product API 障害へ広がるためです。

P11 の fallback は次の優先順位です。

1. active tenant があり、settings lookup に成功し、override がある場合は override
2. active tenant があり、settings lookup に成功し、override がない場合は config default
3. active tenant がない場合は config default
4. session / settings lookup に失敗した場合は config default
5. Redis increment に失敗した場合は fail-open

### metrics label は増やさない

P11 後も rate limit metrics は次の形を維持します。

```text
haohao_rate_limit_total{policy="browser_api",result="allowed"}
haohao_rate_limit_total{policy="browser_api",result="blocked"}
haohao_rate_limit_total{policy="browser_api",result="error"}
```

次は label に入れません。

- tenant id
- tenant slug
- user id
- actor user id
- email
- session id
- IP address
- route path parameter
- idempotency key

tenant ごとの調査が必要な場合は metrics label ではなく、structured log、audit event、または一時的な debug query で見ます。

## Step 1. P11 の境界を固定する

### 1-1. 変更対象を確認する

まず、実装対象が既存の P7 rate limit と tenant settings の接続だけであることを確認します。

```bash
rg -n "RateLimit|rate_limit|TenantSettings|rateLimitBrowserApiPerMinute" backend frontend scripts e2e
```

確認する現在地は次です。

- `backend/internal/middleware/rate_limit.go` の `RateLimitConfig` が config 値だけを持つ
- `backend/internal/app/app.go` が `cfg.RateLimitBrowserAPIPerMinute` をそのまま middleware に渡している
- `backend/internal/service/tenant_settings_service.go` が `RateLimitBrowserAPIPerMinute` を保持している
- `frontend/src/views/TenantAdminTenantDetailView.vue` が `tenant-browser-rate-limit` input を持っている

### 1-2. やらないことを明記する

P11 では次をやりません。

- `tenant_settings` table の column 追加
- new migration
- `openapi/browser.yaml` の schema 変更
- frontend SDK の operation 追加
- Tenant Admin UI の新画面追加
- `login` policy の tenant override
- `external_api` policy の tenant override
- sliding window / token bucket への置き換え
- per route rate limit

ここを広げると、P11 の目的である「保存済み browser API override を runtime に反映する」が曖昧になります。

## Step 2. TenantSettingsService に effective rate limit resolver を追加する

### 2-1. defaults type を追加する

#### ファイル: `backend/internal/service/tenant_settings_service.go`

service 層に、config default を渡すための小さな type を追加します。

```go
type RateLimitDefaults struct {
    LoginPerMinute       int
    BrowserAPIPerMinute  int
    ExternalAPIPerMinute int
}
```

middleware package の型を service package に import しないでください。service は domain / DB 側の責務に留め、middleware の都合を持ち込まないようにします。

### 2-2. resolver method を追加する

同じファイルに resolver を追加します。

```go
func (s *TenantSettingsService) ResolveEffectiveRateLimit(
    ctx context.Context,
    tenantID int64,
    policy string,
    defaults RateLimitDefaults,
) (int, error) {
    settings, err := s.Get(ctx, tenantID)
    if err != nil {
        return defaultRateLimitForPolicy(policy, defaults), err
    }

    switch policy {
    case "browser_api":
        if settings.RateLimitBrowserAPIPerMinute != nil {
            return int(*settings.RateLimitBrowserAPIPerMinute), nil
        }
    case "login":
        if settings.RateLimitLoginPerMinute != nil {
            return int(*settings.RateLimitLoginPerMinute), nil
        }
    case "external_api":
        if settings.RateLimitExternalAPIPerMinute != nil {
            return int(*settings.RateLimitExternalAPIPerMinute), nil
        }
    }
    return defaultRateLimitForPolicy(policy, defaults), nil
}
```

P11 の runtime wiring では `browser_api` だけを呼びます。ただし method 自体は既存 column に合わせて policy を受け取る形にしておくと、後で `external_api` を追加するときに `tenant_settings` 側を作り直さずに済みます。

### 2-3. default helper を追加する

```go
func defaultRateLimitForPolicy(policy string, defaults RateLimitDefaults) int {
    switch policy {
    case "login":
        return defaults.LoginPerMinute
    case "external_api":
        return defaults.ExternalAPIPerMinute
    default:
        return defaults.BrowserAPIPerMinute
    }
}
```

`policy` が未知の場合は `BrowserAPIPerMinute` に寄せます。middleware 側で未知 policy は基本的に渡さないため、この fallback は defensive default です。

### 2-4. validation は既存方針を維持する

`normalizeTenantSettingsInput` には既に rate limit override が positive であることを確認する処理があります。

P11 では validation を増やしません。

- `nil`: config default を使う
- `1` 以上: override として使う
- `0` 以下: update API で reject

## Step 3. rate limit middleware を resolver 対応へ拡張する

### 3-1. decision type を追加する

#### ファイル: `backend/internal/middleware/rate_limit.go`

middleware package に、runtime resolver の戻り値を表す type を追加します。

```go
type RateLimitDecision struct {
    Policy         string
    LimitPerMinute int
    BucketKey      string
}

type RateLimitResolver func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error)
```

`BucketKey` は raw id ではなく、hash 済みまたは非機密の key fragment にします。

### 3-2. config に resolver を追加する

```go
type RateLimitConfig struct {
    Enabled              bool
    LoginPerMinute       int
    BrowserAPIPerMinute  int
    ExternalAPIPerMinute int
    Resolver             RateLimitResolver
}
```

`Resolver` が `nil` の場合は、P7 と同じ config 固定挙動にします。これにより、unit test、OpenAPI export、部分的な runtime wiring が壊れにくくなります。

### 3-3. hash helper を整理する

現在の `hashRateLimitKey` を、bucket key builder として使える形にします。

```go
func RateLimitBucketKey(scope string, values ...string) string {
    joined := scope + "\x00" + strings.Join(values, "\x00")
    sum := sha256.Sum256([]byte(joined))
    return scope + ":" + hex.EncodeToString(sum[:])
}
```

既存の IP fallback もこの helper を使います。

```go
fallbackBucket := RateLimitBucketKey("ip", c.ClientIP())
```

この helper は middleware package の中で使うだけでもよいですが、`app.go` の resolver closure から使うなら export します。

### 3-4. request ごとに decision を解決する

middleware の流れを次にします。

```go
policy, defaultLimit := rateLimitPolicy(c, cfg)
if !cfg.Enabled || client == nil || policy == "" || defaultLimit <= 0 {
    c.Next()
    return
}

decision := RateLimitDecision{
    Policy:         policy,
    LimitPerMinute: defaultLimit,
    BucketKey:      RateLimitBucketKey("ip", c.ClientIP()),
}

if cfg.Resolver != nil {
    resolved, err := cfg.Resolver(c.Request.Context(), c, policy, defaultLimit)
    if err == nil && resolved.LimitPerMinute > 0 && resolved.BucketKey != "" {
        decision = resolved
    }
}
```

resolver error は request を止めません。P11 の方針通り、config default の IP bucket で処理を続けます。

### 3-5. Redis key は decision から作る

Redis key は policy と bucket key と minute window から作ります。

```go
window := time.Now().UTC().Format("200601021504")
key := "rate_limit:" + decision.Policy + ":" + decision.BucketKey + ":" + window
```

既存実装が `EXPIRE 1 minute` だけで key に window を含めない場合でも動きますが、manual investigation では window が key に入っている方が分かりやすいです。

P11 で key format を変える場合は、old key との互換移行は不要です。Redis rate limit key は短命で、永続 data ではありません。

### 3-6. `Retry-After` は固定 60 秒でよい

P11 では fixed window を維持するため、超過時の response は既存と同じで十分です。

```go
c.Header("Retry-After", "60")
writeProblem(c, http.StatusTooManyRequests, "rate limit exceeded")
```

厳密に残り秒数を返す改善は後続でよいです。

## Step 4. app wiring で runtime resolver を接続する

### 4-1. app で resolver closure を作る

#### ファイル: `backend/internal/app/app.go`

`middleware.RateLimit` を呼ぶ前に、browser API 用 resolver closure を作ります。

方針は次です。

- `policy != "browser_api"` の場合は config default の bucket を返す
- session cookie がない場合は config default の IP bucket を返す
- session 解決に失敗した場合は config default の IP bucket を返す
- active tenant がない場合は config default の user bucket または IP bucket を返す
- active tenant がある場合だけ tenant settings resolver を呼ぶ

例:

```go
rateLimitDefaults := service.RateLimitDefaults{
    LoginPerMinute:       cfg.RateLimitLoginPerMinute,
    BrowserAPIPerMinute:  cfg.RateLimitBrowserAPIPerMinute,
    ExternalAPIPerMinute: cfg.RateLimitExternalAPIPerMinute,
}

rateLimitResolver := func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (middleware.RateLimitDecision, error) {
    if policy != "browser_api" || sessionService == nil || tenantSettingsService == nil {
        return middleware.RateLimitDecision{
            Policy:         policy,
            LimitPerMinute: defaultLimit,
            BucketKey:      middleware.RateLimitBucketKey("ip", c.ClientIP()),
        }, nil
    }

    sessionCookie, err := c.Request.Cookie(auth.SessionCookieName)
    if err != nil || strings.TrimSpace(sessionCookie.Value) == "" {
        return middleware.RateLimitDecision{
            Policy:         policy,
            LimitPerMinute: defaultLimit,
            BucketKey:      middleware.RateLimitBucketKey("ip", c.ClientIP()),
        }, nil
    }

    current, err := sessionService.CurrentSession(ctx, sessionCookie.Value)
    if err != nil || current.ActiveTenantID == nil {
        return middleware.RateLimitDecision{
            Policy:         policy,
            LimitPerMinute: defaultLimit,
            BucketKey:      middleware.RateLimitBucketKey("ip", c.ClientIP()),
        }, nil
    }

    requester := current.User
    if current.ActorUser != nil {
        requester = *current.ActorUser
    }

    limit, err := tenantSettingsService.ResolveEffectiveRateLimit(ctx, *current.ActiveTenantID, policy, rateLimitDefaults)
    if err != nil {
        limit = defaultLimit
    }

    return middleware.RateLimitDecision{
        Policy:         policy,
        LimitPerMinute: limit,
        BucketKey: middleware.RateLimitBucketKey(
            "tenant_user",
            strconv.FormatInt(*current.ActiveTenantID, 10),
            strconv.FormatInt(requester.ID, 10),
        ),
    }, nil
}
```

実際の実装では import の追加が必要です。

- `strings`
- `strconv`
- `example.com/haohao/backend/internal/auth`

### 4-2. middleware config に resolver を渡す

`middleware.RateLimit` の config に `Resolver` を渡します。

```go
middleware.RateLimit(redisClient, middleware.RateLimitConfig{
    Enabled:              cfg.RateLimitEnabled,
    LoginPerMinute:       cfg.RateLimitLoginPerMinute,
    BrowserAPIPerMinute:  cfg.RateLimitBrowserAPIPerMinute,
    ExternalAPIPerMinute: cfg.RateLimitExternalAPIPerMinute,
    Resolver:             rateLimitResolver,
}, metrics)
```

`RateLimit` middleware は Huma route handler より前に動きます。そのため、ここで `SessionService.CurrentSession` を呼ぶと、後続 handler でもう一度 session を読む endpoint があります。P11 では正しさを優先し、この重複は許容します。

もし後で性能が問題になったら、request context に `CurrentSession` を保存して後続 handler が再利用する、または tenant settings resolver に短い TTL cache を足します。P11 初期版では cache を入れません。

### 4-3. OpenAPI export path を壊さない

`backend/internal/app/openapi.go` では DB / Redis / session が nil に近い構成で OpenAPI を export します。

resolver が `nil` service を許容し、`Resolver` 自体も config default に戻る実装なら、OpenAPI export は壊れません。

確認コマンド:

```bash
make openapi
```

P11 では OpenAPI の中身に意味のある差分が出ないことが期待値です。

## Step 5. browser API bucket key を `tenant + requester` にする

### 5-1. bucket の比較対象を固定する

P11 で同じ bucket になる request は次です。

```text
same policy
same active tenant
same requester user
same minute window
```

逆に、次は別 bucket になります。

- 同じ user だが active tenant が違う
- 同じ tenant だが user が違う
- support access の actor user が違う
- minute window が違う

### 5-2. active tenant がない browser API

`/api/v1/session` や `/api/v1/tenants` のように、active tenant がなくても呼ばれる browser API があります。

これらは tenant settings override を解決できないため、config default を使います。bucket は IP fallback で十分です。

```text
rate_limit:browser_api:ip:{hash}:{window}
```

この fallback は「active tenant がない request では tenant override を使わない」という明確な仕様です。

### 5-3. route の tenant slug は使わない

Tenant Admin API には `/api/v1/admin/tenants/{tenantSlug}/...` のように path 上の tenant slug があります。

P11 の resolver では path parameter の tenant slug を rate limit tenant として使いません。理由は、middleware の時点では Huma route parameter の抽出に依存しない方が安定するためです。

rate limit の tenant は session の active tenant だけを使います。

この仕様により、tenant admin が active tenant と別 tenant の admin detail を開く場合でも、rate limit bucket は active tenant に紐づきます。P11 ではこの単純さを優先します。将来、admin route の target tenant で制限したい場合は別 policy として設計します。

## Step 6. fallback と failure mode を固定する

### 6-1. settings lookup timeout

settings lookup は request path 上で実行されます。DB が遅い時に rate limit resolver が API 全体を長く止めないように、短い timeout をかけます。

```go
lookupCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
defer cancel()

limit, err := tenantSettingsService.ResolveEffectiveRateLimit(lookupCtx, *current.ActiveTenantID, policy, rateLimitDefaults)
```

timeout した場合は config default に fallback します。

### 6-2. session lookup failure

session lookup failure は rate limit middleware では auth failure として扱いません。

```text
rate limit middleware: default IP bucket で続行
auth / handler: 後段で 401 / 403 を返す
```

rate limit middleware が auth middleware の代わりになると、失敗時の response が分かりにくくなります。P11 では rate limit の責務だけに留めます。

### 6-3. settings lookup failure

settings lookup failure は request を止めず、default limit で判定します。

```go
if err != nil {
    limit = defaultLimit
}
```

この fallback により、tenant settings DB lookup の一時障害で全 browser API が `500` になることを避けます。

### 6-4. Redis failure

Redis `INCR` が失敗した場合は既存通り fail-open にします。

```go
count, err := client.Incr(ctx, key).Result()
if err != nil {
    if metrics != nil {
        metrics.IncRateLimit(policy, "error")
    }
    c.Next()
    return
}
```

このとき `429` は返しません。

### 6-5. metrics の扱い

正常な判定では既存通りです。

```go
metrics.IncRateLimit(policy, "allowed")
metrics.IncRateLimit(policy, "blocked")
```

settings lookup fallback は、最終的に default で判定できているため、`allowed` / `blocked` のどちらかだけを increment します。追加で tenant id 付き metrics を作らないでください。

## Step 7. unit test と smoke を追加する

### 7-1. middleware unit test

#### ファイル: `backend/internal/middleware/rate_limit_test.go`

`miniredis` と `httptest` を使い、middleware 単体を確認します。

最低限の test case は次です。

```text
default config blocks after limit
resolver override blocks after override limit
different bucket keys do not share counters
resolver error falls back to default
redis error fails open
metrics result does not include tenant or user labels
```

構成例:

```go
server := miniredis.RunT(t)
client := redis.NewClient(&redis.Options{Addr: server.Addr()})

router := gin.New()
router.Use(RateLimit(client, RateLimitConfig{
    Enabled:             true,
    BrowserAPIPerMinute: 2,
    Resolver: func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error) {
        return RateLimitDecision{
            Policy:         policy,
            LimitPerMinute: 1,
            BucketKey:      RateLimitBucketKey("tenant_user", "tenant-1", "user-1"),
        }, nil
    },
}, metrics))
router.GET("/api/v1/customer-signals", func(c *gin.Context) {
    c.Status(http.StatusNoContent)
})
```

1 回目は `204`、2 回目は `429` になることを確認します。

### 7-2. service unit test

#### ファイル: `backend/internal/service/tenant_settings_service_test.go`

既存 service test の DB helper 方針に合わせて、`ResolveEffectiveRateLimit` を確認します。

最低限の test case は次です。

- tenant settings row がない場合は default
- `rate_limit_browser_api_per_minute` がある場合は override
- `rate_limit_browser_api_per_minute` が null の場合は default
- `login` / `external_api` は method としては解決できる
- unknown policy は browser default

DB を使う test helper が重い場合は、`defaultRateLimitForPolicy` と `tenantSettingsFromDB` 周辺の純粋関数を中心にし、integration 寄りの確認は smoke に寄せます。

### 7-3. app wiring test

#### ファイル: `backend/internal/app/metrics_test.go` または新規 `backend/internal/app/rate_limit_test.go`

app level では、すべてを DB 付きで組むより、middleware resolver の nil / fallback が OpenAPI export と health route を壊さないことを確認します。

確認すること:

- `/metrics` は rate limit 対象外のまま
- `/healthz` / `/readyz` は rate limit 対象外のまま
- OpenAPI export test が通る

browser session 付きの tenant override は smoke の方が分かりやすいため、ここでは詰め込みすぎません。

### 7-4. smoke script を追加する

#### ファイル: `scripts/smoke-rate-limit-runtime.sh`

single binary または local backend に対して、tenant settings override が runtime に反映されることを確認する smoke を追加します。

流れ:

1. `demo@example.com` で login
2. CSRF token を取得
3. unique tenant slug を作る
4. demo user に `customer_signal_user` を grant
5. active tenant を unique tenant に切り替える
6. tenant settings を `rateLimitBrowserApiPerMinute: 2` に更新する
7. 同じ browser session で `/api/v1/customer-signals` を連続で呼ぶ
8. どこかで `429` が返り、`Retry-After` header があることを確認する
9. 別 tenant または override null では config default に戻ることを確認する

tenant 作成と role grant は既存 admin API を使います。

```bash
tenant_slug="p11-rl-$(date +%s)-$$"

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"slug\":\"$tenant_slug\",\"displayName\":\"P11 Rate Limit\"}" \
  "$BASE_URL/api/v1/admin/tenants" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"userEmail":"demo@example.com","roleCode":"customer_signal_user"}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/memberships" >/dev/null
```

active tenant 切り替え:

```bash
curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"tenantSlug\":\"$tenant_slug\"}" \
  "$BASE_URL/api/v1/session/tenant" >/dev/null
```

settings 更新:

```bash
curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -X PUT \
  -d '{"fileQuotaBytes":104857600,"rateLimitBrowserApiPerMinute":2,"notificationsEnabled":true,"features":{}}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/settings" \
  | rg '"rateLimitBrowserApiPerMinute":2' >/dev/null
```

rate limit 確認:

```bash
blocked=0
for _ in 1 2 3 4; do
  headers="$(mktemp)"
  status="$(curl -sS -o /dev/null -D "$headers" -w '%{http_code}' -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/v1/customer-signals")"
  if [[ "$status" == "429" ]]; then
    rg -i '^Retry-After:' "$headers" >/dev/null
    blocked=1
    rm -f "$headers"
    break
  fi
  rm -f "$headers"
done

if [[ "$blocked" != "1" ]]; then
  echo "expected browser API rate limit to block after tenant override" >&2
  exit 1
fi
```

### 7-5. Makefile target を追加する

#### ファイル: `Makefile`

P7 / P10 smoke と同じ形で target を追加します。

```make
smoke-rate-limit-runtime:
	bash scripts/smoke-rate-limit-runtime.sh
```

CI に入れるかどうかは、`make e2e` と同じく Redis / Postgres / binary 起動コストを見て判断します。まずは local smoke target として追加すれば十分です。

## Step 8. 生成と確認

### 8-1. 生成物を確認する

P11 は public schema を変えないため、`make gen` の主目的は drift がないことの確認です。

```bash
make gen
```

期待値:

- `openapi/openapi.yaml` に route / schema の意図しない差分がない
- `openapi/browser.yaml` に route / schema の意図しない差分がない
- `openapi/external.yaml` に差分がない
- `frontend/src/api/generated` に意図しない差分がない
- `backend/internal/db` に意図しない差分がない

差分が出た場合は、P11 の実装中に Huma request / response type や SQL query を変えていないか確認します。

### 8-2. backend test

```bash
go test ./backend/internal/middleware ./backend/internal/service ./backend/internal/app
go test ./backend/...
```

失敗時の見方:

- `rate_limit_test.go` で 429 が出ない場合は resolver が返した `BucketKey` と `LimitPerMinute` が middleware で使われているか確認する
- resolver fallback test が落ちる場合は resolver error 時に request を止めていないか確認する
- OpenAPI test が落ちる場合は app wiring が nil service / nil Redis を許容しているか確認する
- metrics test が落ちる場合は `policy` / `result` 以外の label を増やしていないか確認する

### 8-3. frontend build

P11 では UI 変更はありませんが、既存 Tenant Admin UI が壊れていないことを確認します。

```bash
npm --prefix frontend run build
```

失敗する場合は、P11 で generated SDK に不要な差分を出していないか確認します。

### 8-4. single binary

```bash
make binary
```

P11 は runtime middleware の変更なので、single binary で確認する価値があります。

### 8-5. common services smoke

```bash
make smoke-common-services
```

既存 P7 smoke が落ちる場合は、settings API と rate limit middleware の fallback を疑います。

特に、`smoke-common-services` は tenant settings を更新します。ここで `429` が早すぎる場合は、smoke 用の request が同じ bucket に入り続けていないか、test 実行前の Redis key が残っていないか確認します。

### 8-6. P11 smoke

```bash
make smoke-rate-limit-runtime
```

期待値:

- tenant settings update response に `"rateLimitBrowserApiPerMinute":2` が出る
- 同一 session / active tenant の browser API 連続 request が `429` になる
- `429` response に `Retry-After` がある
- `/metrics` に `haohao_rate_limit_total{policy="browser_api",result="blocked"}` が出る

失敗時の見方:

- `429` が出ない場合は `tenantSettingsService.ResolveEffectiveRateLimit` が呼ばれているか確認する
- `429` が出ない場合は bucket が request ごとに変わっていないか確認する
- 最初の request から `429` になる場合は Redis に古い key が残っているか、smoke tenant slug が unique か確認する
- settings update が `403` の場合は demo user に `tenant_admin` global role があるか確認する
- Customer Signals list が `403` の場合は unique tenant で `customer_signal_user` tenant role が grant されているか確認する
- `Retry-After` がない場合は blocked branch の response header を確認する

### 8-7. E2E

```bash
make e2e
```

E2E では rate limit を基本的に無効化している場合があります。その場合、P11 の runtime override は `make smoke-rate-limit-runtime` で確認し、E2E では既存 browser journey が rate limit 変更で壊れていないことを確認します。

E2E 実行 script で `RATE_LIMIT_ENABLED=false` を明示している場合、それは正常です。P11 smoke は rate limit enabled の runtime に対して実行します。

## 手動確認

local server を起動して手動で見る場合:

```bash
make up
make db-up
make seed-demo-user
RATE_LIMIT_ENABLED=true RATE_LIMIT_BROWSER_API_PER_MINUTE=120 make backend-dev
```

別 terminal で:

```bash
BASE_URL=http://127.0.0.1:8080 make smoke-rate-limit-runtime
```

UI で確認する場合:

1. `demo@example.com` / `changeme123` で login
2. Tenant Admin で対象 tenant を開く
3. `Browser API limit / minute` に `2` を入れて保存
4. 同じ tenant を active tenant にする
5. Customer Signals 画面で refresh や検索を短時間に繰り返す
6. Network tab で `429` と `Retry-After` を確認する

UI では連打タイミングに左右されるため、最終確認は smoke script で行います。

## トラブルシュート

### settings API では保存できるが runtime に効かない

見る場所:

```bash
rg -n "Resolver|ResolveEffectiveRateLimit|RateLimitBucketKey" backend/internal
```

確認すること:

- `app.go` で `RateLimitConfig.Resolver` に closure を渡している
- closure が `policy == "browser_api"` で settings resolver を呼んでいる
- `tenantSettingsService` が nil になっていない
- `current.ActiveTenantID` が nil になっていない
- `rateLimitBrowserApiPerMinute` が `nil` ではなく保存されている

### すぐ `429` になる

可能性:

- Redis key が tenant / requester で分離されていない
- smoke が同じ tenant slug を使い回している
- test 前の Redis key が TTL 中に残っている
- `LimitPerMinute` が 1 など想定より低い

確認:

```bash
redis-cli --scan --pattern 'rate_limit:browser_api:*'
```

Redis key に raw tenant id や raw user id が出ている場合は修正します。

### `429` が出ない

可能性:

- resolver error で常に config default へ fallback している
- `RATE_LIMIT_ENABLED=false` で起動している
- Redis client が nil で middleware が bypass している
- request ごとに `BucketKey` が変わっている
- active tenant が切り替わっていない

確認:

```bash
curl -sS "$BASE_URL/metrics" | rg 'haohao_rate_limit_total'
```

`browser_api` の `allowed` も出ていない場合は、middleware が対象 path に適用されていない可能性があります。

### E2E だけ落ちる

E2E は短時間に多くの browser API を呼ぶため、rate limit enabled のままだと不安定になります。

`scripts/e2e-single-binary.sh` で `RATE_LIMIT_ENABLED=false` を維持してください。P11 の rate limit runtime 連動は `make smoke-rate-limit-runtime` で確認します。

## 最終確認チェックリスト

P11 実装後は次を確認します。

- `TenantSettingsService.ResolveEffectiveRateLimit` がある
- `RateLimitConfig` が resolver を受け取れる
- resolver が nil のとき P7 と同じ config 固定挙動になる
- `browser_api` の active tenant あり request で tenant settings override が使われる
- active tenant なし request では config default が使われる
- support access 中は actor user で bucket が作られる
- Redis key に raw tenant id、raw user id、email、session id が入らない
- metrics label が `policy` / `result` のまま
- settings lookup failure で request が `500` にならない
- Redis failure で request が fail-open する
- `make smoke-rate-limit-runtime` で `429` と `Retry-After` を確認できる
- `make e2e` が既存 browser journey を通す

この P11 が終わると、Tenant Admin で設定した browser API rate limit override が、表示だけでなく実際の runtime 制御として効く状態になります。
