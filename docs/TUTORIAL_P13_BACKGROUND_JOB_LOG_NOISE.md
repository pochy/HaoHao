# P13: Background Job Log Noise 改善チュートリアル

## この文書の目的

この文書は、短時間に大量出力される background job の成功ログを、運用で使える粒度に改善するための実装チュートリアルです。

対象になる代表的なログは次です。

```json
{"time":"2026-04-26T20:14:55.357281+09:00","level":"INFO","msg":"data lifecycle job completed","trigger":"interval"}
{"time":"2026-04-26T20:14:56.151519+09:00","level":"INFO","msg":"outbox worker completed","trigger":"interval","duration_ms":5.441}
```

このログは、backend の周期ジョブが「1 回の実行に成功した」ことを示します。ただし、成功した実行が何も処理していない場合でも `INFO` として出続けるため、短い interval ではログが埋まります。

このチュートリアルでは、単にログを削除するのではなく、次の状態へ改善します。

- 失敗は必ず `ERROR` で残す
- 実際に処理した run は `INFO` で件数付きで残す
- startup run は起動確認として `INFO` で残してよい
- 何も処理していない interval run は `DEBUG` に下げる
- 高頻度の生存確認はログではなく metrics で見る

## 今回のログへの回答

### `data lifecycle job completed` は何のためのログか

`DataLifecycleJob` が定期的なデータ整理を完了したことを示すログです。

この job は主に次を扱います。

- expired idempotency key の削除
- expired tenant invitation の失効
- processed outbox event の retention 削除
- read notification の retention 削除
- expired tenant data export の soft delete
- soft deleted file body の物理削除

したがって job 自体は必要です。ファイル共有、通知、outbox、データ export を長く運用するほど重要になります。

### `outbox worker completed` は何のためのログか

`OutboxWorker` が outbox event の claim と delivery を 1 batch 分完了したことを示すログです。

outbox は、DB transaction と副作用を分離するための仕組みです。メール送信、監査連携、import/export、通知、外部 provider 連携などを、DB に event として保存してから worker が処理します。

worker 自体は必要です。API request 内で外部副作用を直接完了させないための運用上の安全弁です。

### この `INFO` ログは必要か

現在の粒度では不要です。

`ERROR` は必要です。処理対象があった run の `INFO` も必要です。しかし、何も処理していない interval run を毎回 `INFO` に出す必要はありません。

理由は次です。

- 成功・失敗・処理件数はすでに Prometheus metrics で観測できる
- no-op の成功ログは障害調査で有益な情報が少ない
- smoke / E2E では interval を `200ms` に短縮するため、短時間で大量出力される
- ログが埋まると、本当に必要な `ERROR` や user-facing request log を見落としやすい

このため、実装方針は「job を止める」ではなく「no-op interval の成功ログを `DEBUG` に落とす」です。

## この文書が前提にしている現在地

このチュートリアルは、次の文書と実装がある前提で進めます。

- `docs/TUTORIAL.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`
- `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md`
- `docs/TUTORIAL_P4_OBSERVABILITY.md`
- `docs/TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md`
- `backend/internal/jobs/data_lifecycle.go`
- `backend/internal/jobs/outbox_worker.go`
- `backend/internal/platform/metrics.go`
- `scripts/smoke-file-purge.sh`
- `scripts/e2e-single-binary.sh`

この文書では public API、OpenAPI schema、frontend UI は変更しません。backend の background job logging と test を中心に改善します。

## 完成条件

このチュートリアルの完了条件は次です。

- `DATA_LIFECYCLE_INTERVAL=200ms` でも no-op の `data lifecycle job completed` が `INFO` に出続けない
- `OUTBOX_WORKER_INTERVAL=200ms` でも no-op の `outbox worker completed` が `INFO` に出続けない
- data lifecycle が実際に削除、失効、purge を行った run は `INFO` に件数付きで出る
- outbox worker が event を sent / failed / dead にした run は `INFO` に件数付きで出る
- startup run は起動確認として `INFO` に残せる
- failure は従来通り `ERROR` に残る
- skipped run は従来通り `WARN` に残る
- Prometheus metrics の名前と label は変えない
- `go test ./backend/internal/jobs` と `go test ./backend/...` が通る
- `make smoke-file-purge` と `make e2e` のログ量が実用的になる

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | logging policy | 残すログ、下げるログ、metrics へ任せる情報を決める |
| Step 2 | `backend/internal/jobs/data_lifecycle.go` | data lifecycle の処理件数 summary を作る |
| Step 3 | `backend/internal/jobs/outbox_worker.go` | outbox worker の処理件数 summary を作る |
| Step 4 | `backend/internal/jobs/*_test.go` | no-op が `INFO` に出ないことを test する |
| Step 5 | smoke / E2E | 200ms interval のままログ量を確認する |
| Step 6 | runbook | metrics と log の見方を運用文書へ反映する |

## Step 1. logging policy を決める

### 方針

background job のログは、次の policy に揃えます。

| 状態 | level | 理由 |
| --- | --- | --- |
| startup success | `INFO` | 起動時に worker が有効化されたことを確認できる |
| interval success with work | `INFO` | 何を処理したかを調査できる |
| interval success without work | `DEBUG` | 通常運用では noise のため |
| failure | `ERROR` | 障害調査に必須 |
| previous run still active | `WARN` | interval と timeout の設計不備を検知したい |

### やらないこと

- job を無効化しない
- interval を単に長くするだけで隠さない
- `ERROR` を `DEBUG` に下げない
- metrics を削らない
- file path、storage key、share link token、credential をログに出さない

smoke / E2E の `200ms` interval は、処理を速く完了させるための設定です。ここは変えず、ログの意味を改善します。

## Step 2. DataLifecycleJob に summary を追加する

### 対象ファイル

```text
backend/internal/jobs/data_lifecycle.go
```

### 現在の問題

`runOnce` は `RunOnce` の error だけを見て、成功したら必ず `INFO` を出しています。

```go
err := j.RunOnce(ctx)
if err != nil {
	j.logger.ErrorContext(ctx, "data lifecycle job failed", "trigger", trigger, "error", err.Error())
	return
}
j.logger.InfoContext(ctx, "data lifecycle job completed", "trigger", trigger)
```

このため、削除対象や purge 対象が 0 件でも `INFO` が出ます。

### 実装する summary

private type として処理件数をまとめます。

```go
type dataLifecycleRunSummary struct {
	ExpiredIDKeysDeleted       int64
	TenantInvitationsExpired   int64
	ProcessedOutboxDeleted     int64
	ReadNotificationsDeleted   int64
	TenantDataExportsExpired   int64
	FileBodiesClaimed          int64
	FileBodiesPurged           int64
	FileBodyPurgeFailed        int64
}

func (s dataLifecycleRunSummary) changed() bool {
	return s.ExpiredIDKeysDeleted > 0 ||
		s.TenantInvitationsExpired > 0 ||
		s.ProcessedOutboxDeleted > 0 ||
		s.ReadNotificationsDeleted > 0 ||
		s.TenantDataExportsExpired > 0 ||
		s.FileBodiesClaimed > 0 ||
		s.FileBodiesPurged > 0 ||
		s.FileBodyPurgeFailed > 0
}

func (s dataLifecycleRunSummary) attrs() []any {
	return []any{
		"expired_idempotency_keys_deleted", s.ExpiredIDKeysDeleted,
		"tenant_invitations_expired", s.TenantInvitationsExpired,
		"processed_outbox_events_deleted", s.ProcessedOutboxDeleted,
		"read_notifications_deleted", s.ReadNotificationsDeleted,
		"tenant_data_exports_expired", s.TenantDataExportsExpired,
		"file_bodies_claimed", s.FileBodiesClaimed,
		"file_bodies_purged", s.FileBodiesPurged,
		"file_body_purge_failed", s.FileBodyPurgeFailed,
	}
}
```

`RunOnce(ctx) error` は既存 test や呼び出し側を壊さないため残します。内部用に summary を返す関数を追加します。

```go
func (j *DataLifecycleJob) RunOnce(ctx context.Context) error {
	_, err := j.runOnceWithSummary(ctx)
	return err
}

func (j *DataLifecycleJob) runOnceWithSummary(ctx context.Context) (dataLifecycleRunSummary, error) {
	// 既存 RunOnce の処理をここへ移す
}
```

### logging を変更する

`runOnce` では summary を見て level を切り替えます。

```go
summary, err := j.runOnceWithSummary(ctx)
if j.metrics != nil {
	j.metrics.IncDataLifecycleRun(trigger, err)
}
if err != nil {
	j.logger.ErrorContext(ctx, "data lifecycle job failed", "trigger", trigger, "error", err.Error())
	return
}

attrs := append([]any{"trigger", trigger}, summary.attrs()...)
if trigger == "startup" || summary.changed() {
	j.logger.InfoContext(ctx, "data lifecycle job completed", attrs...)
	return
}
j.logger.DebugContext(ctx, "data lifecycle job completed", "trigger", trigger)
```

注意点:

- `j.addItems(...)` は従来通り呼ぶ
- metrics の kind 名は変えない
- file purge の詳細 `DebugContext` は残してよい
- `FileBodyPurgeFailed` が 1 件以上なら `INFO` に上げる

## Step 3. OutboxWorker に summary を追加する

### 対象ファイル

```text
backend/internal/jobs/outbox_worker.go
```

### 現在の問題

`runBatch` は error だけを返します。そのため、event を 0 件 claim した run と、event を処理した run を `runOnce` が区別できません。

### 実装する summary

private type として処理件数をまとめます。

```go
type outboxRunSummary struct {
	Claimed int
	Sent    int
	Failed  int
	Dead    int
}

func (s outboxRunSummary) changed() bool {
	return s.Claimed > 0 || s.Sent > 0 || s.Failed > 0 || s.Dead > 0
}

func (s outboxRunSummary) attrs() []any {
	return []any{
		"claimed", s.Claimed,
		"sent", s.Sent,
		"failed", s.Failed,
		"dead", s.Dead,
	}
}
```

`runBatch` の signature を変更します。

```go
func (w *OutboxWorker) runBatch(ctx context.Context) (outboxRunSummary, error) {
	var summary outboxRunSummary

	events, err := w.outbox.Claim(ctx, w.config.WorkerID, w.config.BatchSize)
	if err != nil {
		return summary, fmt.Errorf("claim outbox events: %w", err)
	}
	summary.Claimed = len(events)

	for _, event := range events {
		handleErr := w.handler.HandleOutboxEvent(ctx, event)
		if handleErr == nil {
			if err := w.outbox.MarkSent(ctx, event); err != nil {
				return summary, fmt.Errorf("mark outbox event sent: %w", err)
			}
			summary.Sent++
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "sent")
			}
			continue
		}

		if errors.Is(handleErr, service.ErrUnknownOutboxEvent) {
			if err := w.outbox.MarkFailed(ctx, eventWithMaxAttempts(event), handleErr); err != nil {
				return summary, fmt.Errorf("mark unknown outbox event dead: %w", err)
			}
			summary.Dead++
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "dead")
			}
			continue
		}

		if err := w.outbox.MarkFailed(ctx, event, handleErr); err != nil {
			return summary, fmt.Errorf("mark outbox event failed: %w", err)
		}
		summary.Failed++
		if w.metrics != nil {
			w.metrics.IncOutboxEvent(event.EventType, "failed")
		}
	}

	return summary, nil
}
```

### logging を変更する

`runOnce` では duration と summary を合わせて出します。

```go
summary, err := w.runBatch(ctx)
duration := time.Since(startedAt)
if w.metrics != nil {
	w.metrics.ObserveOutboxRun(trigger, duration, err)
}
if err != nil {
	w.logger.ErrorContext(ctx, "outbox worker failed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000, "error", err.Error())
	return
}

attrs := append([]any{
	"trigger", trigger,
	"duration_ms", float64(duration.Microseconds()) / 1000,
}, summary.attrs()...)

if trigger == "startup" || summary.changed() {
	w.logger.InfoContext(ctx, "outbox worker completed", attrs...)
	return
}
w.logger.DebugContext(ctx, "outbox worker completed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000)
```

注意点:

- `Claimed=0` の interval run は `DEBUG`
- `Sent=0` でも `Failed` または `Dead` があれば `INFO`
- `ERROR` log は従来通り duration と error を含める
- event payload、credential、share link token は出さない

## Step 4. job logging test を追加する

### 対象ファイル

```text
backend/internal/jobs/data_lifecycle_test.go
backend/internal/jobs/outbox_worker_test.go
```

既存 test がある場合は、そのファイルへ追加します。`outbox_worker_test.go` が無ければ新規作成します。

### test logger helper

`INFO` 以上だけを buffer に出す logger を用意します。

```go
func newInfoBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
```

no-op run では buffer に completion log が出ないことを確認します。

```go
func TestDataLifecycleJobRunOnceNoopIntervalDoesNotInfoLog(t *testing.T) {
	var logs bytes.Buffer
	job := newDataLifecycleJobForTest(newInfoBufferLogger(&logs))

	job.runOnce(context.Background(), "interval")

	if strings.Contains(logs.String(), "data lifecycle job completed") {
		t.Fatalf("expected no info completion log for noop interval, got %s", logs.String())
	}
}
```

startup run は `INFO` に残してよいので、別 test で確認します。

```go
func TestDataLifecycleJobRunOnceStartupInfoLogs(t *testing.T) {
	var logs bytes.Buffer
	job := newDataLifecycleJobForTest(newInfoBufferLogger(&logs))

	job.runOnce(context.Background(), "startup")

	if !strings.Contains(logs.String(), "data lifecycle job completed") {
		t.Fatalf("expected startup completion log, got %s", logs.String())
	}
}
```

処理件数がある run は `INFO` に残ることを確認します。

```go
func TestDataLifecycleJobRunOnceWithWorkInfoLogsCounts(t *testing.T) {
	var logs bytes.Buffer
	job := newDataLifecycleJobForTest(newInfoBufferLogger(&logs))
	job.queries.(*fakeDataLifecycleQueries).expiredIDKeys = 3

	job.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, "data lifecycle job completed") ||
		!strings.Contains(got, `"expired_idempotency_keys_deleted":3`) {
		t.Fatalf("expected info completion log with counts, got %s", got)
	}
}
```

outbox worker でも同じ観点で test します。

- empty claim の interval run は `INFO` に出ない
- event を sent にした run は `INFO` に `claimed` と `sent` が出る
- handler error で failed/dead になった run は `INFO` に出る
- claim error は `ERROR` に出る

test では log の完全一致ではなく、message と主要 field を部分一致で確認します。JSON field order に依存しない実装にします。

## Step 5. smoke / E2E の interval は維持する

### 対象ファイル

```text
scripts/smoke-file-purge.sh
scripts/e2e-single-binary.sh
```

これらの script では、処理を短時間で終えるために interval を短くしています。

```bash
OUTBOX_WORKER_INTERVAL=200ms
DATA_LIFECYCLE_INTERVAL=200ms
```

この値は変えません。ログが多いから interval を伸ばすと、file purge smoke や E2E の完了待ちが遅くなります。

改善後は、同じ `200ms` でも no-op run が `INFO` に出ないことを確認します。

```bash
make smoke-file-purge
make e2e
```

SeaweedFS storage driver も使っている場合は、次も確認します。

```bash
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY=haohao \
FILE_S3_SECRET_KEY=haohao-dev-secret \
FILE_S3_FORCE_PATH_STYLE=true \
make smoke-file-purge
```

## Step 6. runbook に観測方法を反映する

### 対象ファイル

```text
docs/RUNBOOK_OBSERVABILITY.md
docs/RUNBOOK_OPERABILITY.md
```

必要に応じて、background job の正常性確認を log ではなく metrics 中心にすることを追記します。

確認する metrics:

```text
haohao_data_lifecycle_runs_total
haohao_data_lifecycle_items_total
haohao_outbox_runs_total
haohao_outbox_duration_seconds
haohao_outbox_events_total
```

運用時の見方:

- `*_runs_total{status="error"}` が増えていないか
- `haohao_outbox_duration_seconds` が interval / timeout に近づいていないか
- `haohao_outbox_events_total{status="failed"}` または `status="dead"` が増えていないか
- file purge の `file_objects_body_purged` が expected smoke で増えているか
- `WARN` の skipped run が頻発していないか

## 最終確認コマンド

実装後は次の順で確認します。

```bash
go test ./backend/internal/jobs
go test ./backend/...
npm --prefix frontend run build
make binary
make smoke-file-purge
make e2e
git diff --check
```

SeaweedFS を使う構成も確認する場合:

```bash
make seaweedfs-up
make seaweedfs-config

FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY=haohao \
FILE_S3_SECRET_KEY=haohao-dev-secret \
FILE_S3_FORCE_PATH_STYLE=true \
make smoke-file-purge
```

## 生成物と手書きファイルの境界

このチュートリアルでは、次は手で編集してよい source です。

```text
backend/internal/jobs/data_lifecycle.go
backend/internal/jobs/data_lifecycle_test.go
backend/internal/jobs/outbox_worker.go
backend/internal/jobs/outbox_worker_test.go
docs/RUNBOOK_OBSERVABILITY.md
docs/RUNBOOK_OPERABILITY.md
```

次は生成物または build artifact なので、今回の修正で直接編集しません。

```text
db/schema.sql
backend/internal/db/*
openapi/openapi.yaml
frontend/src/api/generated/*
backend/web/dist/*
bin/haohao
```

## 実装時の注意

- no-op interval を `DEBUG` に落としても、metrics は必ず更新する
- startup run を `INFO` に残す場合でも、件数 field を含める
- `ERROR` log の message は既存の監視に影響しやすいため、可能なら変更しない
- `WARN` の skipped run は頻発すると worker が詰まっている可能性があるため、まずは維持する
- test では time に依存する sleep を避け、`runOnce` を直接呼ぶ
- log assertion は JSON field order に依存しない
- smoke の 200ms interval は維持し、ログ側を正す

## 完了後に期待するログ

no-op interval では、通常の `INFO` 出力に completion log が流れ続けません。

処理対象があった場合だけ、次のように件数付きで残ります。

```json
{"level":"INFO","msg":"data lifecycle job completed","trigger":"interval","expired_idempotency_keys_deleted":3,"tenant_invitations_expired":0,"processed_outbox_events_deleted":0,"read_notifications_deleted":0,"tenant_data_exports_expired":0,"file_bodies_claimed":0,"file_bodies_purged":0,"file_body_purge_failed":0}
{"level":"INFO","msg":"outbox worker completed","trigger":"interval","duration_ms":4.858,"claimed":2,"sent":2,"failed":0,"dead":0}
```

失敗時は従来通り `ERROR` で残ります。

```json
{"level":"ERROR","msg":"data lifecycle job failed","trigger":"interval","error":"delete processed outbox events: context deadline exceeded"}
{"level":"ERROR","msg":"outbox worker failed","trigger":"interval","duration_ms":5000.122,"error":"claim outbox events: context deadline exceeded"}
```

この状態であれば、短い interval を使う smoke / E2E でも必要なログだけが残り、通常運用では metrics を中心に job の健全性を確認できます。
