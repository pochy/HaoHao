# HaoHao OpenFGA 導入計画

## 1. 目的

HaoHao に Google Drive / OneDrive / Dropbox のようなファイル共有サービスを追加するため、OpenFGA をリソース単位の認可基盤として導入する。

既存の `AuthzService` は引き続き以下を担当する。

- ログイン済みユーザーの解決
- global role / tenant role の解決
- active tenant の解決
- tenant 境界の一次チェック

OpenFGA は以下に限定して担当する。

- Drive file / folder の Owner / Editor / Viewer 判定
- folder から child folder / file への権限継承
- user / group 共有
- share link 経由の viewer 判定
- resource operation ごとの `can_view`, `can_edit`, `can_delete`, `can_share`, `can_download`

OpenFGA 障害時は fail-closed とする。権限判定に失敗した場合、閲覧・編集・削除・共有・download は許可しない。

API response は次の方針にする。

- OpenFGA 障害、未設定、timeout は `503 Service Unavailable`
- OpenFGA check が明示的に denied を返した場合は `403 Forbidden`
- tenant mismatch や存在を隠すべき resource は既存方針どおり `404 Not Found`

### 1.1 既存システムの現在地

現在の HaoHao は、OpenAPI 3.1 優先の Go/Huma/Gin backend、Vue/Vite frontend、PostgreSQL/sqlc、Redis session を中心に構成されている。

既に実装済みの基盤:

- local password login と Zitadel OIDC browser login
- Redis backed Cookie session / CSRF / OIDC state / delegation state
- `AuthzService` による local user、global role、tenant role、active tenant 解決
- `tenant_memberships` と `tenant_role_overrides` による tenant-aware auth context
- Zitadel bearer token verification
- external bearer API surface
- M2M bearer middleware
- SCIM user provisioning と provisioning reconcile
- tenant admin API/UI
- audit events
- tenant settings / entitlements
- existing `FileService` による tenant-aware file upload/download/soft delete/lifecycle purge
- OpenAPI full/browser/external split

OpenFGA 導入は、この既存認証・tenant・audit・file storage 基盤を置き換えない。Drive domain の resource-level authorization だけを追加する。

### 1.2 担当境界

Zitadel の担当:

- user authentication
- OIDC browser login
- bearer token issuer
- subject / email / profile claim の提供
- provider group / role claim の提供
- SCIM などを通じた identity lifecycle の入口
- machine-to-machine token issuer

Zitadel が担当しないもの:

- Drive file / folder の Owner / Editor / Viewer 判定
- folder inheritance
- user / group / share link の resource sharing
- share link token 発行と検証
- tenant policy 判定
- locked / deleted / retention などの resource state 判定
- audit event の保存

`AuthzService` の担当:

- Zitadel subject または local login から local `users` row を解決する
- provider groups から global role を同期する
- `tenant:<slug>:<role>` claim から `tenant_memberships(source='provider_claim')` を同期する
- SCIM / local override 由来の tenant membership と合わせて active tenant 候補を解決する
- request context に `AuthContext` と `ActiveTenant` を載せる

OpenFGA の担当:

- DB で tenant/resource state を確認した後の relationship check
- `user:<public_id>` / `group:<public_id>#member` / `share_link:<public_id>` を subject とした relation graph
- `folder:<public_id>` / `file:<public_id>` の parent, owner, editor, viewer, can_* 判定
- `Check`, `BatchCheck`, `ListObjects`, `WriteTuples`, `DeleteTuples`

PostgreSQL の担当:

- local user / identity / tenant / tenant membership の source of truth
- Drive folder / file metadata の source of truth
- group / member / share / share link metadata の source of truth
- share link token hash と status の source of truth
- tenant policy、locked/deleted 状態、audit log の source of truth
- OpenFGA に渡す object ID の元になる `public_id` の source of truth

`DriveAuthorizationService` の担当:

- backend service 層に OpenFGA access を閉じ込める
- OpenFGA object ID / user ID / relation name の組み立てを一元化する
- timeout、token、fail-closed、metrics、trace、error mapping を一元化する
- API handler から OpenFGA SDK を直接呼ばせない

### 1.3 Browser request flow

Zitadel browser login 後の Drive API は次の流れに固定する。

```text
1. Browser が Zitadel で login する
2. HaoHao callback が Zitadel code を token exchange する
3. HaoHao が Zitadel subject を local user identity に紐付ける
4. AuthzService が global role / tenant membership / active tenant を解決する
5. HaoHao が Redis session と CSRF token を発行する
6. Browser は Cookie session + CSRF で /api/v1/drive/... を呼ぶ
7. API は requireActiveTenantRole で session と active tenant を確認する
8. DriveService が DB で resource tenant / deleted / locked / policy を確認する
9. DriveAuthorizationService が OpenFGA Check / BatchCheck / ListObjects を呼ぶ
10. 許可された場合だけ resource operation を実行し audit を記録する
```

Zitadel token は Drive API のたびに OpenFGA へ渡さない。OpenFGA へ渡す user は local `users.public_id` に正規化した `user:<public_id>` とする。

### 1.4 Identity と object ID の対応

Zitadel subject と OpenFGA user object は直接一致させない。

```text
Zitadel subject
  -> user_identities(provider='zitadel', subject)
  -> users.id / users.public_id
  -> OpenFGA user:<users.public_id>
```

理由:

- local password login と Zitadel login を同じ resource permission model で扱える
- IdP 移行時にも Drive permission tuple を維持しやすい
- OpenFGA tuple に provider-specific subject を漏らさない
- tenant audit と既存 user public ID 表示に揃えられる

Drive group は初期導入では HaoHao app-managed group とする。

```text
drive_groups.public_id
  -> OpenFGA group:<drive_groups.public_id>
drive_group_members(user_id)
  -> OpenFGA group:<drive_groups.public_id>#member@user:<users.public_id>
```

Zitadel group claim は引き続き tenant role / global role 同期に使う。Drive group member へ自動同期しない。将来、Zitadel group と Drive group を同期する場合は、別の provisioning mapping と conflict policy を追加する。

### 1.5 Tenant と resource 境界

tenant 境界は OpenFGA model だけに任せない。

全 Drive API は OpenFGA check の前に DB で以下を確認する。

- request の active tenant が存在する
- actor user が active tenant に所属している
- resource row の `tenant_id` が active tenant と一致する
- share / group / share link の `tenant_id` が active tenant と一致する
- tenant が inactive ではない

tenant mismatch は `404 Not Found` を返して resource の存在を隠す。OpenFGA object ID には tenant ID を埋め込まないため、この DB check は必須とする。

### 1.6 External bearer / M2M / SCIM の扱い

初期導入の Drive API は browser surface に追加する。つまり Cookie session + CSRF + active tenant を前提にする。

初期導入で扱う:

- browser user の Drive 操作
- local password login user
- Zitadel OIDC login user
- tenant admin UI からの Drive policy 管理

初期導入では扱わない:

- external bearer からの Drive business API
- M2M client による Drive file/folder 操作
- SCIM group を Drive group へ直接同期すること
- downstream delegated auth token を使った Drive resource 操作

SCIM は local user lifecycle と tenant membership の同期には使い続ける。SCIM で user が deactivated になった場合、`AuthzService` が認証済み actor として扱わないため、OpenFGA tuple が残っていても Drive access はできない。tuple cleanup は後続の maintenance job として別途設計する。

### 1.7 Zitadel と OpenFGA の運用上の分離

Zitadel と OpenFGA はどちらも認可に関わるが、運用上は別コンポーネントとして扱う。

- Zitadel 障害は login / token verification / readiness に影響する。
- OpenFGA 障害は Drive resource authorization に影響する。
- 既存 session が有効でも、OpenFGA が落ちていれば Drive protected operation は fail-closed にする。
- OpenFGA が正常でも、Zitadel / local user / session / active tenant が無効なら Drive operation は開始しない。
- `/readyz` は設定に応じて Zitadel discovery と OpenFGA API の両方を確認する。

## 2. 初期スコープ

初期導入で実装する。

- 自前 Docker 運用の OpenFGA
- repo 管理の OpenFGA authorization model
- `Owner / Editor / Viewer`
- folder / file の作成、一覧、download、soft delete
- file rename / move / overwrite update
- folder から child folder / file への継承
- 継承停止
- user share
- group / group member / group share
- share link 作成、期限、無効化
- share link の download 禁止
- permissions list
- 閲覧可能 item list と権限 filter 済み検索
- Drive UI
- audit log
- smoke test

初期導入では実装しない。

- 外部ユーザーへの直接共有
- 未登録メールアドレス招待
- パスワード付き share link
- ドメイン allow / deny
- 管理者承認フロー
- 明示的 deny
- Owner 移譲
- 完全削除
- リーガルホールド / retention policy UI
- tenant admin によるファイル本文閲覧
- Commenter / Uploader role
- workspace table の新設

`tenant_admin` は共有設定、ポリシー、監査ログを扱えるが、明示的に共有されていないファイル本文は閲覧できない。

初期導入では、DRIVE_OPENFGA_PERMISSIONS_SPEC の `Organization / Tenant / Workspace` のうち既存 project の `tenant` を organization/workspace 境界として扱う。独立した `workspace` resource は、複数 workspace が必要になった段階で追加する。

## 3. OpenFGA 運用

### 3.1 Docker compose

`compose.yaml` に OpenFGA 用の専用 Postgres として以下を追加する。

- `openfga-postgres`
- `openfga-migrate`
- `openfga`

ローカル開発では OpenFGA HTTP API を `127.0.0.1:8088` に expose する。

production 前提の設定では playground を無効にし、API token などの認証を有効化できる構成にする。

### 3.2 Backend config

`backend/internal/config` に以下を追加する。

```text
OPENFGA_ENABLED=false
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=
OPENFGA_AUTHORIZATION_MODEL_ID=
OPENFGA_API_TOKEN=
OPENFGA_TIMEOUT=2s
OPENFGA_FAIL_CLOSED=true
```

`OPENFGA_ENABLED=true` の場合、`OPENFGA_STORE_ID` と `OPENFGA_AUTHORIZATION_MODEL_ID` は必須とする。

### 3.3 Model 管理

OpenFGA model は repo 内に固定する。

推奨ファイル:

```text
openfga/drive.fga
scripts/openfga-bootstrap.sh
```

runtime 起動時には store 作成や model 更新を行わない。`scripts/openfga-bootstrap.sh` で store 作成と model 登録を行い、出力された store ID / model ID を `.env` に設定する。

## 4. OpenFGA Model

初期 model は次の方針にする。

```fga
model
  schema 1.1

type user

type group
  relations
    define member: [user]

type share_link

type folder
  relations
    define parent: [folder]
    define owner: [user, group#member] or owner from parent
    define editor: [user, group#member] or owner or editor from parent
    define viewer: [user, group#member, share_link with not_expired] or editor or viewer from parent
    define can_view: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner

type file
  relations
    define parent: [folder]
    define owner: [user, group#member] or owner from parent
    define editor: [user, group#member] or owner or editor from parent
    define viewer: [user, group#member, share_link with not_expired] or editor or viewer from parent
    define can_view: viewer
    define can_download: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner

condition not_expired(current_time: timestamp, expires_at: timestamp) {
  current_time < expires_at
}
```

実装時の object ID は DB の `public_id` を使う。

```text
user:<users.public_id>
group:<drive_groups.public_id>
folder:<drive_folders.public_id>
file:<file_objects.public_id>
share_link:<drive_share_links.public_id>
```

tenant ID は OpenFGA object ID に埋め込まない。tenant 分離は DB の `tenant_id` と active tenant check で OpenFGA check の前に保証する。

## 5. Database

### 5.1 `file_objects` 拡張

既存 file body の source of truth として `file_objects` を残す。

追加 column:

```text
purpose に 'drive' を追加
drive_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL
locked_at TIMESTAMPTZ
locked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL
lock_reason TEXT
inheritance_enabled BOOLEAN NOT NULL DEFAULT true
deleted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL
```

既存 attachment / avatar / import / export API は `purpose != 'drive'` のまま互換維持する。

Drive API は `purpose = 'drive'` の file だけを扱う。

### 5.2 Folder

`drive_folders` を追加する。

主要 column:

```text
id
public_id
tenant_id
parent_folder_id
name
created_by_user_id
inheritance_enabled
deleted_at
deleted_by_user_id
created_at
updated_at
```

制約:

- `tenant_id` は必須
- active folder 名は同一 tenant / parent 内で重複禁止
- folder 移動時は循環参照を拒否
- root folder は `parent_folder_id IS NULL`

index:

- active row 用の tenant scoped partial index
- folder child listing 用の `(tenant_id, parent_folder_id, name)` index

### 5.3 Group

`drive_groups` と `drive_group_members` を追加する。

`drive_groups`:

```text
id
public_id
tenant_id
name
description
created_by_user_id
deleted_at
created_at
updated_at
```

`drive_group_members`:

```text
group_id
user_id
added_by_user_id
deleted_at
created_at
updated_at
```

active member だけを OpenFGA の `group#member` tuple に同期する。

index:

- active row 用の tenant scoped partial index
- active member lookup 用の `(group_id, user_id)` partial unique index

### 5.4 Shares

`drive_resource_shares` を追加する。

主要 column:

```text
id
public_id
tenant_id
resource_type -- file / folder
resource_id
subject_type -- user / group
subject_id
role -- owner / editor / viewer
status -- active / revoked / pending_sync
created_by_user_id
revoked_by_user_id
revoked_at
created_at
updated_at
```

OpenFGA tuple write に失敗した share は `pending_sync` とし、アクセス許可には使わない。

index:

- active row 用の tenant scoped partial index
- resource permissions list 用の `(tenant_id, resource_type, resource_id)` index
- subject share lookup 用の `(tenant_id, subject_type, subject_id)` index

### 5.5 Share Links

`drive_share_links` を追加する。

主要 column:

```text
id
public_id
tenant_id
resource_type -- file / folder
resource_id
token_hash
role -- viewer only for initial release
can_download
expires_at
status -- active / disabled / expired / pending_sync
created_by_user_id
disabled_by_user_id
disabled_at
created_at
updated_at
```

token raw value は保存しない。API response で作成直後のみ raw token を返す。

index:

- `token_hash` unique index
- active row 用の tenant scoped partial index
- resource link listing 用の `(tenant_id, resource_type, resource_id)` index

### 5.6 Tenant Drive Policy

DRIVE_OPENFGA_PERMISSIONS_SPEC の組織ポリシーは、初期導入では既存 `tenant_settings.features` の `drive` object に保存する。

初期 policy:

```json
{
  "drive": {
    "linkSharingEnabled": true,
    "anonymousLinksEnabled": true,
    "linkExpiresRequired": true,
    "maxLinkTtlHours": 720,
    "viewerDownloadEnabled": true,
    "shareLinkDownloadEnabled": true,
    "editorCanReshare": false,
    "editorCanDelete": false,
    "externalUserSharingEnabled": false
  }
}
```

判定方針:

- `linkSharingEnabled=false` の tenant では share link 作成を拒否する。
- `anonymousLinksEnabled=false` の tenant では public token link 作成を拒否する。
- `linkExpiresRequired=true` の tenant では `expires_at` なし link を拒否する。
- `maxLinkTtlHours` を超える `expires_at` は拒否する。
- `viewerDownloadEnabled=false` の tenant では viewer の content download を拒否する。
- `shareLinkDownloadEnabled=false` の tenant では share link 経由の content download を拒否する。
- `editorCanReshare=false` と `editorCanDelete=false` を初期 default とし、OpenFGA model 上も `can_share` / `can_delete` は Owner のみから開始する。
- `externalUserSharingEnabled=false` を初期 default とし、外部ユーザー直接共有とメール招待は次フェーズまで提供しない。

## 6. Backend Services

### 6.1 `DriveAuthorizationService`

OpenFGA SDK は API 層から直接呼ばない。

`backend/internal/service/drive_authorization_service.go` に以下を閉じ込める。

- `Check`
- `BatchCheck`
- `ListObjects`
- `WriteTuples`
- `DeleteTuples`

主な method:

```text
CanViewFile(ctx, actor, file)
CanDownloadFile(ctx, actor, file)
CanEditFile(ctx, actor, file)
CanDeleteFile(ctx, actor, file)
CanShareFile(ctx, actor, file)
CanViewFolder(ctx, actor, folder)
CanEditFolder(ctx, actor, folder)
CanDeleteFolder(ctx, actor, folder)
CanShareFolder(ctx, actor, folder)
```

OpenFGA disabled の場合は fail-closed にする。ただし unit test 用に fake implementation を注入できる interface にする。

### 6.2 `DriveService`

Drive domain の DB 操作と OpenFGA tuple 操作をまとめる service を追加する。

主な責務:

- root / child folder 作成
- folder rename / move / delete
- drive file upload / download / overwrite / rename / move / delete
- folder children list
- viewable item list
- metadata search with authorization filter
- permissions list
- user share
- group share
- share revoke
- group create / update / delete
- group member add / remove
- share link create / update / disable
- public share link resolve

resource 作成時の順序:

```text
1. DB transaction で resource row を作成する
2. audit を transaction 内で記録する
3. transaction commit
4. OpenFGA owner tuple と parent tuple を write する
5. tuple write に失敗した場合は resource を使用不可状態にし、audit に failed を残す
```

共有作成時の順序:

```text
1. can_share を確認する
2. DB に active share row を作成する
3. audit を記録する
4. OpenFGA tuple を write する
5. tuple write 失敗時は share row を pending_sync に更新する
```

共有解除時の順序:

```text
1. can_share を確認する
2. OpenFGA tuple を delete する
3. DB share row を revoked にする
4. audit を記録する
```

継承停止:

```text
1. can_share を確認する
2. DB の inheritance_enabled=false に更新する
3. OpenFGA parent tuple を delete する
4. audit を記録する
```

DB の `parent_folder_id` は UI と一覧表示のために残す。OpenFGA の parent tuple だけを削除することで権限継承を止める。

### 6.3 権限チェック順

全ての Drive 操作は次の順で判定する。

```text
1. 認証済み user または有効な share link token を確認する
2. active tenant を確認する
3. resource が存在し、request tenant に属していることを DB で確認する
4. resource が deleted ではないことを確認する
5. locked / retention / read-only などの resource state を確認する
6. tenant policy を確認する
7. OpenFGA Check / BatchCheck / ListObjects を呼ぶ
8. denied / failed は必要に応じて audit に記録する
```

locked file は閲覧だけ許可し、edit / overwrite / rename / move / delete / share change は OpenFGA の許可結果より優先して拒否する。

### 6.4 一覧・検索の権限 filter

Drive の一覧・検索は、DB query だけで結果を返さない。

- folder children list は DB で tenant / parent / deleted を絞った後、OpenFGA `BatchCheck(can_view)` で不可視 item を除外する。
- viewable item list は OpenFGA `ListObjects(can_view)` で file / folder object を取得し、DB metadata と join して返す。
- keyword / content type / owner / updated time / shared state 検索は DB で候補を絞った後、OpenFGA `BatchCheck(can_view)` を必ず適用する。
- pagination は DB cursor と OpenFGA filter 後の不足分追加 fetch を組み合わせ、権限がない item を response に含めない。

### 6.5 既存 service wiring への接続

OpenFGA 導入時の backend wiring は既存の構造に合わせる。

追加する service:

```text
OpenFGAClient
DriveAuthorizationService
DriveService
```

`backend/cmd/main/main.go` では、既存 service の後に次の順で組み立てる。

```text
config.Load()
PostgreSQL / Redis
db.Queries
AuditService
AuthzService
TenantSettingsService
FileStorage
FileService
OpenFGAClient
DriveAuthorizationService
DriveService
app.New(...)
```

`DriveService` は既存 `FileService` と `FileStorage` を再利用する。ただし Drive file は `purpose='drive'` と `drive_folder_id` を必須にし、既存 attachment/import/export flow と混ぜない。

`backend/internal/api/register.go` には `DriveService` を `Dependencies` に追加し、browser surface のみで `registerDriveRoutes(api, deps)` を呼ぶ。

初期導入では external surface / M2M surface / SCIM surface には Drive route を登録しない。OpenAPI artifact では `openapi/browser.yaml` に Drive browser API が追加され、`openapi/external.yaml` には追加しない。

### 6.6 Zitadel claim と Drive permission の接続禁止事項

Zitadel claim を Drive file/folder permission として直接解釈しない。

禁止:

- `groups` claim に `drive:folder:*` のような resource permission を入れる
- Zitadel role を OpenFGA tuple の代わりとして使う
- tenant admin role を `folder:*#viewer` 相当として扱う
- Zitadel organization membership だけで Drive file body access を許可する

許可:

- `tenant:<slug>:<role>` claim を既存 `tenant_memberships` に同期する
- `tenant_admin` global role で tenant settings / Drive policy / audit UI へアクセスする
- Drive share UI で選択された local user / drive group を OpenFGA tuple に変換する
- SCIM / provider claim で deactivated になった user の session / auth context を無効化する

## 7. API

`/api/v1/drive/...` を browser API として追加する。

### 7.1 Folder

```text
GET    /api/v1/drive/folders/root
POST   /api/v1/drive/folders
GET    /api/v1/drive/folders/{folderPublicId}
PATCH  /api/v1/drive/folders/{folderPublicId}
DELETE /api/v1/drive/folders/{folderPublicId}
GET    /api/v1/drive/folders/{folderPublicId}/children
PATCH  /api/v1/drive/folders/{folderPublicId}/inheritance
```

### 7.2 File

```text
POST   /api/v1/drive/files
GET    /api/v1/drive/files/{filePublicId}
PATCH  /api/v1/drive/files/{filePublicId}
GET    /api/v1/drive/files/{filePublicId}/content
PUT    /api/v1/drive/files/{filePublicId}/content
DELETE /api/v1/drive/files/{filePublicId}
PATCH  /api/v1/drive/files/{filePublicId}/inheritance
```

`content` は `can_view` と `can_download` の両方を要求する。

`PATCH /api/v1/drive/files/{filePublicId}` は rename / move を扱い、`can_edit` を要求する。

`PUT /api/v1/drive/files/{filePublicId}/content` は file body overwrite を扱い、`can_edit` を要求する。locked file、deleted file、read-only policy 対象 file は拒否する。

### 7.3 List / Search

```text
GET /api/v1/drive/items
GET /api/v1/drive/search
```

`GET /api/v1/drive/items` は user が閲覧可能な folder / file を返す。`scope=owned|shared|all`、`cursor`、`limit` を受け付ける。

`GET /api/v1/drive/search` は `keyword`、`contentType`、`ownerUserPublicId`、`updatedAfter`、`updatedBefore`、`sharedState`、`cursor`、`limit` を受け付ける。すべての検索結果に OpenFGA `can_view` filter を適用する。

### 7.4 Permissions / Shares

```text
GET    /api/v1/drive/files/{filePublicId}/permissions
POST   /api/v1/drive/files/{filePublicId}/shares
DELETE /api/v1/drive/files/{filePublicId}/shares/{sharePublicId}

GET    /api/v1/drive/folders/{folderPublicId}/permissions
POST   /api/v1/drive/folders/{folderPublicId}/shares
DELETE /api/v1/drive/folders/{folderPublicId}/shares/{sharePublicId}
```

permissions response は direct と inherited を分ける。

### 7.5 Groups

```text
GET    /api/v1/drive/groups
POST   /api/v1/drive/groups
GET    /api/v1/drive/groups/{groupPublicId}
PATCH  /api/v1/drive/groups/{groupPublicId}
DELETE /api/v1/drive/groups/{groupPublicId}
POST   /api/v1/drive/groups/{groupPublicId}/members
DELETE /api/v1/drive/groups/{groupPublicId}/members/{userPublicId}
```

### 7.6 Share Links

```text
POST   /api/v1/drive/files/{filePublicId}/share-links
PATCH  /api/v1/drive/share-links/{shareLinkPublicId}
DELETE /api/v1/drive/share-links/{shareLinkPublicId}

POST   /api/v1/drive/folders/{folderPublicId}/share-links
PATCH  /api/v1/drive/share-links/{shareLinkPublicId}
DELETE /api/v1/drive/share-links/{shareLinkPublicId}
```

Public link access:

```text
GET /api/public/drive/share-links/{token}
GET /api/public/drive/share-links/{token}/content
```

public content download は `can_download=true` の link だけ許可する。

## 8. UI

Vue app に Drive 画面を追加する。

主な画面:

- Drive browser
- Folder children list
- Viewable items list
- Search results
- File upload
- File rename / move / overwrite
- File download
- Folder create / rename
- File / folder delete
- Share dialog
- Permissions panel
- Group share
- Group management
- Share link create / disable

UI の方針:

- 初期表示は Drive browser にする。landing page は作らない。
- folder / file row はスキャンしやすい table/list にする。
- 権限不足の操作ボタンは非表示または disabled にし、API 側でも必ず拒否する。
- `can_download=false` の link では download button を出さない。
- 継承停止中の resource は明示する。
- tenant admin 画面には共有ポリシーと audit への入口だけを置く。
- tenant admin 画面に本文閲覧導線は追加しない。
- anonymous link 作成、password なし link 作成、folder delete、download 禁止設定には警告を表示する。
- Owner 移譲と完全削除は初期導入では UI に出さない。
- download 禁止は操作上の制限であり、スクリーンショット等を完全には防止できないことを管理者向けに表示する。

## 9. Audit / Metrics / Readiness

### 9.1 Audit

追加 action:

```text
drive.file.create
drive.file.view
drive.file.download
drive.file.update
drive.file.move
drive.file.delete
drive.folder.create
drive.folder.view
drive.folder.update
drive.folder.delete
drive.share.create
drive.share.revoke
drive.share_link.create
drive.share_link.update
drive.share_link.disable
drive.share_link.access
drive.group.create
drive.group.update
drive.group.delete
drive.group_member.add
drive.group_member.remove
drive.authz.denied
```

audit metadata に入れないもの:

- share link raw token
- storage key
- API token
- OpenFGA API token
- password / secret
- raw idempotency key

### 9.2 Metrics

低 cardinality label だけを使う。

追加候補:

```text
haohao_openfga_requests_total{operation,result}
haohao_openfga_request_duration_seconds{operation,result}
haohao_drive_authz_denied_total{operation,resource_type}
```

`operation` label は次の低 cardinality 値だけを使う。

```text
openfga_check
openfga_write
openfga_delete
openfga_list_objects
```

tenant id、user id、file id、folder id、link token は label に入れない。

### 9.3 Readiness

`OPENFGA_ENABLED=true` の場合、`/readyz` で OpenFGA API に疎通確認する。

OpenFGA が落ちている場合は readiness を fail にする。

## 10. Test Plan

### 10.1 Model test

OpenFGA model で確認する。

- owner は viewer / editor / delete / share を満たす
- editor は viewer / edit を満たす
- viewer は view のみ満たす
- parent folder viewer は child folder / child file viewer になる
- parent folder editor は child folder / child file editor になる
- parent tuple を消すと継承が止まる
- group member は group share 経由で access できる
- expired share link は access できない

### 10.2 Go unit test

fake OpenFGA client を使って確認する。

- OpenFGA error は fail-closed
- locked file は editor でも edit / delete / share 不可
- deleted resource は check 前に拒否される
- tenant mismatch は resource existence を隠す
- `can_download=false` share link は metadata だけ許可される
- share 作成時の tuple write 失敗は `pending_sync`
- share revoke は tuple delete 成功後に DB revoked
- group member add / remove が tuple write / delete を呼ぶ

### 10.3 DB / sqlc test

- folder cycle prevention
- active folder child listing
- active file child listing
- active share partial unique behavior
- share link token hash lookup
- revoked / disabled / expired link exclusion
- tenant drive policy defaults and overrides
- search candidate query is always followed by authorization filter

### 10.4 API smoke

新設 script:

```text
scripts/smoke-openfga.sh
make smoke-openfga
```

確認シナリオ:

```text
1. owner user が folder を作る
2. owner user が file を upload する
3. 別 user は download できない
4. owner が別 user に viewer share する
5. viewer は download できる
6. viewer は delete できない
7. editor share user は file rename / overwrite できる
8. locked file は editor でも overwrite / delete できない
9. owner が group を作り member を追加する
10. group に folder viewer を付与し child file を閲覧できる
11. search 結果には閲覧可能 item だけが出る
12. share link を作成し public metadata を取得できる
13. can_download=false link で content download が拒否される
14. share link disable 後に public access が拒否される
```

### 10.5 Final verification

最終確認コマンド:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make smoke-openfga
make e2e
```

### 10.6 UI E2E

- Drive 画面で folder を作成できる
- Drive 画面で file を upload / download できる
- Drive 画面で file rename / move / overwrite ができる
- Drive 画面の検索結果に閲覧可能 item だけが表示される
- owner が user share を作成、解除できる
- owner が group share を作成、解除できる
- share link を作成、無効化できる
- 権限不足 user には操作不可 UI が表示され、API も拒否する

## 11. 実装順

1. OpenFGA compose / config / model / bootstrap script を追加する。
2. DB migration と sqlc query を追加する。
3. OpenFGA client wrapper と `DriveAuthorizationService` を追加する。
4. `DriveService` の folder / file core flow を実装する。
5. share / group / share link flow を実装する。
6. `/api/v1/drive/...` API と raw upload / download route を追加する。
7. audit / metrics / readiness を接続する。
8. Vue Drive UI を追加する。
9. smoke / unit / E2E を追加する。
10. `make gen` と最終 verification を実行する。

## 12. Assumptions

- OpenFGA store は環境単位で 1 つにする。
- tenant 分離は既存 active tenant 判定と DB の `tenant_id` で OpenFGA check の前に保証する。
- 外部ユーザーへの直接共有、パスワード付き link、ドメイン制限、承認フロー、明示的 deny は次フェーズに回す。
- Editor による再共有・削除は初期では許可しない。`can_share` と `can_delete` は Owner のみから開始する。
- 参考情報は OpenFGA Google Drive modeling、parent-child、conditions、Docker setup、production recommendations を使う。

## 13. DRIVE_OPENFGA_PERMISSIONS_SPEC 対応状況

DRIVE_OPENFGA_PERMISSIONS_SPEC の要件は、初期導入で実装するもの、アプリ層で制御するもの、次フェーズへ送るものに分ける。

初期導入で含める。

- tenant 分離
- Owner / Editor / Viewer
- file / folder の閲覧、download、編集、削除、共有判定
- folder から child folder / file への継承
- 継承停止
- user share
- group / group member / group share
- share revoke
- 有効期限付き share link
- share link 無効化
- share link / viewer download 禁止
- locked file の編集、上書き、削除、共有変更拒否
- 閲覧可能 file / folder 一覧
- 検索結果への権限 filter
- permissions direct / inherited 表示
- 基本 audit と access denied audit
- tenant drive policy の初期セット
- danger operation の UI warning

OpenFGA ではなくアプリ層で制御する。

- deleted / locked / read-only / retention などの resource state
- tenant policy
- share link token hash と有効期限
- download 禁止時の UI / API 制御
- folder cycle prevention
- storage body lifecycle
- audit metadata の秘匿

次フェーズへ送る。

- Commenter / Uploader
- workspace resource
- 外部ユーザー直接共有
- 未登録メールアドレス招待
- パスワード付き link
- 外部ドメイン allow / deny
- 管理者承認フロー
- Owner 移譲
- explicit deny
- restore
- complete delete
- legal hold / retention policy UI
- MFA / IP / device policy
- 監査管理者 / セキュリティ管理者 role の分離

## 14. 受け入れ条件

- OpenFGA が compose で起動し、bootstrap で store / model を登録できる。
- `OPENFGA_ENABLED=true` で backend が OpenFGA に接続して起動する。
- Drive file / folder access は OpenFGA check を通過しない限り許可されない。
- tenant admin は明示共有なしにファイル本文を閲覧できない。
- folder inheritance と inheritance stop が機能する。
- user share / group share / share link が機能する。
- share link raw token は DB / audit / log に保存されない。
- OpenFGA 障害時に protected operation が fail-closed になる。
- `make smoke-openfga` が通る。
- 既存 attachment/import/export file flow が壊れない。
