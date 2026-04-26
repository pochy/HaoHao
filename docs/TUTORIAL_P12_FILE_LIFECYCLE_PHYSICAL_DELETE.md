# P12 file lifecycle 物理削除チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **file lifecycle の物理削除** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P7 では tenant-aware file upload、local file storage、`file_objects` metadata、soft delete、data lifecycle job の入口を作りました。一方で、現状の data lifecycle は deleted file metadata を触るだけで、`FILE_LOCAL_DIR` 配下の実 file body は削除しません。

P12 では、この残りを閉じます。retention を過ぎた soft-deleted file について、DB 上の tombstone を残しつつ local storage 上の file body を削除し、`purged_at` で物理削除済みであることを記録します。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P7_WEB_SERVICE_COMMON.md` / `TUTORIAL_P11_TENANT_RATE_LIMIT_RUNTIME.md` と同じように、対象ファイル、主要コード方針、確認コマンド、失敗時の見方まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- P7 の `0012_web_service_common` migration が適用済み
- P10 の `0013_p10_cross_cutting_extensions` migration がある
- `file_objects` に `storage_driver`、`storage_key`、`status`、`deleted_at` がある
- `FileService.Upload` が body を local storage に保存し、metadata を `file_objects` に作成する
- `FileService.Delete` は `file_objects` を soft delete する
- `LocalFileStorage.Delete(ctx, key)` が idempotent に file body を削除できる
- `DataLifecycleJob` が `idempotency_keys`、`outbox_events`、`notifications`、`tenant_data_exports` の cleanup を実行している
- `/metrics` に `haohao_data_lifecycle_runs_total` と `haohao_data_lifecycle_items_total` が出る
- `make gen` が sqlc / OpenAPI / frontend SDK を更新できる
- `make binary` で frontend embed single binary を作れる

この P12 の初期実装は `storage_driver = 'local'` の file body を対象にします。SeaweedFS 導入後は同じ purge path が実行中 storage driver に合わせて `local` または `seaweedfs_s3` を claim します。

## 完成条件

このチュートリアルの完了条件は次です。

- `0014_file_lifecycle_physical_delete` migration が追加される
- `file_objects` に `purged_at`、`purge_attempts`、`purge_locked_at`、`purge_locked_by`、`last_purge_error` が追加される
- retention を過ぎた deleted local file を DB で claim できる
- claim は `FOR UPDATE SKIP LOCKED` と stale lock timeout に対応する
- file body deletion は DB transaction の外で実行される
- file body deletion 成功後に `file_objects.purged_at` が記録される
- file body deletion 失敗時は `last_purge_error` を残し、次回 retry できる
- job crash 後も `purge_locked_at` の timeout 後に retry できる
- `storage.Delete` 成功後に `MarkFileObjectBodyPurged` が失敗しても、次回 retry で復旧できる
- physical purge は system actor の `file.purge` audit event を残す
- audit metadata、log、metrics label に file name、storage key、tenant id、user id を出さない
- `DataLifecycleJob` が file body purge を実行し、metrics に purge 成功 / 失敗件数を出す
- `RUNBOOK_DEPLOYMENT.md` の retention 説明が更新される
- smoke で upload -> soft delete -> retention 経過 -> body deletion -> `purged_at` 記録を確認できる
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make smoke-common-services`、`make smoke-file-purge`、`make e2e` が通る

## 実装後の確認はまず smoke で見る

P12 の挙動確認は、まず自動 smoke を source of truth にします。

```bash
make up
make db-up
go test ./backend/...
make smoke-file-purge
make e2e
```

`make smoke-file-purge` は、single binary を一時 `FILE_LOCAL_DIR` で起動し、次を一続きで確認します。

- upload 後に実 file body が存在する
- DELETE API 後に download が `404` になる
- `FILE_DELETED_RETENTION=1s` 経過後に `DataLifecycleJob` が purge する
- `file_objects.purged_at` が入る
- 実 file body が消える
- `/metrics` に `kind="file_objects_body_purged"` が出る

手動確認は、smoke が通った後に「DB の状態や実 file path を自分の目で追う」ための補助手段です。手動確認だけで始めると、`DATABASE_URL` 未 export、server 未起動、`FILE_LOCAL_DIR` の取り違えで迷いやすくなります。

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | P12 境界 | 物理削除の意味と削除順序を固定する |
| Step 2 | `db/migrations/0014_file_lifecycle_physical_delete.*.sql` | purge 状態と retry 用 column を追加する |
| Step 3 | `db/queries/file_objects.sql` | purge candidate claim / mark success / mark failure query を追加する |
| Step 4 | `make db-schema`, `make sqlc` | schema と sqlc generated code を更新する |
| Step 5 | `backend/internal/service/file_service.go` | deleted file body purge を実装する |
| Step 6 | `backend/internal/jobs/data_lifecycle.go` | lifecycle job から purge service を呼ぶ |
| Step 7 | config / main wiring | batch size、lock timeout、file service 接続を追加する |
| Step 8 | audit / metrics / runbook | 運用時に追える情報を増やす |
| Step 9 | unit test / smoke | retry、idempotency、物理削除を自動確認する |
| Step 10 | 生成と確認 | build、smoke、E2E、失敗時の見方を確認する |

## 先に決める方針

### P12 の物理削除は file body の削除を指す

P12 では、`file_objects` row を hard delete しません。

削除するのは `FILE_LOCAL_DIR` 配下の実 file body です。DB metadata は tombstone として残し、`purged_at` で body が削除済みであることを記録します。

理由は次です。

- `customer_signal_imports.input_file_object_id` は `file_objects(id)` を `ON DELETE RESTRICT` で参照している
- `tenant_data_exports.file_object_id` など、file metadata を job status や audit の説明に使う場所がある
- physical purge 後も「いつ、どの file metadata が purge されたか」を運用上確認したい
- metadata hard delete は FK policy と監査保持期間を別途決めてから行うべきである

将来 metadata も削る場合は、P12 とは別の **metadata compaction** として設計します。

### DB transaction の中で filesystem delete をしない

filesystem delete は DB transaction の rollback で戻せません。そのため、P12 の順序は次に固定します。

```text
1. DB で purge candidate を claim する
2. DB transaction の外で storage.Delete(storage_key) を実行する
3. 成功したら DB に purged_at を記録する
4. 失敗したら DB に last_purge_error を記録して retry 可能にする
```

この順序では、`storage.Delete` 成功後に `MarkFileObjectBodyPurged` が失敗することがあります。その場合でも、次回 run で同じ key を再度 `Delete` します。`LocalFileStorage.Delete` は file が存在しない場合も成功扱いにするため、この retry は安全です。

### purge は retry 前提にする

file body purge は外部副作用です。local filesystem でも permission、volume mount、disk、race、deploy restart で失敗します。

P12 では terminal failed state を作りません。

- 失敗時は `last_purge_error` を残す
- `purge_locked_at` / `purge_locked_by` を解除する
- 次回 data lifecycle run で retry する
- job crash で lock が残った場合は `FILE_PURGE_LOCK_TIMEOUT` 後に retry する

一定回数で諦める dead-letter は、operator UI や alert と一緒に設計する段階で追加します。

### metrics label は増やさない

P12 後も data lifecycle metrics の label は低 cardinality に留めます。

使ってよい label:

- `trigger`
- `status`
- `kind`

使わない label:

- tenant id
- tenant slug
- user id
- file public id
- storage key
- original filename
- content type

file 単位の調査は metrics ではなく、DB の `file_objects`、audit event、structured log で行います。

## Step 1. P12 の境界を固定する

### 1-1. 現在の file lifecycle を確認する

まず、soft delete と data lifecycle の現在地を確認します。

```bash
rg -n "SoftDeleteFileObject|SoftDeleteDeletedFileObjectsBefore|DataLifecycle|LocalFileStorage|FileDeletedRetention|file_objects" backend db scripts
```

見る場所は次です。

- `db/queries/file_objects.sql`
- `backend/internal/service/file_service.go`
- `backend/internal/service/local_file_storage.go`
- `backend/internal/jobs/data_lifecycle.go`
- `backend/cmd/main/main.go`
- `.env.example`
- `RUNBOOK_DEPLOYMENT.md`

現状の `SoftDeleteDeletedFileObjectsBefore` は physical purge ではありません。`deleted_at < cutoff` の row に対して `updated_at = now()` するだけなので、P12 では置き換え対象にします。

### 1-2. やらないことを明記する

P12 では次をやりません。

- S3 / GCS driver の実装
- `file_objects` row の hard delete
- file download API の public contract 変更
- OpenAPI schema の変更
- frontend generated SDK の意味のある変更
- Tenant Admin UI への purge 操作追加
- per tenant の retention override
- purge dead-letter UI

P12 の目的は、retention 経過後に local file body が確実に消えることです。

## Step 2. purge 状態を DB に追加する

### 2-1. up migration を追加する

#### ファイル: `db/migrations/0014_file_lifecycle_physical_delete.up.sql`

```sql
ALTER TABLE file_objects
    ADD COLUMN purged_at TIMESTAMPTZ,
    ADD COLUMN purge_attempts INTEGER NOT NULL DEFAULT 0 CHECK (purge_attempts >= 0),
    ADD COLUMN purge_locked_at TIMESTAMPTZ,
    ADD COLUMN purge_locked_by TEXT,
    ADD COLUMN last_purge_error TEXT;

CREATE INDEX file_objects_purge_candidates_idx
    ON file_objects (deleted_at, id)
    WHERE deleted_at IS NOT NULL
      AND purged_at IS NULL;

CREATE INDEX file_objects_purge_lock_idx
    ON file_objects (purge_locked_at)
    WHERE purge_locked_at IS NOT NULL
      AND purged_at IS NULL;
```

`purged_at` は body deletion が成功した時刻です。`deleted_at` は user / API による logical delete、`purged_at` は storage body の physical delete と分けて扱います。

`last_purge_error` は operator 用の短い診断情報です。application log に詳細な error を出しつつ、DB には 1000 文字程度に切った値だけを保存します。

### 2-2. down migration を追加する

#### ファイル: `db/migrations/0014_file_lifecycle_physical_delete.down.sql`

```sql
DROP INDEX IF EXISTS file_objects_purge_lock_idx;
DROP INDEX IF EXISTS file_objects_purge_candidates_idx;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS last_purge_error,
    DROP COLUMN IF EXISTS purge_locked_by,
    DROP COLUMN IF EXISTS purge_locked_at,
    DROP COLUMN IF EXISTS purge_attempts,
    DROP COLUMN IF EXISTS purged_at;
```

down migration は P12 で追加した column と index だけを戻します。既存の `file_objects` schema は触りません。

## Step 3. purge 用 query を追加する

### 3-1. claim query を追加する

#### ファイル: `db/queries/file_objects.sql`

既存 query の下に、実行中 storage driver で削除できる deleted file を claim する query を追加します。

```sql
-- name: ClaimDeletedFileObjectsForPurge :many
UPDATE file_objects
SET
    purge_locked_at = now(),
    purge_locked_by = sqlc.arg(worker_id),
    purge_attempts = purge_attempts + 1,
    updated_at = now()
WHERE id IN (
    SELECT id
    FROM file_objects
    WHERE storage_driver = ANY(sqlc.arg(storage_drivers)::text[])
      AND status = 'deleted'
      AND deleted_at IS NOT NULL
      AND deleted_at < sqlc.arg(cutoff)
      AND purged_at IS NULL
      AND (
          purge_locked_at IS NULL
          OR purge_locked_at < now() - sqlc.arg(lock_timeout)
      )
    ORDER BY deleted_at, id
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

`FOR UPDATE SKIP LOCKED` は複数 process が同時に data lifecycle を動かしても同じ row を同時に claim しないために使います。

`purge_locked_at < now() - lock_timeout` は、process crash 後に残った persistent lock を回収するためです。

### 3-2. success / failure query を追加する

同じ `db/queries/file_objects.sql` に success / failure の更新 query を追加します。

```sql
-- name: MarkFileObjectBodyPurged :one
UPDATE file_objects
SET
    purged_at = now(),
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'deleted'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
RETURNING *;

-- name: MarkFileObjectPurgeFailed :one
UPDATE file_objects
SET
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = left(sqlc.arg(last_error), 1000),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND purged_at IS NULL
RETURNING *;
```

`MarkFileObjectBodyPurged` は再確認条件を持たせます。claim 後に何らかの手動操作で row が変わっていても、active file を purged にしないためです。

### 3-3. 既存 query の扱い

既存の `SoftDeleteDeletedFileObjectsBefore` は P12 後は使いません。

削除してもよいですが、まずは query と generated method を消す前に `DataLifecycleJob` 側の呼び出しを置き換え、`rg` で参照がなくなったことを確認してから消します。

```bash
rg -n "SoftDeleteDeletedFileObjectsBefore" backend db
```

参照が `db/queries/file_objects.sql` と generated code だけになったら query を削除します。

## Step 4. schema と sqlc を更新する

### 4-1. migration を適用する

```bash
make db-up
```

新しい migration が適用され、`file_objects` に purge column が追加されることを確認します。

```bash
psql "$DATABASE_URL" -c '\d file_objects'
```

### 4-2. schema を再生成する

```bash
make db-schema
```

`db/schema.sql` に次が出ていれば OK です。

- `purged_at`
- `purge_attempts`
- `purge_locked_at`
- `purge_locked_by`
- `last_purge_error`
- `file_objects_purge_candidates_idx`
- `file_objects_purge_lock_idx`

### 4-3. sqlc を再生成する

```bash
make sqlc
```

`backend/internal/db/file_objects.sql.go` に次の method が生成されます。

- `ClaimDeletedFileObjectsForPurge`
- `MarkFileObjectBodyPurged`
- `MarkFileObjectPurgeFailed`

`backend/internal/db/models.go` の `FileObject` に purge column が追加されます。

## Step 5. FileService に purge を追加する

### 5-1. purge input / result を追加する

#### ファイル: `backend/internal/service/file_service.go`

`FileDownload` 付近に purge 用の type を追加します。

```go
type FilePurgeInput struct {
    Cutoff      time.Time
    BatchSize   int32
    WorkerID    string
    LockTimeout time.Duration
}

type FilePurgeResult struct {
    Claimed int64
    Purged  int64
    Failed  int64
}
```

`BatchSize` は sqlc generated type に合わせて `int32` にします。外側の config は `int` でも構いませんが、service 境界では query に近い形に寄せると変換場所が少なくなります。

### 5-2. purge method を追加する

同じ file に method を追加します。

```go
func (s *FileService) PurgeDeletedBodies(ctx context.Context, input FilePurgeInput) (FilePurgeResult, error) {
    if s == nil || s.queries == nil || s.storage == nil {
        return FilePurgeResult{}, fmt.Errorf("file service is not configured")
    }
    if input.BatchSize <= 0 {
        input.BatchSize = 50
    }
    if input.LockTimeout <= 0 {
        input.LockTimeout = 15 * time.Minute
    }
    if input.Cutoff.IsZero() {
        return FilePurgeResult{}, fmt.Errorf("%w: purge cutoff is required", ErrInvalidFileInput)
    }

    rows, err := s.queries.ClaimDeletedFileObjectsForPurge(ctx, db.ClaimDeletedFileObjectsForPurgeParams{
        WorkerID:       strings.TrimSpace(input.WorkerID),
        StorageDrivers: []string{FileStorageDriverName(s.storage)},
        Cutoff:         pgtype.Timestamptz{Time: input.Cutoff, Valid: true},
        LockTimeout:    pgtype.Interval{Microseconds: input.LockTimeout.Microseconds(), Valid: true},
        BatchSize:      input.BatchSize,
    })
    if err != nil {
        return FilePurgeResult{}, fmt.Errorf("claim deleted file objects: %w", err)
    }

    result := FilePurgeResult{Claimed: int64(len(rows))}
    for _, row := range rows {
        if err := s.purgeDeletedBody(ctx, row); err != nil {
            result.Failed++
            continue
        }
        result.Purged++
    }
    return result, nil
}
```

`purgeDeletedBody` は 1 file ごとの処理に分けます。unit test で成功 / 失敗を切りやすくするためです。

### 5-3. 1 file の purge 処理を書く

```go
func (s *FileService) purgeDeletedBody(ctx context.Context, row db.FileObject) error {
    if row.StorageDriver != "local" {
        err := fmt.Errorf("unsupported storage driver for purge")
        _, _ = s.queries.MarkFileObjectPurgeFailed(ctx, db.MarkFileObjectPurgeFailedParams{
            ID:        row.ID,
            LastError: err.Error(),
        })
        return err
    }

    if err := s.storage.Delete(ctx, row.StorageKey); err != nil {
        _, _ = s.queries.MarkFileObjectPurgeFailed(ctx, db.MarkFileObjectPurgeFailedParams{
            ID:        row.ID,
            LastError: err.Error(),
        })
        return err
    }

    purged, err := s.queries.MarkFileObjectBodyPurged(ctx, row.ID)
    if err != nil {
        return fmt.Errorf("mark file body purged: %w", err)
    }

    if s.audit != nil {
        tenantID := purged.TenantID
        s.audit.RecordBestEffort(ctx, AuditEventInput{
            AuditContext: AuditContext{
                ActorType: AuditActorSystem,
                TenantID:  &tenantID,
            },
            Action:     "file.purge",
            TargetType: "file",
            TargetID:   purged.PublicID.String(),
            Metadata: map[string]any{
                "purpose":       purged.Purpose,
                "contentType":   purged.ContentType,
                "byteSize":      purged.ByteSize,
                "storageDriver": purged.StorageDriver,
            },
        })
    }

    return nil
}
```

audit metadata に `original_filename` と `storage_key` は入れません。filename は機密情報を含むことがあり、storage key は内部 path の調査情報に寄せるべきです。

### 5-4. file object mapping を更新する

`FileObject` domain type に `DeletedAt` と `PurgedAt` が必要なら追加します。

```go
DeletedAt *time.Time
PurgedAt  *time.Time
```

API response には出さなくて構いません。P12 の public contract は変えないため、service / test / operator query 用に留めます。

## Step 6. DataLifecycleJob から purge を呼ぶ

### 6-1. purger interface を追加する

#### ファイル: `backend/internal/jobs/data_lifecycle.go`

`jobs` package から `service.FileService` に直接依存しすぎないよう、必要な method だけの interface を置きます。

```go
type DeletedFilePurger interface {
    PurgeDeletedBodies(ctx context.Context, input service.FilePurgeInput) (service.FilePurgeResult, error)
}
```

`jobs` package は既に outbox worker で service package を扱っています。P12 でも同じ粒度で問題ありません。

### 6-2. config を拡張する

```go
type DataLifecycleConfig struct {
    Enabled               bool
    Interval              time.Duration
    Timeout               time.Duration
    RunOnStartup          bool
    OutboxRetention       time.Duration
    NotificationRetention time.Duration
    FileDeletedRetention  time.Duration
    FilePurgeBatchSize    int
    FilePurgeLockTimeout  time.Duration
    WorkerID              string
}
```

`WorkerID` は log と `purge_locked_by` 用です。未指定なら hostname と pid から作ります。実装は `OutboxWorker` の `WorkerID` 生成を参考にします。

### 6-3. job struct と constructor を更新する

```go
type DataLifecycleJob struct {
    queries    *db.Queries
    filePurger DeletedFilePurger
    config     DataLifecycleConfig
    logger     *slog.Logger
    metrics    DataLifecycleMetrics
    running    atomic.Bool
}

func NewDataLifecycleJob(queries *db.Queries, filePurger DeletedFilePurger, config DataLifecycleConfig, logger *slog.Logger, metrics DataLifecycleMetrics) *DataLifecycleJob {
    ...
}
```

OpenAPI export や test で `filePurger` が nil の場合は、file body purge だけ no-op にします。

### 6-4. RunOnce の file cleanup を置き換える

既存のこの処理を削ります。

```go
fileBefore := time.Now().Add(-j.config.FileDeletedRetention)
filesTouched, err := j.queries.SoftDeleteDeletedFileObjectsBefore(ctx, pgtype.Timestamptz{Time: fileBefore, Valid: true})
if err != nil {
    return fmt.Errorf("cleanup deleted file objects: %w", err)
}
j.addItems("file_objects", filesTouched)
```

代わりに、file purger を呼びます。

```go
fileBefore := time.Now().Add(-j.config.FileDeletedRetention)
if j.filePurger != nil {
    result, err := j.filePurger.PurgeDeletedBodies(ctx, service.FilePurgeInput{
        Cutoff:      fileBefore,
        BatchSize:   int32(j.config.FilePurgeBatchSize),
        WorkerID:    j.config.WorkerID,
        LockTimeout: j.config.FilePurgeLockTimeout,
    })
    if err != nil {
        return fmt.Errorf("purge deleted file bodies: %w", err)
    }
    j.addItems("file_objects_body_purged", result.Purged)
    j.addItems("file_objects_body_purge_failed", result.Failed)
}
```

`Claimed` は debug log に出してもよいですが、metrics では成功 / 失敗の方が運用上分かりやすいです。

## Step 7. config と main wiring を追加する

### 7-1. config に環境変数を追加する

#### ファイル: `backend/internal/config/config.go`

`Config` に追加します。

```go
FilePurgeBatchSize   int
FilePurgeLockTimeout time.Duration
```

`Load` で読み込みます。

```go
filePurgeLockTimeout, err := getEnvPositiveDuration("FILE_PURGE_LOCK_TIMEOUT", "15m")
if err != nil {
    return Config{}, err
}
```

return value では次のように設定します。

```go
FilePurgeBatchSize:   positiveInt(getEnvInt("FILE_PURGE_BATCH_SIZE", 50), 50),
FilePurgeLockTimeout: filePurgeLockTimeout,
```

### 7-2. `.env.example` を更新する

#### ファイル: `.env.example`

data lifecycle の近くに追加します。

```dotenv
FILE_PURGE_BATCH_SIZE=50
FILE_PURGE_LOCK_TIMEOUT=15m
```

`FILE_DELETED_RETENTION=720h` は既存のまま、soft delete から physical purge までの猶予期間として扱います。

### 7-3. main で file service を渡す

#### ファイル: `backend/cmd/main/main.go`

`NewDataLifecycleJob` の呼び出しを更新します。

```go
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
```

`fileService` は既に local storage、tenant settings、audit、metrics を持っているため、P12 の purge 実装に必要な依存を再構築しなくて済みます。

## Step 8. audit / metrics / runbook を更新する

### 8-1. audit event 方針

P12 で追加する audit action は次だけです。

| action | actor | target | 用途 |
| --- | --- | --- | --- |
| `file.purge` | system | file public id | retention 経過後の physical body deletion |

metadata は低感度情報だけにします。

```json
{
  "purpose": "attachment",
  "contentType": "text/plain",
  "byteSize": 123,
  "storageDriver": "local"
}
```

次は入れません。

- original filename
- storage key
- local filesystem path
- tenant slug
- user email

### 8-2. metrics 方針

既存の `haohao_data_lifecycle_items_total{kind=...}` を使います。

P12 で使う `kind` は次です。

```text
file_objects_body_purged
file_objects_body_purge_failed
```

追加の counter は不要です。`kind` は固定文字列だけにし、file id や tenant id を含めません。

### 8-3. deployment runbook を更新する

#### ファイル: `RUNBOOK_DEPLOYMENT.md`

Retention の file_objects 行を更新します。

```md
- `file_objects`: soft-deleted metadata is retained as a tombstone; local file bodies are purged after `FILE_DELETED_RETENTION` and marked with `purged_at`.
```

Backup の注意も補足します。

```md
- Local file storage: once `purged_at` is set, the body is intentionally absent from `FILE_LOCAL_DIR`; restore drills should not expect purged bodies to reappear.
```

## Step 9. unit test と smoke を追加する

### 9-1. local storage test

#### ファイル: `backend/internal/service/local_file_storage_test.go`

最低限、`Delete` が idempotent であることを確認します。

```text
Save -> Delete -> Delete again returns nil
```

P12 の retry はこの性質に依存します。

### 9-2. FileService purge test

#### ファイル: `backend/internal/service/file_service_test.go`

DB helper と temp dir を使い、次を確認します。

- active file は purge claim されない
- deleted だが retention 前の file は purge claim されない
- deleted かつ cutoff 前の local file は body が削除され、`purged_at` が入る
- body が既に存在しなくても `purged_at` が入る
- storage delete が失敗した場合は `last_purge_error` が入り、`purged_at` は入らない
- `storage_driver != 'local'` は purge しない

storage failure test は fake storage を使うと書きやすいです。

```go
type failingDeleteStorage struct {
    service.FileStorage
}

func (s failingDeleteStorage) Delete(ctx context.Context, key string) error {
    return errors.New("delete failed")
}
```

### 9-3. DataLifecycleJob test

#### ファイル: `backend/internal/jobs/data_lifecycle_test.go`

fake purger を渡して、`RunOnce` が file purge を呼び、metrics kind を増やすことを確認します。

確認すること:

- `FileDeletedRetention` から cutoff が計算される
- purger が nil の場合でも job は落ちない
- purger error は `RunOnce` の error になる
- success / failure 件数が `file_objects_body_purged` / `file_objects_body_purge_failed` に流れる

### 9-4. smoke script を追加する

#### ファイル: `scripts/smoke-file-purge.sh`

single binary を一時 file directory で起動し、実 file body が削除されることを確認します。

流れ:

1. `make binary` 済みの `./bin/haohao` を使う
2. temp `FILE_LOCAL_DIR` を作る
3. `FILE_DELETED_RETENTION=1s`、`DATA_LIFECYCLE_INTERVAL=200ms` で起動する
4. demo user で login
5. active tenant を `acme` にする
6. Customer Signal を作る
7. file を upload する
8. DB から `storage_key` を取得し、`FILE_LOCAL_DIR/$storage_key` が存在することを確認する
9. file delete API で soft delete する
10. `purged_at IS NOT NULL` かつ file path が消えるまで待つ
11. `/metrics` に `file_objects_body_purged` が出ることを確認する

storage key の取得例:

```bash
storage_key="$(psql "$DATABASE_URL" -tA -c "SELECT storage_key FROM file_objects WHERE public_id = '$file_public_id'")"
file_path="$FILE_DIR/$storage_key"
test -f "$file_path"
```

purge 待ちの例:

```bash
purged=0
for _ in {1..80}; do
  purged_at="$(psql "$DATABASE_URL" -tA -c "SELECT COALESCE(purged_at::text, '') FROM file_objects WHERE public_id = '$file_public_id'")"
  if [[ -n "$purged_at" && ! -e "$file_path" ]]; then
    purged=1
    break
  fi
  sleep 0.25
done

if [[ "$purged" != "1" ]]; then
  echo "expected file body to be purged" >&2
  exit 1
fi
```

### 9-5. Makefile target を追加する

#### ファイル: `Makefile`

single binary と temp file storage を使うため、`binary` に依存させます。

```make
smoke-file-purge: binary
	bash scripts/smoke-file-purge.sh
```

既存 smoke と同じく、PostgreSQL と Redis は local dependency として使います。

## Step 10. 生成と確認

### 10-1. 生成物を更新する

P12 は DB schema と sqlc query を変更します。まず生成を一通り実行します。

```bash
make gen
```

期待値:

- `db/schema.sql` に P12 column / index が入る
- `backend/internal/db/file_objects.sql.go` に purge query が入る
- `backend/internal/db/models.go` の `FileObject` に purge column が入る
- OpenAPI / frontend SDK に意味のある差分は出ない

OpenAPI に差分が出る場合は、P12 で API response type を変えていないか確認します。

### 10-2. backend test

```bash
go test ./backend/internal/service ./backend/internal/jobs
go test ./backend/...
```

失敗時の見方:

- sqlc compile error が出る場合は query parameter 名と generated params type を確認する
- purge test で body が残る場合は `storage_key` と temp root の join を確認する
- `purged_at` が入らない場合は `MarkFileObjectBodyPurged` の再確認条件に合っているか確認する
- retry test が落ちる場合は `LocalFileStorage.Delete` が missing file を nil にしているか確認する
- data lifecycle test が落ちる場合は nil purger と config default の扱いを確認する

### 10-3. frontend build

P12 は UI を変えませんが、generated SDK に意図しない差分がないことを確認します。

```bash
npm --prefix frontend run build
```

### 10-4. single binary

```bash
make binary
```

P12 は runtime job と local file storage の変更なので、single binary で確認します。

### 10-5. common services smoke

```bash
make smoke-common-services
```

既存の upload / download / soft delete が壊れていないことを確認します。

ここで失敗する場合は、P12 で public file API の response や authorization を変えていないか確認します。

### 10-6. file purge smoke

```bash
make smoke-file-purge
```

期待値:

- upload 直後は `FILE_LOCAL_DIR/$storage_key` が存在する
- delete API 後、download API は `404` になる
- retention 経過後、実 file body が消える
- `file_objects.purged_at` が入る
- `/metrics` に `file_objects_body_purged` が出る

失敗時の見方:

- file path が存在しない場合は smoke の `FILE_LOCAL_DIR` と server の `FILE_LOCAL_DIR` が一致しているか確認する
- `purged_at` が入らない場合は `DATA_LIFECYCLE_ENABLED=true`、`DATA_LIFECYCLE_INTERVAL`、`FILE_DELETED_RETENTION` を確認する
- `last_purge_error` が入る場合は file permission、storage key、root path を確認する
- `purge_locked_at` が残る場合は job が途中で落ちていないか、`FILE_PURGE_LOCK_TIMEOUT` が長すぎないか確認する
- metrics が出ない場合は `METRICS_ENABLED=true` と `DataLifecycleMetrics` wiring を確認する
- `psql` が `/tmp/.s.PGSQL.5432` に接続しようとする場合は、確認 terminal で `DATABASE_URL` が export されていない。`set -a && source .env && set +a` を実行してから再試行する
- `curl: (7) Failed to connect` が出る場合は、確認対象の `BASE_URL` / port で server が起動していない。`curl "$BASE_URL/readyz"` と `lsof -nP -iTCP:<port> -sTCP:LISTEN` で確認する
- DB row が `status = active` なのに `FILE_LOCAL_DIR/$storage_key` が存在しない場合は、ほぼ `FILE_LOCAL_DIR` の取り違え。server 起動時の値と確認 terminal の `FILE_DIR` が同じか確認する

### 10-7. E2E

```bash
make e2e
```

既存 E2E は `DATA_LIFECYCLE_ENABLED=false` で動いている場合があります。その場合は正常です。P12 の physical purge は `make smoke-file-purge` で確認し、E2E では browser journey が壊れていないことを確認します。

## 手動確認

`make smoke-file-purge` は自動確認用です。手で 1 個ずつ見たい場合は、この節の手順で確認します。

手動確認では、server を起動する terminal と curl / psql で確認する terminal が別になります。環境変数は terminal 間で共有されないため、特に次の 3 つを両方の terminal で同じ値にします。

- `DATABASE_URL`
- `BASE_URL`
- `FILE_DIR`

`FILE_DIR` は、server 起動時には `FILE_LOCAL_DIR` として渡します。確認側では `FILE_DIR/$storage_key` を `ls` します。ここがずれると、DB row は `active` なのに file path が見つからない、という紛らわしい状態になります。

手動確認では、原則として新しく upload した file の `FILE_ID` を使います。過去の `FILE_ID` を流用する場合は、その file がどの `FILE_LOCAL_DIR` で作られたか分からなくなりやすいため、先に次の状態確認をします。

```bash
echo "FILE_ID=$FILE_ID"
echo "FILE_DIR=$FILE_DIR"

psql "$DATABASE_URL" -c "
SELECT public_id, status, deleted_at, purged_at, purge_attempts, last_purge_error, storage_key
FROM file_objects
WHERE public_id = '$FILE_ID';
"
```

読み方:

| DB state | file path state | 意味 |
| --- | --- | --- |
| `status = active`、`deleted_at IS NULL` | file が存在する | upload 後、delete 前。正常 |
| `status = active`、`deleted_at IS NULL` | file が存在しない | purge ではない。確認している `FILE_DIR` が server の `FILE_LOCAL_DIR` と違う可能性が高い |
| `status = deleted`、`purged_at IS NULL` | file が存在する | soft delete 後、retention / lifecycle purge 待ち |
| `status = deleted`、`purged_at IS NOT NULL` | file が存在しない | P12 の成功状態 |
| `last_purge_error IS NOT NULL` | 任意 | purge は失敗して retry 待ち。error 内容と file permission / path を確認する |

### まず前提を揃える

```bash
cd /Users/pochy/Projects/HaoHao

make up
make db-up
make seed-demo-user
make binary
```

確認用の固定 directory を使います。`mktemp` でも動きますが、手動確認ではどの terminal でも同じ値を再設定しやすい固定 path の方が安全です。

```bash
export BASE_URL=http://127.0.0.1:18081
export FILE_DIR=/tmp/haohao-manual-file-purge

rm -rf "$FILE_DIR"
mkdir -p "$FILE_DIR"
```

`.env` の `DATABASE_URL` は `make db-up` では読まれますが、自分の shell には自動 export されません。psql を直接使う terminal では必ず読み込みます。

```bash
set -a
source .env
set +a

echo "$DATABASE_URL"
psql "$DATABASE_URL" -c 'select 1;'
```

`psql` が `/tmp/.s.PGSQL.5432` に接続しようとして失敗する場合は、`DATABASE_URL` が空です。上の `set -a && source .env && set +a` を再実行します。

### Terminal A: server を起動する

`.env` が `AUTH_MODE=zitadel` でも手動確認しやすいように、ここでは `AUTH_MODE=local` を明示します。

```bash
cd /Users/pochy/Projects/HaoHao

set -a
source .env
set +a

export BASE_URL=http://127.0.0.1:18081
export FILE_DIR=/tmp/haohao-manual-file-purge

HTTP_PORT=18081 \
APP_BASE_URL="$BASE_URL" \
FRONTEND_BASE_URL="$BASE_URL" \
DATABASE_URL="$DATABASE_URL" \
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}" \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
COOKIE_SECURE=false \
DOCS_AUTH_REQUIRED=false \
RATE_LIMIT_ENABLED=false \
FILE_LOCAL_DIR="$FILE_DIR" \
DATA_LIFECYCLE_ENABLED=true \
DATA_LIFECYCLE_INTERVAL=200ms \
DATA_LIFECYCLE_TIMEOUT=5s \
DATA_LIFECYCLE_RUN_ON_STARTUP=true \
FILE_DELETED_RETENTION=1s \
FILE_PURGE_BATCH_SIZE=10 \
FILE_PURGE_LOCK_TIMEOUT=2s \
./bin/haohao
```

この terminal は起動したままにします。別 terminal から次を叩いて ready になっていれば OK です。

```bash
curl -fsS "$BASE_URL/readyz"
```

`curl: (7) Failed to connect` が出る場合は、server がまだ起動していないか、port / `BASE_URL` が違います。

```bash
lsof -nP -iTCP:18081 -sTCP:LISTEN
```

server が起動していない状態で DELETE API を叩いても DB は変わりません。その場合、`file_objects.status` は `active` のままで、`deleted_at` も `purged_at` も入りません。

### Terminal B: login と tenant 選択

別 terminal を開き、同じ `BASE_URL` / `FILE_DIR` を設定します。

```bash
cd /Users/pochy/Projects/HaoHao

set -a
source .env
set +a

export BASE_URL=http://127.0.0.1:18081
export FILE_DIR=/tmp/haohao-manual-file-purge
export COOKIE_JAR="$(mktemp)"
export UPLOAD_FILE="$(mktemp)"

printf 'hello file purge\n' > "$UPLOAD_FILE"
```

login します。

```bash
curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$BASE_URL/api/v1/login" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/csrf" >/dev/null

export CSRF="$(awk '$6 == "XSRF-TOKEN" { print $7 }' "$COOKIE_JAR" | tail -n 1)"
test -n "$CSRF"
```

tenant を `acme` にします。

```bash
curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $CSRF" \
  -d '{"tenantSlug":"acme"}' \
  "$BASE_URL/api/v1/session/tenant" | rg '"slug":"acme"'
```

### 1. file upload 後に実ファイルが存在する

file を attachment として upload するため、まず customer signal を 1 件作ります。

```bash
CREATE_RESPONSE="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $CSRF" \
  -d '{"customerName":"Acme","title":"manual purge check","body":"manual","source":"support","priority":"medium","status":"new"}' \
  "$BASE_URL/api/v1/customer-signals")"

export SIGNAL_ID="$(printf '%s' "$CREATE_RESPONSE" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
test -n "$SIGNAL_ID"
```

file を upload します。

```bash
UPLOAD_RESPONSE="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $CSRF" \
  -F "purpose=attachment" \
  -F "attachedToType=customer_signal" \
  -F "attachedToId=$SIGNAL_ID" \
  -F "file=@$UPLOAD_FILE;filename=manual-purge.txt;type=text/plain" \
  "$BASE_URL/api/v1/files")"

export FILE_ID="$(printf '%s' "$UPLOAD_RESPONSE" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
test -n "$FILE_ID"
```

DB から `storage_key` を取り、実 file body があることを確認します。

```bash
export STORAGE_KEY="$(psql "$DATABASE_URL" -tA -c "SELECT storage_key FROM file_objects WHERE public_id = '$FILE_ID'")"
export FILE_PATH="$FILE_DIR/$STORAGE_KEY"

echo "FILE_ID=$FILE_ID"
echo "FILE_DIR=$FILE_DIR"
echo "STORAGE_KEY=$STORAGE_KEY"
echo "FILE_PATH=$FILE_PATH"

ls -l "$FILE_PATH"
```

ここで `ls` が成功すれば、upload 後の body 存在確認は OK です。

`status = active` なのに `ls` が失敗する場合は、purge ではありません。`FILE_DIR` が server 起動時の `FILE_LOCAL_DIR` と違います。実 file の場所を探すには次を使います。

```bash
find .data/files /tmp "${TMPDIR:-/tmp}" -path "*/$STORAGE_KEY" -type f 2>/dev/null
```

見つかった path が次のような形なら:

```text
/var/folders/.../tmp.y2ZlLtixOa/tenants/1/files/<uuid>
```

正しい `FILE_DIR` は `tenants/...` より前の部分です。

```bash
export FILE_DIR=/var/folders/.../tmp.y2ZlLtixOa
export FILE_PATH="$FILE_DIR/$STORAGE_KEY"
ls -l "$FILE_PATH"
```

### 2. DELETE API 後に download が 404 になる

soft delete します。

```bash
curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $CSRF" \
  -X DELETE \
  "$BASE_URL/api/v1/files/$FILE_ID" >/dev/null
```

download が `404` になることを確認します。

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/files/$FILE_ID"
```

期待値:

```text
404
```

DB ではこの時点で `status = deleted`、`deleted_at` ありになります。

```bash
psql "$DATABASE_URL" -c "
SELECT status, deleted_at, purged_at, purge_attempts, last_purge_error
FROM file_objects
WHERE public_id = '$FILE_ID';
"
```

### 3. `FILE_DELETED_RETENTION=1s` 経過後に DataLifecycleJob が purge する

server は `DATA_LIFECYCLE_INTERVAL=200ms`、`FILE_DELETED_RETENTION=1s` で起動しているため、通常は数秒以内に purge されます。

```bash
for i in {1..20}; do
  psql "$DATABASE_URL" -c "
  SELECT status, deleted_at, purged_at, purge_attempts, last_purge_error
  FROM file_objects
  WHERE public_id = '$FILE_ID';
  "
  sleep 0.5
done
```

期待値:

- `status = deleted`
- `deleted_at` が入っている
- `purged_at` が入っている
- `purge_attempts >= 1`
- `last_purge_error` が空

### 4. 実ファイル body が消える

`purged_at` が入った後、body が消えていることを確認します。

```bash
test ! -e "$FILE_PATH" && echo "file body purged"
```

期待値:

```text
file body purged
```

`purged_at` が入っているのに file が残る場合は、server が別の `FILE_LOCAL_DIR` を見ている可能性があります。`FILE_PATH` と server 起動時の `FILE_LOCAL_DIR` を再確認します。

### 5. `/metrics` に `file_objects_body_purged` が出る

```bash
curl -sS "$BASE_URL/metrics" | rg 'file_objects_body_purged'
```

期待値の例:

```text
haohao_data_lifecycle_items_total{app_version="0.1.0",kind="file_objects_body_purged"} 1
```

この行が出れば、DataLifecycleJob の purge 件数が metrics に流れています。

### 手動確認の成功条件

最後に次が全部成立していれば P12 の手動確認は完了です。

- upload 後に `ls -l "$FILE_PATH"` が成功した
- DELETE 後に download API が `404` を返した
- `file_objects.status = deleted`
- `file_objects.deleted_at` が入った
- `file_objects.purged_at` が入った
- `file_objects.last_purge_error` が空
- `test ! -e "$FILE_PATH"` が成功した
- `/metrics` に `kind="file_objects_body_purged"` が出た

## トラブルシュート

### soft delete はできるが body が消えない

見る場所:

```bash
rg -n "PurgeDeletedBodies|ClaimDeletedFileObjectsForPurge|FilePurge" backend db
```

確認すること:

- `DataLifecycleJob` に `fileService` を渡している
- `DATA_LIFECYCLE_ENABLED=true` で起動している
- `FILE_DELETED_RETENTION` を過ぎている
- `storage_driver` が起動中の `FILE_STORAGE_DRIVER` と一致している
- `purged_at IS NULL` の row が残っている
- `purge_locked_at` が stale lock timeout 内で止まっていない

### upload したはずの body が見つからない

まず DB row がどの状態か確認します。

```bash
psql "$DATABASE_URL" -c "
SELECT public_id, status, deleted_at, purged_at, purge_attempts, last_purge_error, storage_key
FROM file_objects
WHERE public_id = '$FILE_ID';
"
```

`status = active` かつ `purged_at IS NULL` なら、P12 の purge では body は消えていません。ほぼ `FILE_DIR` の取り違えです。実 body を探します。

```bash
find .data/files /tmp "${TMPDIR:-/tmp}" -path "*/$STORAGE_KEY" -type f 2>/dev/null
```

見つかった path の `tenants/...` より前が、upload 時に server が使っていた `FILE_LOCAL_DIR` です。確認 terminal の `FILE_DIR` をその値に合わせます。

```bash
export FILE_DIR="/path/from/find/output/before/tenants"
export FILE_PATH="$FILE_DIR/$STORAGE_KEY"
ls -l "$FILE_PATH"
```

手動確認でこの状態になりたくない場合は、server 起動前に固定 path を使います。

```bash
export FILE_DIR=/tmp/haohao-manual-file-purge
rm -rf "$FILE_DIR"
mkdir -p "$FILE_DIR"
```

そして server 起動時に必ず同じ値を渡します。

```bash
FILE_LOCAL_DIR="$FILE_DIR" ./bin/haohao
```

### DELETE API が効いていない

DELETE 後も DB row が `status = active` のままなら、DELETE API は成功していません。まず HTTP status を捨てずに見ます。

```bash
curl -sS -o /tmp/delete-file-response.json -w '%{http_code}\n' \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $CSRF" \
  -X DELETE \
  "$BASE_URL/api/v1/files/$FILE_ID"

cat /tmp/delete-file-response.json
```

よくある原因:

- server がその `BASE_URL` / port で起動していない
- `COOKIE_JAR` が別 server / 別 session のもの
- `CSRF` が空
- active tenant が `acme` ではない
- `FILE_ID` が別 tenant の file

server 起動確認:

```bash
curl -fsS "$BASE_URL/readyz"
lsof -nP -iTCP:18081 -sTCP:LISTEN
```

### `purge_locked_at` が残り続ける

まず job process の log を確認します。

```bash
rg "data lifecycle|purge deleted file" /tmp/haohao-*.log
```

`FILE_PURGE_LOCK_TIMEOUT` より古い lock が残っているなら、次回 run で再 claim されるはずです。再 claim されない場合は claim query の stale lock 条件を確認します。

### `last_purge_error` が入る

DB で直近 error を見ます。

```bash
psql "$DATABASE_URL" -c "
SELECT public_id, storage_driver, purge_attempts, last_purge_error
FROM file_objects
WHERE last_purge_error IS NOT NULL
ORDER BY updated_at DESC
LIMIT 20;
"
```

よくある原因:

- `FILE_LOCAL_DIR` の volume が mount されていない
- runtime user に delete 権限がない
- storage key が root 外を指していて `pathForKey` で拒否されている
- local storage 以外の `storage_driver` が混ざっている

### `storage.Delete` 後に DB 更新が失敗した

この状態では body は消えていますが `purged_at` が未設定です。

P12 ではこれを許容します。次回 run で `storage.Delete` は missing file を成功扱いにし、`MarkFileObjectBodyPurged` を再試行します。

この復旧ができることを unit test と smoke で確認してください。

### hard delete したくなる場合

P12 では hard delete しません。

理由は、現時点で `customer_signal_imports.input_file_object_id` が `ON DELETE RESTRICT` で file metadata を参照しているためです。metadata hard delete を入れる場合は、少なくとも次を別チュートリアルで設計します。

- import / export job history の保持期間
- `ON DELETE RESTRICT` の変更可否
- audit event と tenant data export における tombstone の扱い
- 物理 body purge 済み metadata を何日残すか
- restore drill で metadata と body の不一致をどう説明するか

## 完了後の期待状態

P12 完了後、file lifecycle は次の状態になります。

```text
upload
  -> file_objects.status = active
  -> local body exists

delete API
  -> file_objects.status = deleted
  -> deleted_at is set
  -> local body still exists during retention

data lifecycle after FILE_DELETED_RETENTION
  -> local body is removed
  -> purged_at is set
  -> metadata tombstone remains
```

この形にすると、ユーザー向けには soft delete 直後から file は見えず、運用向けには retention 後に storage capacity が回収され、事故調査向けには file metadata tombstone が残ります。
