# Phase 2: Drive DB / sqlc 実装チュートリアル

## この文書の目的

この文書は、OpenFGA Drive 導入に必要な PostgreSQL schema、index、sqlc query、tenant drive policy を追加する手順書です。

OpenFGA は relation graph を持ちますが、Drive metadata の source of truth ではありません。folder/file/share/link/group/policy/audit に必要な情報は PostgreSQL に保存します。

## 完成条件

- `file_objects` が Drive file を表現できる
- 既存 attachment/import/export file flow は `purpose != 'drive'` として互換維持される
- `drive_folders`、`drive_groups`、`drive_group_members`、`drive_resource_shares`、`drive_share_links` が追加される
- active row 用 partial index、token hash unique index、folder child listing index がある
- folder cycle を service/query で拒否できる
- tenant drive policy の default と override を扱える
- sqlc generate が通る

## Step 1. migration 番号を決める

### 対象ディレクトリ

```text
db/migrations/
```

まず現在の最大 migration 番号を確認します。

```bash
ls db/migrations | sort | tail
```

既存が `0014_*` までなら、OpenFGA Drive schema は `0015_openfga_drive.*.sql` から始めます。

```text
db/migrations/0015_openfga_drive.up.sql
db/migrations/0015_openfga_drive.down.sql
```

## Step 2. `file_objects` を Drive 用に拡張する

### 対象ファイル

```text
db/migrations/0015_openfga_drive.up.sql
```

### 既存 `purpose` と追加 column

既存 `file_objects` はすでに `purpose` を持っています。Drive file 用には `purpose` column を追加し直さず、既存 check constraint に `drive` を追加します。

```sql
ALTER TABLE file_objects
  DROP CONSTRAINT file_objects_purpose_check;

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_purpose_check
  CHECK (purpose IN ('attachment', 'avatar', 'import', 'export', 'drive'));
```

Drive file 用に次の metadata column を追加します。

```sql
ALTER TABLE file_objects
  ADD COLUMN drive_folder_id BIGINT,
  ADD COLUMN locked_at TIMESTAMPTZ,
  ADD COLUMN locked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN lock_reason TEXT,
  ADD COLUMN inheritance_enabled BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN deleted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
```

`drive_folder_id` は `drive_folders` 作成後に FK を張ります。実装時は `drive_folders` table を先に作り、その後 `file_objects` に `drive_folder_id` を追加すると migration が単純です。

```sql
ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_drive_folder_id_fkey
  FOREIGN KEY (drive_folder_id) REFERENCES drive_folders(id) ON DELETE SET NULL;
```

既存 row は現在の `purpose` を維持します。既存 attachment/import/export API は Drive file を返さないよう、query 側で `purpose <> 'drive'` または既存 purpose 条件を明示します。既存 `FileService.Upload` から `purpose='drive'` を作れると Drive API / OpenFGA を迂回できるため、既存 FileService では `drive` purpose を拒否します。

### index

```sql
CREATE INDEX file_objects_drive_children_idx
  ON file_objects (tenant_id, drive_folder_id, original_filename)
  WHERE deleted_at IS NULL AND purpose = 'drive';
```

## Step 3. folder table を追加する

### 対象ファイル

```text
db/migrations/0015_openfga_drive.up.sql
```

### schema

```sql
CREATE TABLE drive_folders (
  id BIGSERIAL PRIMARY KEY,
  public_id UUID NOT NULL DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  parent_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  inheritance_enabled BOOLEAN NOT NULL DEFAULT true,
  deleted_at TIMESTAMPTZ,
  deleted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_folders_public_id_key
  ON drive_folders(public_id);

CREATE UNIQUE INDEX drive_folders_active_name_key
  ON drive_folders(tenant_id, COALESCE(parent_folder_id, 0), lower(name))
  WHERE deleted_at IS NULL;

CREATE INDEX drive_folders_children_idx
  ON drive_folders(tenant_id, parent_folder_id, name)
  WHERE deleted_at IS NULL;
```

PostgreSQL の unique index は `NULL` を重複として扱わないため、root folder 名を tenant 内で重複禁止にするには `COALESCE(parent_folder_id, 0)` を使います。

## Step 4. group table を追加する

### schema

```sql
CREATE TABLE drive_groups (
  id BIGSERIAL PRIMARY KEY,
  public_id UUID NOT NULL DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_groups_public_id_key
  ON drive_groups(public_id);

CREATE UNIQUE INDEX drive_groups_active_name_key
  ON drive_groups(tenant_id, lower(name))
  WHERE deleted_at IS NULL;

CREATE TABLE drive_group_members (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES drive_groups(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  added_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_group_members_active_key
  ON drive_group_members(group_id, user_id)
  WHERE deleted_at IS NULL;
```

Drive group は HaoHao app-managed group です。Zitadel group claim から自動同期しません。

`drive_group_members` は soft delete 後に同じ user を再追加できる必要があるため、`(group_id, user_id)` を primary key にしません。`id` primary key と active row partial unique index を使います。

## Step 5. share table を追加する

### schema

```sql
CREATE TABLE drive_resource_shares (
  id BIGSERIAL PRIMARY KEY,
  public_id UUID NOT NULL DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
  resource_id BIGINT NOT NULL,
  subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'group')),
  subject_id BIGINT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
  status TEXT NOT NULL CHECK (status IN ('active', 'revoked', 'pending_sync')),
  created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  revoked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_resource_shares_public_id_key
  ON drive_resource_shares(public_id);

CREATE UNIQUE INDEX drive_resource_shares_active_key
  ON drive_resource_shares(tenant_id, resource_type, resource_id, subject_type, subject_id)
  WHERE status = 'active';

CREATE INDEX drive_resource_shares_resource_idx
  ON drive_resource_shares(tenant_id, resource_type, resource_id);

CREATE INDEX drive_resource_shares_subject_idx
  ON drive_resource_shares(tenant_id, subject_type, subject_id)
  WHERE status = 'active';
```

`pending_sync` の share row は UI 上で表示してもよいですが、access 許可には使いません。実際の許可は OpenFGA tuple が source です。

## Step 6. share link table を追加する

### schema

```sql
CREATE TABLE drive_share_links (
  id BIGSERIAL PRIMARY KEY,
  public_id UUID NOT NULL DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
  resource_id BIGINT NOT NULL,
  token_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role = 'viewer'),
  can_download BOOLEAN NOT NULL DEFAULT true,
  expires_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'expired', 'pending_sync')),
  created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  disabled_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  disabled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_share_links_public_id_key
  ON drive_share_links(public_id);

CREATE UNIQUE INDEX drive_share_links_token_hash_key
  ON drive_share_links(token_hash);

CREATE INDEX drive_share_links_resource_idx
  ON drive_share_links(tenant_id, resource_type, resource_id)
  WHERE status = 'active';

CREATE INDEX drive_share_links_active_lookup_idx
  ON drive_share_links(token_hash, expires_at)
  WHERE status = 'active';
```

raw token は保存しません。作成 API の response で一度だけ返します。audit / log / metrics にも raw token を出しません。

## Step 7. tenant drive policy を追加する

### 対象

既存 `tenant_settings.features` を使います。

### default policy

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

tenant settings service には typed accessor を追加します。

```text
GetDrivePolicy(ctx, tenantID)
UpdateDrivePolicy(ctx, tenantID, input)
```

初期導入では `editorCanReshare=false` と `editorCanDelete=false` を default にし、OpenFGA model 上も `can_share` / `can_delete` は Owner のみです。

## Step 8. sqlc query を追加する

### 対象ファイル

```text
db/queries/drive_folders.sql
db/queries/drive_files.sql
db/queries/drive_groups.sql
db/queries/drive_shares.sql
db/queries/drive_share_links.sql
```

### 必要な query

Folder:

- create root/child folder
- get folder by public ID for tenant
- list child folders
- rename folder
- move folder
- soft delete folder
- check folder cycle candidate

File:

- create drive file object
- get drive file by public ID for tenant
- list child drive files
- rename file
- move file
- overwrite metadata
- soft delete drive file
- list/search candidate files

Group:

- create/update/delete group
- list groups
- get group by public ID for tenant
- add/remove member
- list members

Share:

- create share row
- mark share pending sync
- revoke share
- list direct permissions by resource
- list active shares by subject

Share link:

- create share link
- lookup active link by token hash
- update link
- disable link
- mark link pending sync
- list resource links

## Step 9. folder cycle を拒否する

folder move では、移動先が自分自身または自分の descendant なら拒否します。

sqlc query 例:

```sql
-- name: IsFolderDescendant :one
WITH RECURSIVE descendants AS (
  SELECT id
  FROM drive_folders
  WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
  UNION ALL
  SELECT child.id
  FROM drive_folders child
  JOIN descendants d ON child.parent_folder_id = d.id
  WHERE child.tenant_id = $2 AND child.deleted_at IS NULL
)
SELECT EXISTS (
  SELECT 1 FROM descendants WHERE id = $3
);
```

`$1` は移動元 folder ID、`$3` は移動先 parent folder ID です。

## Step 10. schema と生成物を更新する

```bash
make db-up
make db-schema
make sqlc
```

最終的には `make gen` に含めて通します。

## Phase 2 の完了確認

```bash
make db-up
make db-down
make db-up
make db-schema
make sqlc
go test ./backend/...
```

確認観点:

- migration up/down が通る
- `db/schema.sql` が更新される
- sqlc generated code が更新される
- 既存 file attachment query が Drive file を返さない
- 既存 FileService が `purpose='drive'` upload を拒否する
- `drive_share_links.token_hash` が unique で lookup できる
- folder children list が tenant / parent / deleted で絞れる
