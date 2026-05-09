# Open WebUI + Ruri embedding runbook

このrunbookは、Open WebUI経由で `cl-nagoya/ruri-v3-310m` をembedding runtimeとして検証し、HaoHao Drive semantic search / RAGへ接続するための手順です。

## Scope

対象:

- Open WebUIをローカルにインストールする
- Open WebUIからembedding APIを呼べるか確認する
- `cl-nagoya/ruri-v3-310m` のembedding dimensionを確認する
- HaoHao側のlocal search設定値を整理する
- index rebuildと検索smokeの流れを確認する

対象外:

- Open WebUI自体のproduction hardening
- embedding modelの精度評価設計
- multi-tenant production rollout

## Install Open WebUI

Open WebUI公式docsでは、ローカル導入はDockerが推奨されています。ここではHaoHao開発環境から疎通確認しやすいDocker / Docker Composeの手順だけを扱います。

HaoHaoの `compose.yaml` にはGPU対応Open WebUI serviceを `openwebui` profile付きで定義しています。通常の `make up` では起動しません。

```bash
make openwebui-up
make openwebui-logs
```

Infinity embedding/reranker runtimeも同じprofileに入れています。

```bash
make infinity-build
make infinity-up
make infinity-logs
```

Open WebUIとInfinityをまとめて起動する場合:

```bash
make openwebui-stack-up
```

このcompose serviceは既定でOpen WebUIへ次を渡します。

```dotenv
RAG_EMBEDDING_MODEL=cl-nagoya/ruri-v3-310m
RAG_EMBEDDING_MODEL_TRUST_REMOTE_CODE=True
```

Infinityの既定model:

```dotenv
INFINITY_EMBEDDING_MODEL=cl-nagoya/ruri-v3-310m
INFINITY_RERANKER_MODEL=cl-nagoya/ruri-v3-reranker-310m
```

InfinityはRuri v3向けのcustom imageを使います。baseの `michaelf34/infinity:latest` は `sentence-transformers 3.3.1` を含むことがあり、Ruri v3のModernBERT系modelで起動時に落ちます。`docker/infinity-ruri/Dockerfile` では `sentence-transformers>=3.4.1,<4` / `transformers>=4.48.0,<5` / `huggingface-hub>=0.27.0,<1` にそろえ、互換性問題を起こす古いFlashAttentionを外しています。

ブラウザで開きます。

```text
http://127.0.0.1:3000/
```

ログインを省略する開発用起動:

```bash
WEBUI_AUTH=False docker compose --profile openwebui up -d --force-recreate open-webui
```

LAN内の別端末から `http://192.168.x.x:3000/` で開きたい場合は、host bindも広げます。

```bash
OPEN_WEBUI_HOST_BIND=0.0.0.0 WEBUI_AUTH=False RAG_EMBEDDING_MODEL=cl-nagoya/ruri-v3-310m docker compose --profile openwebui up -d --force-recreate open-webui
```

既に認証ありで初期化したvolumeを使っている場合、Open WebUI側の状態が残ってログイン画面が出続けることがあります。開発用データを消してよい場合だけ、次でvolumeごと作り直します。

```bash
docker compose --profile openwebui down -v
OPEN_WEBUI_HOST_BIND=0.0.0.0 WEBUI_AUTH=False RAG_EMBEDDING_MODEL=cl-nagoya/ruri-v3-310m docker compose --profile openwebui up -d open-webui
```

### Docker

```bash
docker pull ghcr.io/open-webui/open-webui:main

docker run -d \
  --name open-webui \
  --restart unless-stopped \
  -p 3000:8080 \
  -v open-webui:/app/backend/data \
  -e WEBUI_SECRET_KEY="$(openssl rand -hex 32)" \
  ghcr.io/open-webui/open-webui:main
```

確認:

```bash
docker ps --filter name=open-webui
curl -I http://127.0.0.1:3000/
```

ブラウザで開きます。

```text
http://127.0.0.1:3000/
```

補足:

- `-v open-webui:/app/backend/data` はOpen WebUIのデータ永続化用です。消すとユーザー、設定、チャットなども失われます。
- `-p 3000:8080` はhostの3000番をcontainer内8080番へ公開します。
- `WEBUI_SECRET_KEY` を固定しないと、container再作成時にログインセッションが壊れやすくなります。
- Docker Hub imageを使う場合は `openwebui/open-webui:main` でも同等です。

### Docker Compose

`/tmp/open-webui-compose.yml` のような任意の場所に作ります。

```yaml
services:
  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    container_name: open-webui
    restart: unless-stopped
    ports:
      - "3000:8080"
    environment:
      WEBUI_SECRET_KEY: "${WEBUI_SECRET_KEY}"
    volumes:
      - open-webui:/app/backend/data

volumes:
  open-webui:
```

起動:

```bash
export WEBUI_SECRET_KEY="$(openssl rand -hex 32)"
docker compose -f /tmp/open-webui-compose.yml up -d
docker compose -f /tmp/open-webui-compose.yml logs -f open-webui
```

停止:

```bash
docker compose -f /tmp/open-webui-compose.yml down
```

完全削除する場合だけvolumeも削除します。

```bash
docker compose -f /tmp/open-webui-compose.yml down -v
```

### Single User Mode

開発用にログインを省略したい場合だけ使います。

```bash
docker run -d \
  --name open-webui \
  --restart unless-stopped \
  -p 3000:8080 \
  -v open-webui:/app/backend/data \
  -e WEBUI_AUTH=False \
  -e WEBUI_SECRET_KEY="$(openssl rand -hex 32)" \
  ghcr.io/open-webui/open-webui:main
```

注意: Open WebUI公式docsでは、single-user modeとmulti-account modeを後から切り替えないよう警告されています。共有環境やproductionでは使わないでください。

### Update

個人/開発用途で `:main` を追う場合:

```bash
docker rm -f open-webui
docker pull ghcr.io/open-webui/open-webui:main
docker run -d \
  --name open-webui \
  --restart unless-stopped \
  -p 3000:8080 \
  -v open-webui:/app/backend/data \
  -e WEBUI_SECRET_KEY="<same-secret-as-before>" \
  ghcr.io/open-webui/open-webui:main
```

共有環境では `:main` ではなく具体的なversion tagへpinし、更新前にOpen WebUIのrelease notesを確認します。

### Uninstall

containerだけ削除:

```bash
docker rm -f open-webui
```

データも削除:

```bash
docker volume rm open-webui
```

## HaoHao Dimension Support

HaoHaoのlocal search embeddingは可変dimension対応です。

- 既定dimension: `1024`
- 許可dimension: `1` から `2000`
- `local_search_embeddings.embedding`: `vector`
- 検索時は `model` と `dimension` の両方で一致させます。

`cl-nagoya/ruri-v3-310m` は768次元です。HaoHaoで使う場合は tenant settings のDrive local searchで `dimension=768` を設定します。

Infinityへ直接接続する場合は、embedding runtimeに `infinity` を選び、runtime URLを `http://127.0.0.1:7997` にします。

## Open WebUI Endpoint Shape

Open WebUIの接続方式によってbase URLが変わります。HaoHaoの `lmstudio` embedding runtimeは、設定した `runtimeURL` に `/v1/embeddings` を追加して呼びます。

例:

```text
runtimeURL=http://127.0.0.1:3000/ollama
actual request=http://127.0.0.1:3000/ollama/v1/embeddings
```

Open WebUIインスタンスが認証付きの場合は通常 `Authorization: Bearer ...` が必要です。現行HaoHaoの `LocalEmbeddingProvider` は任意header/API key設定を持たないため、HaoHaoから直接認証付きOpen WebUIへ接続するには次のいずれかが必要です。

- loopback限定の認証なしproxyをOpen WebUI手前に置く
- HaoHaoのembedding providerにAPI key/header設定を追加する
- 開発環境でのみ認証なしendpointを使う

## Open WebUI Embedding Smoke

まずOpen WebUI側でRuriがembedding modelとして起動していることを確認します。

```bash
export OPENWEBUI_BASE=http://127.0.0.1:3000/ollama
export OPENWEBUI_API_KEY=<Open WebUI API key if required>
export RURI_MODEL=cl-nagoya/ruri-v3-310m
```

認証あり:

```bash
curl -sS "$OPENWEBUI_BASE/v1/embeddings" \
  -H "Authorization: Bearer $OPENWEBUI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"model\":\"$RURI_MODEL\",\"input\":[\"紅茶\",\"ミルクティー\"]}" \
  | jq '{count: (.data | length), dimensions: [.data[].embedding | length]}'
```

認証なし:

```bash
curl -sS "$OPENWEBUI_BASE/v1/embeddings" \
  -H 'Content-Type: application/json' \
  -d "{\"model\":\"$RURI_MODEL\",\"input\":[\"紅茶\",\"ミルクティー\"]}" \
  | jq '{count: (.data | length), dimensions: [.data[].embedding | length]}'
```

期待値:

```json
{
  "count": 2,
  "dimensions": [768, 768]
}
```

`dimensions` が空、またはHTTP 404/401/500になる場合は、Open WebUI側で次を確認します。

- base URLが正しいか。HaoHao用には `/v1/embeddings` の直前までを `runtimeURL` にする。
- Open WebUIがembedding endpointを公開しているか。
- API keyが必要なendpointにAuthorization headerを付けているか。
- model idがOpen WebUI側の表記と一致しているか。

## Infinity Smoke

Infinityの疎通確認です。このcomposeで使うInfinity `0.0.77` は `/v1/*` ではなく `/models` / `/embeddings` / `/rerank` を公開します。

```bash
export INFINITY_BASE=http://127.0.0.1:7997
export RURI_MODEL=cl-nagoya/ruri-v3-310m

curl -sS "$INFINITY_BASE/models" | jq .

curl -sS "$INFINITY_BASE/embeddings" \
  -H 'Content-Type: application/json' \
  -d "{\"model\":\"$RURI_MODEL\",\"input\":[\"紅茶\",\"ミルクティー\"]}" \
  | jq '{count: (.data | length), dimensions: [.data[].embedding | length]}'
```

reranker:

```bash
curl -sS "$INFINITY_BASE/rerank" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "cl-nagoya/ruri-v3-reranker-310m",
    "query": "紅茶",
    "documents": ["ロイヤルミルクティー", "烏龍茶ミルクティー", "高性能デザインツール"]
  }' \
  | jq .
```

HaoHaoの現行embedding providerはreranker endpointを使いません。rerankerは検索品質評価や将来の後段rerank実装用です。

## HaoHao Settings

tenant settingsのDrive local searchは次の値にします。

```json
{
  "drive": {
    "localSearch": {
      "vectorEnabled": true,
      "embeddingRuntime": "infinity",
      "runtimeURL": "http://127.0.0.1:7997",
      "model": "cl-nagoya/ruri-v3-310m",
      "dimension": 768
    }
  }
}
```

UIでは Tenant Admin -> Drive policy -> Local search / RAG の設定欄から同じ値を入れます。

現行HaoHaoのままこの設定を保存すると、dimension validationで失敗します。その場合はまだRuri切り替え前の状態です。

## Rebuild

modelやdimensionを変えたら、既存embeddingは使い回せません。必ずlocal search indexをrebuildします。

API経由:

```bash
curl -sS -X POST \
  "http://127.0.0.1:18080/api/v1/admin/tenants/acme/drive/search/local-index/rebuilds" \
  -H "Cookie: SESSION_ID=<session>" \
  -H "X-CSRF-Token: <csrf>" \
  -H 'Content-Type: application/json' \
  -d '{}'
```

進捗確認:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"
select status, count(*)
from local_search_index_jobs
group by status
order by status;

select model, dimension, status, count(*)
from local_search_embeddings
group by model, dimension, status
order by model, dimension, status;
\""
```

期待値:

- `local_search_index_jobs.status = completed` が増える
- `local_search_embeddings.model = cl-nagoya/ruri-v3-310m`
- `local_search_embeddings.dimension = 768`
- `local_search_embeddings.status = completed`

## Search Smoke

Drive semantic search:

```bash
curl -sS \
  "http://127.0.0.1:18080/api/v1/drive/search/documents?q=%E7%B4%85%E8%8C%B6&mode=semantic&limit=20" \
  -H "Cookie: SESSION_ID=<session>" \
  | jq '.items[] | {name: (.item.file.originalFilename // .item.name), snippet, matches}'
```

RAG:

```bash
curl -sS -X POST \
  "http://127.0.0.1:18080/api/v1/drive/rag/query" \
  -H "Cookie: SESSION_ID=<session>" \
  -H "X-CSRF-Token: <csrf>" \
  -H 'Content-Type: application/json' \
  -d '{"query":"紅茶に関連する商品を教えて","mode":"semantic","limit":8}' \
  | jq '{answer, citations, matches, blocked}'
```

検証観点:

- `ミルクティー` で見つかる商品が、`紅茶` でもsemantic上位に入るか
- `keyword` ではなく `semantic` / `hybrid` で改善しているか
- citationsが空でないか
- DLP blocked / permission deniedのファイルが混ざっていないか

## Troubleshooting

### 401 Unauthorized

Open WebUIがAuthorization headerを要求しています。curlにはBearer tokenを付けられますが、現行HaoHaoのembedding providerは任意headerを送れません。開発用proxyかprovider拡張が必要です。

### Dimension mismatch

Ruri v3 310mは768次元です。tenant settingsのDrive local searchで `dimension=768` を設定してください。1024次元の既存モデルから切り替えた場合は、対象tenantのlocal search rebuildを実行してembeddingを作り直してください。

### Search finds exact terms but not broader concepts

embedding modelの意味検索性能、chunk内容、OCR/product extractionのindexing範囲を確認します。まずDBで対象語がindexに入っているかを確認します。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"
select d.resource_kind, d.title, left(coalesce(e.source_text, d.body_text, d.snippet, ''), 300) as text
from local_search_embeddings e
join local_search_documents d on d.id = e.document_id
where coalesce(e.source_text, d.body_text, d.snippet, '') ilike '%ミルクティー%'
   or coalesce(e.source_text, d.body_text, d.snippet, '') ilike '%紅茶%'
order by d.resource_kind, d.title
limit 50;
\""
```

### Open WebUI works with curl but HaoHao fails

確認する順序:

1. HaoHaoから見た `runtimeURL` が正しいか。
2. Open WebUI endpointが認証なしで到達できるか。
3. `model` がOpen WebUI側のmodel idと完全一致しているか。
4. responseがOpenAI互換の `{ "data": [{ "embedding": [...] }] }` 形式か。
5. dimensionがHaoHao側のschema/validationと一致しているか。

## Rollback

Ruriへの切り替えで検索品質が悪化した場合:

1. Tenant settingsを元のmodel/runtime/dimensionへ戻す。
2. local search index rebuildを再実行する。
3. `local_search_embeddings` のmodel/dimension別件数を確認する。
4. Drive semantic search / RAG smokeを再実行する。

## References

- Open WebUI Quick Start: https://docs.openwebui.com/getting-started/quick-start/
- Open WebUI OpenAI-compatible endpoints: https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible/
- Open WebUI Updating: https://docs.openwebui.com/getting-started/updating
