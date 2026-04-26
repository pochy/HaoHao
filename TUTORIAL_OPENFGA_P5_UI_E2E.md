# Phase 5: Drive UI / E2E 実装チュートリアル

## この文書の目的

この文書は、OpenFGA Drive API を Vue app に接続し、Drive browser、share dialog、permissions panel、group/share link UI、tenant admin 導線、E2E を追加する手順書です。

UI は作業用アプリとして実装します。landing page や説明用 hero は作りません。

## 完成条件

- Drive 画面で folder/file を一覧できる
- folder 作成、file upload/download/delete ができる
- file rename / move / overwrite ができる
- search 結果に閲覧可能 item だけが出る
- owner が user share / group share を作成、解除できる
- share link を作成、無効化できる
- `can_download=false` link では download button が出ない
- permissions panel が direct / inherited を分けて表示する
- 権限不足操作は UI 上で不可になり、API も拒否する
- tenant admin 画面には Drive policy と audit への入口だけがあり、本文閲覧導線がない
- `npm --prefix frontend run build` と `make e2e` が通る

## Step 1. generated SDK wrapper を追加する

### 対象ファイル

```text
frontend/src/api/drive.ts
```

### 方針

Vue component から generated SDK を直接呼びません。既存 `frontend/src/api/client.ts` の Cookie / CSRF 対応 wrapper を使い、Drive 用の薄い API wrapper を作ります。

wrapper の責務:

- Drive endpoint 呼び出しをまとめる
- multipart upload / binary download を扱う
- API error を UI で扱いやすい shape にする
- raw share link token を create response 直後だけ扱う

## Step 2. Pinia store を追加する

### 対象ファイル

```text
frontend/src/stores/drive.ts
```

### state

```text
currentFolder
children
viewableItems
searchResults
selectedItem
permissions
groups
shareLinks
loading
error
lastRawShareLink
```

### action

```text
loadRoot()
loadFolder(folderPublicId)
createFolder(input)
uploadFile(input)
downloadFile(file)
renameFile(file, name)
moveFile(file, targetFolder)
overwriteFile(file, blob)
deleteFile(file)
search(input)
loadPermissions(resource)
createUserShare(resource, input)
createGroupShare(resource, input)
revokeShare(resource, share)
createShareLink(resource, input)
disableShareLink(link)
loadGroups()
createGroup(input)
addGroupMember(group, user)
removeGroupMember(group, user)
```

store では「UI 表示可否」だけを判断します。最終的な許可/拒否は必ず API に任せます。

## Step 3. route と navigation を追加する

### 対象ファイル

```text
frontend/src/router/*
frontend/src/App.vue
frontend/src/components/*
```

### route

```text
/drive
/drive/folders/:folderPublicId
/drive/search
```

既存 navigation に Drive 入口を追加します。tenant selector とは別にし、active tenant が未選択の場合は Drive 画面で操作できない状態を表示します。

## Step 4. Drive browser view を作る

### 対象ファイル

```text
frontend/src/views/DriveView.vue
frontend/src/components/DriveToolbar.vue
frontend/src/components/DriveItemTable.vue
frontend/src/components/DriveBreadcrumbs.vue
```

### UI 方針

- 初期表示は Drive browser にする
- table/list 中心にして、ファイル名、種別、所有者、更新日時、共有状態を見せる
- row action は icon button とし、tooltip を付ける
- loading / empty / forbidden / error を明確に分ける
- 権限不足の action は disabled または非表示にする
- table の高さや action column が hover でずれないよう固定幅を持たせる

Drive は SaaS の作業画面なので、装飾的な card-heavy layout や hero は使いません。

## Step 5. file / folder 操作 UI を追加する

### 対象 component

```text
frontend/src/components/DriveCreateFolderDialog.vue
frontend/src/components/DriveUploadDialog.vue
frontend/src/components/DriveRenameDialog.vue
frontend/src/components/DriveMoveDialog.vue
frontend/src/components/DriveOverwriteDialog.vue
frontend/src/components/DriveDeleteDialog.vue
```

### 操作

- folder create
- folder rename
- file upload
- file download
- file rename
- file move
- file overwrite
- file/folder soft delete

delete は確認 dialog を挟みます。folder delete は配下への影響が大きいため warning を表示します。

locked file は UI 上でも edit/delete/share の action を disabled にします。ただし API 側でも必ず拒否します。

## Step 6. share dialog と permissions panel を作る

### 対象 component

```text
frontend/src/components/DriveShareDialog.vue
frontend/src/components/DrivePermissionsPanel.vue
frontend/src/components/DriveShareSubjectPicker.vue
frontend/src/components/DriveRoleSelect.vue
```

### share dialog

初期導入で扱う subject:

- same tenant user
- drive group
- share link

role:

- Viewer
- Editor
- Owner は初期 UI では owner transfer と混同しやすいため出さない

Owner transfer は次フェーズです。

### permissions panel

表示項目:

- owner
- direct user shares
- direct group shares
- inherited permissions
- share links
- inheritance enabled/disabled
- download allowed
- locked state

direct と inherited は必ず分けて表示します。

## Step 7. share link UI を追加する

### 対象 component

```text
frontend/src/components/DriveShareLinkPanel.vue
frontend/src/components/DriveShareLinkCreateDialog.vue
```

### create fields

```text
expiresAt
canDownload
```

tenant policy により次を UI で反映します。

- link sharing disabled
- anonymous link disabled
- expires required
- max TTL
- share link download disabled

raw token は作成直後だけ表示します。再表示はできない前提にします。log や audit にも出しません。

`can_download=false` の link では public content download を案内しません。管理者向けに、download 禁止は操作上の制限でありスクリーンショット等を完全には防止できないことを表示します。

## Step 8. group management UI を追加する

### 対象 component/view

```text
frontend/src/views/DriveGroupsView.vue
frontend/src/components/DriveGroupList.vue
frontend/src/components/DriveGroupMemberList.vue
frontend/src/components/DriveGroupDialog.vue
```

### 方針

Drive group は HaoHao app-managed group です。Zitadel group claim とは同期しません。

UI には次を表示します。

- group name
- description
- member count
- members
- group share usage count

member add/remove は audit 対象です。

## Step 9. search UI を追加する

### 対象

```text
frontend/src/views/DriveSearchView.vue
frontend/src/components/DriveSearchFilters.vue
frontend/src/components/DriveSearchResults.vue
```

### filter

```text
keyword
contentType
owner
updatedAfter
updatedBefore
sharedState
```

API は必ず OpenFGA `can_view` filter を適用します。UI 側では「検索しても見えないものは存在しない」扱いにします。

## Step 10. tenant admin UI に policy と audit 入口を追加する

### 対象

```text
frontend/src/views/*
frontend/src/components/*
```

既存 tenant admin 画面に Drive policy section を追加します。

扱う policy:

- link sharing enabled
- anonymous links enabled
- link expires required
- max link TTL
- viewer download enabled
- share link download enabled
- editor can reshare
- editor can delete
- external user sharing enabled

tenant admin 画面には Drive file body を開く導線を置きません。tenant admin は設定、共有状態、audit を扱えますが、明示共有なしに本文閲覧できません。

## Step 11. UI E2E を追加する

### 対象

既存 E2E 構成に合わせて追加します。

```text
scripts/e2e-single-binary.sh
```

Playwright を使っている場合は Drive 用 spec を追加します。

### scenario

```text
1. owner user で login する
2. Drive 画面へ移動する
3. folder を作成する
4. file を upload する
5. file を rename / move / overwrite する
6. viewer user に share する
7. viewer user で login し、download できることを確認する
8. viewer user が delete できないことを確認する
9. owner user が share link を作成する
10. public metadata が見えることを確認する
11. can_download=false link で download button が出ないことを確認する
12. share link を disabled にし、public access が拒否されることを確認する
```

## Step 12. build と manual 確認

```bash
make gen
npm --prefix frontend run build
make binary
make e2e
```

manual 確認:

```bash
OPENFGA_ENABLED=true make backend-dev
npm --prefix frontend run dev
```

確認項目:

- Drive 画面が active tenant に従って切り替わる
- direct share と inherited share が区別される
- 権限不足操作が UI と API の両方で拒否される
- tenant admin が明示共有なしに file content を開けない
- raw share link token が作成直後以外に再表示されない

