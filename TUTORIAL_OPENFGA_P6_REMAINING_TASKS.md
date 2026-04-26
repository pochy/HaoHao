# Phase 6: OpenFGA Drive 残タスク実装チュートリアル

## この文書の目的

この文書は、`TUTORIAL_OPENFGA_P5_UI_E2E.md` までで導入した OpenFGA Drive を、実運用向けに拡張するための続編チュートリアルです。

Phase 1-5 では、Owner / Editor / Viewer、folder/file 継承、user share、group share、期限付き share link、download 禁止、Drive UI、audit、smoke、E2E までを実装しました。Phase 6 では、その時点で意図的に残した要件を **実装順に分けた backlog** として扱います。

この文書の目的は、残タスクを一度に全部実装することではありません。目的は、次にどの領域から着手し、どの DB / API / UI / audit / test を追加すべきかを、他のチュートリアルと同じ粒度で迷わず追えるようにすることです。

特にこの Phase 6 は、`TUTORIAL_P5_TENANT_ADMIN_UI.md` で作った tenant admin 画面の続きとして、Drive policy、Drive audit、共有状態、整合性運用を tenant admin の管理対象へ広げます。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `TUTORIAL_OPENFGA_P1_INFRA_MODEL.md` から `TUTORIAL_OPENFGA_P5_UI_E2E.md` までが実装済み
- `openfga/drive.fga` と `openfga/drive.fga.yaml` が repo 管理されている
- `OPENFGA_ENABLED=true` で backend が OpenFGA `/healthz` を readiness に含められる
- Drive metadata は `drive_folders`、`file_objects(purpose='drive')`、`drive_groups`、`drive_group_members`、`drive_resource_shares`、`drive_share_links` に保存されている
- `DriveAuthorizationService` が OpenFGA SDK 呼び出しを service 層に閉じ込めている
- `DriveService` が DB tenant boundary、resource state、tenant policy、OpenFGA check/write/delete を扱っている
- browser Drive API は Cookie session + CSRF + active tenant を前提に `/api/v1/drive/...` に追加済み
- public share link metadata/content endpoint がある
- Vue app に Drive browser、group view、public share link view、share dialog、permissions panel がある
- tenant admin UI は global role `tenant_admin` で保護されている
- `tenant_admin` は Drive policy と audit への入口を扱えるが、明示共有なしにファイル本文は閲覧できない
- `make smoke-openfga` と OpenFGA enabled E2E が通る

この Phase 6 でも、既存の Zitadel / AuthzService / tenant-aware auth を置き換えません。

## この Phase でやること / やらないこと

やること:

```text
Drive tenant policy を typed policy として扱う
外部ユーザー共有と未登録メール招待を tenant membership なしで扱う
domain allow / deny と管理者承認を tuple write 前に適用する
password 付き share link と rate limit を public access に追加する
tenant admin が Drive audit / 共有状態 / repair 状態を調査できるようにする
DB を source of truth とした pending_sync repair / tuple drift 検出を作る
user lifecycle と Drive group sync の整理 job を用意する
restore / purge / retention / legal hold の service guard を先に入れる
```

やらないこと:

```text
tenant admin によるファイル本文の横断閲覧
匿名 Editor link
workspace resource の本格導入
object storage driver と signed URL の本格導入
virus scan / DLP / billing / HA/DR
external bearer / M2M Drive API の全 endpoint 実装
```

Phase 6 の中心は「共有拡張、管理 UI、整合性運用」です。authorization model を広げる Step は採用判断だけを固定し、実際の model 変更は Phase 7 以降へ分けます。

## 完成条件

Phase 6 backlog を一通り実装した状態の完成条件は次です。

- tenant admin が Drive policy を UI から更新できる
- 外部ユーザー直接共有と未登録メール招待の保存、承認、失効、受諾が audit 付きで扱える
- domain allow / deny と管理者承認フローにより、外部共有 tuple が作られる前に DB policy で止められる
- password 付き share link が作成でき、password hash と rate limit を使って public access を制御できる
- tenant admin が Drive audit と共有状態を検索できるが、ファイル本文は開けない
- `pending_sync` share/link を検出し、repair job で OpenFGA tuple を再同期できる
- OpenFGA tuple drift を dry-run で検出できる
- deactivated user の Drive tuple cleanup と SCIM group to Drive group sync の方針が実装または明示的に disabled になる
- restore / complete delete / retention / legal hold の下準備が DB と service guard に入る
- Owner 移譲、Commenter / Uploader、explicit deny は採用可否と実装順が分かる状態になる
- external bearer / M2M Drive API は browser API と混ぜず、scope と tenant 解決方針が固定される
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make smoke-openfga`、OpenFGA enabled E2E が通る

## 実装順の全体像

| Step | 主題 | 目的 |
| --- | --- | --- |
| Step 1 | Drive tenant policy API / UI の拡張 | 外部共有、password link、承認、最大 TTL などを tenant admin から管理できるようにする |
| Step 2 | 外部ユーザー直接共有と未登録メール招待の土台 | tenant membership を増やさず、Drive resource 単位で外部 user に共有できるようにする |
| Step 3 | domain allow / deny と管理者承認フロー | 外部共有 tuple を作る前に tenant policy と承認で止める |
| Step 4 | password 付き share link | public link access に password hash、rate limit、短命 verification を追加する |
| Step 5 | Drive audit / 共有状態 admin UI | tenant admin が共有状態と audit を調査できるようにする |
| Step 6 | `pending_sync` repair と OpenFGA tuple drift 検出 | DB と OpenFGA のずれを検出し、fail-closed を維持したまま修復する |
| Step 7 | deactivated user / SCIM group 同期の整理 job | user lifecycle と Drive tuple / group membership の整合性を保つ |
| Step 8 | restore / complete delete / retention / legal hold の下準備 | ファイルライフサイクルの禁止条件を DB と service に先に入れる |
| Step 9 | Owner 移譲、Commenter / Uploader、explicit deny の検討枠 | authorization model を変える前に採用順と影響範囲を固定する |
| Step 10 | external bearer / M2M Drive API の境界定義 | browser surface と external surface を分けたまま Drive API を広げる |
| Step 11 | smoke / E2E / final verification | 外部共有、password link、repair、admin UI の回帰確認を固定する |

## 先に決める方針

### Phase 6 は Drive/OpenFGA だけを扱う

この文書では SMTP provider、Grafana dashboard、billing、multi-region、HA/DR は扱いません。これらは HaoHao 全体の backlog であり、Drive/OpenFGA の実装手順とは分けます。

ただし、未登録メール招待では既存の invitation / notification / log email sender を使います。外部 email provider が未導入でも、DB state と audit が正しく残ることを先に完成条件にします。

### tenant admin は引き続きファイル本文を閲覧しない

Phase 6 で tenant admin に追加するのは、Drive policy、共有状態、audit、repair 操作です。

tenant admin であっても、Drive file body を download / preview するには通常の Drive resource permission が必要です。管理画面には本文を開く導線を置きません。

### 外部共有は tenant membership を増やさない

外部ユーザーに Drive resource を共有しても、`tenant_memberships` は作りません。tenant に参加させる操作と、Drive resource を限定共有する操作は別物です。

外部ユーザー access は次の条件で許可します。

- user は local login または Zitadel OIDC login で HaoHao user として認証済み
- user は target tenant の member ではない
- accepted / active な Drive invitation または external share grant がある
- DB の resource tenant と share tenant が一致する
- OpenFGA check が許可する

通常の Drive browser は active tenant member 向けです。外部共有された resource は、専用の "shared with me" / invitation access route で扱います。

### OpenFGA tuple は許可が確定してから書く

外部共有、承認待ち、password link、repair のすべてで、DB record が存在するだけでは access を許可しません。

OpenFGA tuple を書く順序は次に固定します。

- 承認不要で即時共有できる場合: DB active row を作り、audit を残し、OpenFGA tuple を書く。tuple write 失敗時は `pending_sync` にして access は許可しない
- 承認が必要な場合: DB pending row と audit だけを作る。承認完了後に OpenFGA tuple を書き、成功してから active にする
- revoke / disable の場合: OpenFGA tuple delete を先に成功させ、その後 DB row を revoked / disabled にする

### raw token と password はログに残さない

share link raw token、password、password verification token、storage key は request log、audit、metrics、E2E trace に残しません。

audit には `shareLinkPublicId`、`resourcePublicId`、`passwordRequired`、`expiresAt`、`policyDecision` のような低感度 metadata だけを残します。

### authorization model の変更は別 bootstrap として扱う

`openfga/drive.fga` を変更する場合、runtime 起動時に model を自動更新しません。

authorization model を変更したら、`openfga/drive.fga.yaml` の model test を増やし、`scripts/openfga-bootstrap.sh` または明示的な model write 手順で新しい `OPENFGA_AUTHORIZATION_MODEL_ID` を環境へ反映します。

## 推奨 PR 分割

Phase 6 は同じ tenant admin UI に見える変更が多いですが、DB state、OpenFGA tuple、public access が絡むため、次の順で PR を分けます。

| PR | 含める Step | merge gate |
| --- | --- | --- |
| 1 | Step 1 | typed Drive policy、validation、tenant admin UI build が通る |
| 2 | Step 2-3 | invitation / approval / domain policy が tuple write 前に fail-closed する |
| 3 | Step 4 | password link、verification cookie、rate limit、public E2E が通る |
| 4 | Step 5 | admin 調査 UI が本文・raw token・storage key を返さない |
| 5 | Step 6-7 | drift dry-run、repair、cleanup job が idempotent に動く |
| 6 | Step 8-10 | lifecycle guard と次 Phase の採用判断が文書化される |
| 7 | Step 11 | smoke / E2E / `IMPL.md` が更新される |

## Step 1. Drive tenant policy API / UI を拡張する

### 対象ファイル

```text
backend/internal/api/tenant_settings.go
backend/internal/service/tenant_settings_service.go
frontend/src/api/tenant-settings.ts
frontend/src/views/TenantAdminTenantDetailView.vue
frontend/src/stores/tenant-admin.ts
```

### 実装方針

既存の tenant settings API は `/api/v1/admin/tenants/{tenantSlug}/settings` を使います。Phase 6 では新しい table を作らず、`tenant_settings.features.drive` を Drive policy の source of truth として拡張します。

Drive policy の shape は次に固定します。

```json
{
  "drive": {
    "linkSharingEnabled": true,
    "publicLinksEnabled": true,
    "externalUserSharingEnabled": false,
    "passwordProtectedLinksEnabled": false,
    "requireShareLinkPassword": false,
    "requireExternalShareApproval": false,
    "allowedExternalDomains": [],
    "blockedExternalDomains": [],
    "maxShareLinkTTLHours": 168,
    "viewerDownloadEnabled": true,
    "externalDownloadEnabled": false,
    "editorCanReshare": false,
    "editorCanDelete": false
  }
}
```

`tenant_settings.features` は自由な map ですが、service 層では Drive policy を typed struct に normalize します。未知 key は preserve してもよいですが、Drive policy の既知 key は型と範囲を検証します。

### API / DB / UI の境界

- DB: `tenant_settings.features` に保存する
- API: 既存 settings response に加えて、frontend で扱いやすい typed Drive policy helper を API wrapper に追加する
- UI: tenant admin detail に Drive policy section を追加する
- service: default policy を `DriveService` と `TenantSettingsService` で共有する

### validation

- `maxShareLinkTTLHours` は `1` から `2160` まで
- domain は lower-case で保存し、先頭の `@` は保存しない
- allow list と block list に同じ domain がある場合は block を優先する
- `requireShareLinkPassword=true` の場合は `passwordProtectedLinksEnabled=true` も要求する
- `externalUserSharingEnabled=false` の場合、未承認の外部共有作成 API は `403` を返す

### audit / metrics / security

- policy 更新は `tenant_settings.drive_policy.update` として audit に残す
- metadata は changed field 名、domain 件数、boolean の変更だけにする
- domain 値そのものを audit に残す場合は tenant admin だけが閲覧できる audit UI に限定する

### 確認

```bash
make gen
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
```

## Step 2. 外部ユーザー直接共有と未登録メール招待の土台を作る

### 対象ファイル

```text
db/migrations/*
db/queries/drive_invitations.sql
backend/internal/service/drive_service.go
backend/internal/api/drive_shares.go
frontend/src/components/DriveShareDialog.vue
frontend/src/views/DriveView.vue
```

### 実装方針

外部共有は、tenant membership 追加ではなく Drive resource 限定の invitation / grant として扱います。

追加する DB table は次です。

```text
drive_share_invitations
```

保持する主な項目:

```text
tenant_id
public_id
resource_type
resource_id
invitee_email_hash
invitee_email_domain
invitee_user_id nullable
role
status: pending / pending_approval / accepted / expired / revoked
expires_at
approved_by_user_id nullable
approved_at nullable
accepted_at nullable
created_by_user_id
created_at
updated_at
accept_token_hash nullable
accept_token_expires_at nullable
masked_invitee_email nullable
```

email raw value は通知送信に必要な最小期間だけ扱います。DB に残す正本は `invitee_email_hash` と domain です。UI 表示用に masked email が必要な場合は `a***@example.com` のような mask 済み値だけ保存します。

招待 acceptance には、email hash だけを使いません。invite URL 用の one-time token を発行し、DB には token hash と expiry だけを保存します。login 済み user が accept する時は、次のいずれかを満たす場合だけ invitee とみなします。

- user の verified email hash が `invitee_email_hash` と一致する
- 既存 user share として `invitee_user_id` が明示されている

domain が一致するだけで invitation を受諾可能にしてはいけません。

### API / DB / UI の境界

- owner は existing tenant user と external email のどちらにも share dialog から共有できる
- external email share は invitation record を作る
- invitee が既存 user の場合も、tenant membership は作らない
- invitee が login / signup 後に invitation を accept すると、OpenFGA tuple を `user:<users.public_id>` に対して書く
- accepted invitation は `drive_resource_shares` にも active row を作り、既存 permissions panel に表示する

追加する API:

```text
POST /api/v1/drive/{resourceType}/{resourcePublicId}/invitations
GET /api/v1/drive/invitations
POST /api/v1/drive/invitations/{invitationPublicId}/accept
POST /api/v1/drive/invitations/{invitationPublicId}/revoke
```

`accept` は invitee 本人だけが実行できます。tenant admin は承認や revoke はできますが、accept は代行しません。

### audit / metrics / security

- invitation 作成は `drive.share_invitation.create`
- accept は `drive.share_invitation.accept`
- revoke は `drive.share_invitation.revoke`
- external user に tuple を書く前に、tenant policy と domain policy を必ず確認する
- invitee email raw value は audit / metrics / request log に出さない

### 確認

```bash
make gen
go test ./backend/internal/service ./backend/internal/api
make smoke-openfga
```

## Step 3. domain allow / deny と管理者承認フローを追加する

### 対象ファイル

```text
backend/internal/service/drive_service.go
backend/internal/service/tenant_settings_service.go
backend/internal/api/tenant_admin.go
frontend/src/views/TenantAdminTenantDetailView.vue
frontend/src/components/DriveShareDialog.vue
```

### 実装方針

外部共有の判定順は次に固定します。

```text
1. externalUserSharingEnabled を確認する
2. invitee domain を lower-case に normalize する
3. blockedExternalDomains に一致したら拒否する
4. allowedExternalDomains が空でなければ、一致する domain だけ許可する
5. requireExternalShareApproval=true なら pending_approval にする
6. 承認不要なら invitation を pending または accepted flow に進める
```

deny は allow より優先します。

domain matching は exact match と subdomain match を分けて実装します。`badexample.com` が `example.com` に一致するような suffix 判定は使いません。保存時は lower-case、末尾 dot 除去、先頭 `@` 除去、IDNA normalize を行い、判定時も同じ正規化を通します。

### API / DB / UI の境界

承認 API を追加します。

```text
GET /api/v1/admin/tenants/{tenantSlug}/drive/share-approvals
POST /api/v1/admin/tenants/{tenantSlug}/drive/share-approvals/{invitationPublicId}/approve
POST /api/v1/admin/tenants/{tenantSlug}/drive/share-approvals/{invitationPublicId}/reject
```

承認 UI は tenant admin detail に追加します。tenant admin は resource name、resource type、requester、role、domain、requested time を見られますが、file content への link は表示しません。

### audit / metrics / security

- approval は `drive.share_approval.approve`
- reject は `drive.share_approval.reject`
- policy reject は `drive.share.external_denied`
- metrics は `haohao_drive_external_share_decisions_total{decision,reason}` のような低 cardinality label にする

### 確認

```bash
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
```

## Step 4. password 付き share link を追加する

### 対象ファイル

```text
db/migrations/*
db/queries/drive_share_links.sql
backend/internal/service/drive_service.go
backend/internal/api/drive_share_links.go
frontend/src/components/DriveShareDialog.vue
frontend/src/views/PublicDriveShareView.vue
```

### 実装方針

`drive_share_links` に password 用 column を追加します。

```text
password_hash nullable
password_required boolean default false
password_updated_at nullable
```

password hash は平文保存しません。既存 local password が PostgreSQL `crypt()` を使っているため、share link password も DB 側 `crypt()` または同等の one-way hash helper に統一します。

public access は次の流れにします。

```text
1. GET /api/public/drive/share-links/{token}
2. password_required=true なら metadata は返すが content access は locked とする
3. POST /api/public/drive/share-links/{token}/password に password を送る
4. password が正しければ短命 verification cookie を発行する
5. content endpoint は token validity、expiry、link status、password verification、OpenFGA condition を確認する
```

verification cookie は HttpOnly、SameSite=Lax、短命、path scoped にします。raw password や verification secret は audit / log に残しません。

### API / DB / UI の境界

- owner は share dialog で password required を選べる
- `requireShareLinkPassword=true` の tenant では password 未設定 link 作成を拒否する
- `can_download=false` link は password が正しくても content download を拒否し、metadata / preview 相当だけ許可する
- password 変更時は既存 verification cookie を無効化できるように `password_updated_at` を見る

### audit / metrics / security

- password 付き link 作成は `drive.share_link.create` metadata に `passwordRequired=true` を残す
- password verification failure は `drive.share_link.password_failed`
- rate limit は raw token ではなく token hash または public ID 相当の低感度 key で行う
- brute force 対策として IP + token hash の組み合わせで一時 block する

### 確認

```bash
go test ./backend/internal/service ./backend/internal/api
make smoke-openfga
E2E_OPENFGA_ENABLED=true make e2e
```

## Step 5. Drive audit / 共有状態 admin UI を追加する

### 対象ファイル

```text
backend/internal/api/audit.go
backend/internal/api/tenant_admin.go
backend/internal/service/audit_service.go
frontend/src/views/TenantAdminTenantDetailView.vue
frontend/src/api/tenant-admin.ts
frontend/src/stores/tenant-admin.ts
```

### 実装方針

tenant admin に Drive 調査用の read-only view を追加します。

表示対象:

- Drive audit event
- active / pending / revoked share
- active / disabled / expired / pending_sync share link
- external invitation
- approval request
- OpenFGA sync status

本文閲覧導線は置きません。resource name と public ID、resource type、owner、共有対象、role、status、created/updated time だけを表示します。

### API / DB / UI の境界

追加する admin API:

```text
GET /api/v1/admin/tenants/{tenantSlug}/drive/shares
GET /api/v1/admin/tenants/{tenantSlug}/drive/share-links
GET /api/v1/admin/tenants/{tenantSlug}/drive/invitations
GET /api/v1/admin/tenants/{tenantSlug}/drive/audit-events
```

audit API は existing `audit_events` を tenant scope で filter します。`event_type LIKE 'drive.%'` だけではなく、target resource ID / actor / result / date range でも絞れるようにします。

### audit / metrics / security

- admin read は原則 audit に残さなくてもよいが、bulk export を追加する場合は `drive.admin_export.create` を残す
- raw token、password hash、storage key は response に含めない
- tenant mismatch は `404`、role 不足は `403`

### 確認

```bash
go test ./backend/internal/api ./backend/internal/service
npm --prefix frontend run build
```

## Step 6. `pending_sync` repair と OpenFGA tuple drift 検出を追加する

### 対象ファイル

```text
db/queries/drive_shares.sql
db/queries/drive_share_links.sql
backend/internal/service/drive_sync_service.go
backend/internal/api/tenant_admin.go
scripts/smoke-openfga.sh
```

### 実装方針

OpenFGA tuple write/delete が失敗した場合、DB row は `pending_sync` または active なのに tuple がない状態になりえます。Phase 6 では、この状態を検出・修復する service を追加します。

追加する service:

```text
DriveOpenFGASyncService
```

責務:

- `pending_sync` share/link を tenant scope で list する
- DB active row から期待 tuple を組み立てる
- OpenFGA `Check` / `WriteTuples` / `DeleteTuples` で repair する
- dry-run mode で差分だけ返す
- repair 結果を audit に残す

実装上の注意:

- repair は tenant 単位の advisory lock または job lock を取り、同じ tenant で並列実行しない
- expected tuple は DB row から deterministic に生成し、sort して diff しやすくする
- OpenFGA の list API が使えない環境でも動くよう、最小実装は expected tuple に対する `Check` を chunk 実行する
- write/delete は idempotent に扱い、成功済み item の再実行で失敗扱いにしない
- repair 中に policy が changed / revoked になった item は再読込して skip する

### API / DB / UI の境界

tenant admin API:

```text
GET /api/v1/admin/tenants/{tenantSlug}/drive/openfga-sync/drift
POST /api/v1/admin/tenants/{tenantSlug}/drive/openfga-sync/repair
```

`repair` は destructive に近いため CSRF 必須、tenant admin 必須、audit 必須です。

### audit / metrics / security

- dry-run は `drive.openfga_sync.drift_check`
- repair は `drive.openfga_sync.repair`
- metrics は `haohao_drive_openfga_sync_items_total{status}` と `haohao_drive_openfga_sync_duration_seconds`
- repair が失敗した item は access を許可しない

### 確認

```bash
go test ./backend/internal/service
make smoke-openfga
```

## Step 7. deactivated user / SCIM group 同期の整理 job を追加する

### 対象ファイル

```text
backend/internal/service/provisioning_service.go
backend/internal/service/drive_sync_service.go
db/queries/drive_groups.sql
db/queries/drive_shares.sql
```

### 実装方針

SCIM や provider claim により user が deactivated になった場合、`AuthzService` が actor として扱わないため Drive access は止まります。ただし OpenFGA tuple と Drive group membership は残る可能性があります。

Phase 6 では cleanup job を追加します。

cleanup 対象:

- deactivated user の direct user share tuple
- deactivated user の drive group member tuple
- expired invitation
- expired share link の tuple

SCIM group to Drive group sync は、最初は明示 mapping 方式にします。

```text
drive_group_external_mappings
```

mapping が存在する group だけ、SCIM group / provider group と Drive group を同期します。すべての Zitadel group を自動で Drive group にしません。

### audit / metrics / security

- cleanup は `drive.openfga_sync.cleanup`
- SCIM group sync は `drive.group.external_sync`
- tuple delete は失敗時に retry できるよう item 単位で結果を残す

### 確認

```bash
go test ./backend/internal/service
```

## Step 8. restore / complete delete / retention / legal hold の下準備を入れる

### 対象ファイル

```text
db/migrations/*
db/queries/drive_files.sql
db/queries/drive_folders.sql
backend/internal/service/drive_service.go
backend/internal/api/drive_files.go
backend/internal/api/drive_folders.go
```

### 実装方針

Phase 6 では restore / complete delete の全 UI を急いで作る前に、削除禁止条件を DB と service guard に入れます。

追加する主な state:

```text
deleted_parent_folder_id
retention_until
legal_hold_at
legal_hold_by_user_id
legal_hold_reason
purge_block_reason
```

service guard:

- `retention_until > now()` の file/folder は complete delete できない
- `legal_hold_at IS NOT NULL` の file/folder は edit/delete/purge を拒否する
- restore 時は元 parent が存在し、actor が parent に edit 可能か確認する
- 元 parent が存在しない場合は restore target folder を明示指定する

### audit / metrics / security

- restore は `drive.file.restore` / `drive.folder.restore`
- complete delete は `drive.file.complete_delete` / `drive.folder.complete_delete`
- legal hold は `drive.file.legal_hold.set` / `drive.file.legal_hold.release`
- retention / legal hold による拒否は `drive.authz.denied` reason に入れる

### 確認

```bash
go test ./backend/internal/service
make smoke-openfga
```

## Step 9. Owner 移譲、Commenter / Uploader、explicit deny の検討枠を固定する

### 対象ファイル

```text
openfga/drive.fga
openfga/drive.fga.yaml
backend/internal/service/drive_authorization_service.go
backend/internal/service/drive_service.go
frontend/src/components/DriveShareDialog.vue
```

### 実装方針

この Step は authorization model を変えるため、他の Step と同じ PR に混ぜません。まず採用順を固定します。

推奨順:

```text
1. Owner 移譲
2. Commenter
3. Uploader
4. explicit deny
```

Owner 移譲:

- 新 owner tuple を書く
- DB owner metadata を更新する
- 旧 owner tuple を消す
- 途中失敗時に owner が 0 人にならない順序を守る

Commenter / Uploader:

- `openfga/drive.fga` に relation を追加する
- `can_view` / `can_edit` との包含関係を model test で固定する
- UI role select に追加する

explicit deny:

- MVP の継承停止と policy deny では足りないケースが明確になってから追加する
- deny を OpenFGA model に入れる場合は、permissions panel で「なぜ拒否されたか」を説明できるようにする

### 確認

```bash
cd openfga && fga model test --tests drive.fga.yaml
go test ./backend/internal/service
```

## Step 10. external bearer / M2M Drive API の境界を定義する

### 対象ファイル

```text
backend/internal/api/drive_*.go
backend/internal/api/openapi.go
backend/internal/service/authz_service.go
openapi/external.yaml
```

### 実装方針

browser Drive API をそのまま external surface に出しません。external bearer / M2M 用には次を先に固定します。

```text
scope:
  drive:read
  drive:write
  drive:share
  drive:admin

tenant resolution:
  token tenant claim
  または explicit tenant slug header

auth:
  bearer token
  CSRF なし
  Cookie session なし
```

external API では active tenant selector が存在しないため、tenant 解決を request ごとに明示します。tenant mismatch は browser と同じく `404` にします。

### API / DB / UI の境界

- browser OpenAPI と external OpenAPI は分ける
- external route は generated browser SDK から使わない
- M2M client が Drive file body を扱う場合、download URL / signed access / object storage 方針を先に決める

### 確認

```bash
make gen
go test ./backend/internal/api ./backend/internal/service
```

## Step 11. smoke / E2E / final verification を更新する

### 対象ファイル

```text
scripts/smoke-openfga.sh
e2e/drive.spec.ts
Makefile
TUTORIAL_OPENFGA.md
IMPL.md
```

### 実装方針

Phase 6 で増えた挙動は、default E2E に全部入れません。OpenFGA store/model、email sender、外部 user、password link が必要な scenario は、明示 env 付きの smoke / E2E に分けます。

追加する確認:

- external share policy disabled で外部共有が拒否される
- domain allow / deny が期待通りに効く
- approval pending 中は OpenFGA tuple がなく access できない
- approval 後に access できる
- password link は password 未検証なら content access できない
- password link は失敗回数で rate limit される
- `pending_sync` row は repair まで access できない
- repair 後に access できる
- tenant admin は shared state / audit を見られるが body は開けない

### 最終確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
E2E_OPENFGA_ENABLED=true make e2e
```

外部共有の縦通しを CI に入れる場合は、log email sender で invitation URL を取り出せる smoke を追加します。

```bash
RUN_OPENFGA_EXTERNAL_SHARE_SMOKE=1 make smoke-openfga
```

最後に `IMPL.md` へ、Phase 6 で有効化した policy とまだ disabled の機能を追記します。特に external share、password link、repair job は env / tenant policy / background job のどれで有効化されるのかを明記します。

## この Phase でも残すもの

Phase 6 でも、次は別 Phase または別設計として残します。

- tenant admin によるファイル本文の横断閲覧
- 匿名 Editor link
- workspace resource の本格導入
- object storage driver と signed URL の本格導入
- virus scan / content inspection / DLP
- billing / subscription / plan enforcement
- multi-region / HA / DR
- external bearer / M2M Drive API の全 endpoint 実装

これらは Drive/OpenFGA の authorization と密接に関係しますが、Phase 6 の中心である「共有拡張、管理 UI、整合性運用」とは分けて扱います。

次に扱う場合は、`TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` に沿って、高度な管理閲覧、workspace、object storage、DLP、billing、HA/DR、M2M Drive API を個別に設計してから実装します。
