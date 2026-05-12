# Drive LM Studio OCR Runbook

HaoHao Drive OCR の `lmstudio` engine は、ローカルの LM Studio Developer server に OpenAI-compatible chat completion request を送り、画像または PDF ページ画像から検索用テキストを生成します。

この engine は text-only chat model では安定しません。OCR 用には画像入力に対応した vision model を LM Studio 側でロードしてください。

## Runtime

LM Studio を起動し、Developer server を `http://127.0.0.1:1234` で有効にします。

HaoHao の tenant settings は local runtime URL を `localhost` または `127.0.0.1` に制限します。別ホストの LM Studio を使う場合は、`scripts/openai-compatible-proxy.mjs` などで localhost proxy を立ててから、その proxy URL を Tenant Admin に設定してください。

## Model Check

まず LM Studio server が応答することを確認します。

```bash
curl -sS -m 5 http://127.0.0.1:1234/v1/models
```

`data[].id` に Tenant Admin へ設定する model id が出ていることを確認します。`qwen/qwen3.5-9b` のような text-only model は RAG generation には使えても、Drive OCR の画像入力には使わないでください。

次に、小さい PNG/JPEG で画像付き chat completion を直接確認します。

```bash
IMAGE_PATH=/path/to/small-ocr-sample.png
MODEL_ID=<loaded-vision-model-id>
IMAGE_DATA="$(base64 -w 0 "$IMAGE_PATH")"

curl -sS -m 180 http://127.0.0.1:1234/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d "{
    \"model\": \"${MODEL_ID}\",
    \"messages\": [
      {\"role\":\"system\",\"content\":\"You extract readable text from images. Return plain text only.\"},
      {\"role\":\"user\",\"content\":[
        {\"type\":\"text\",\"text\":\"Extract readable text from this image.\"},
        {\"type\":\"image_url\",\"image_url\":{\"url\":\"data:image/png;base64,${IMAGE_DATA}\"}}
      ]}
    ],
    \"temperature\": 0,
    \"max_tokens\": 512,
    \"stream\": false
  }"
```

この request が timeout せず、2xx response を返す状態にしてから HaoHao の OCR job を再実行します。LM Studio 側で 400 / 500 が返る場合は、model が vision input に対応していない、model id が違う、または LM Studio server が model を正しくロードできていない可能性が高いです。

## Tenant Policy

Tenant Admin の Drive Policy で次を設定します。

- OCR enabled: on
- OCR engine: `LM Studio`
- LM Studio base URL: `http://127.0.0.1:1234`
- LM Studio model: `/v1/models` に出ている vision model id
- OCR timeout seconds per page: まず `180`、重い model では最大 `300`
- OCR max pages: 検証時は `1` から開始

`OCR timeout seconds per page` は 1 から 300 秒の範囲で、LM Studio OCR の HTTP timeout に直接使われます。PDF の場合も各ページ画像ごとに同じ timeout が使われます。

## Smoke

最初は小さい PNG/JPEG 1 枚を Drive にアップロードし、OCR job を作成します。成功したら PDF や複数ページへ広げます。

結果は DB で確認できます。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select r.id, r.public_id, r.status, r.error_message, r.page_count, r.processed_page_count, r.created_at, r.completed_at from drive_ocr_runs r join file_objects f on f.id=r.file_object_id where f.public_id='<filePublicId>' order by r.created_at desc limit 5;\""
```

ページ単位の保存結果を確認します。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select p.page_number, length(p.raw_text) as raw_text_len, left(p.raw_text, 200) as raw_text_preview from drive_ocr_pages p join drive_ocr_runs r on r.id=p.ocr_run_id join file_objects f on f.id=r.file_object_id where f.public_id='<filePublicId>' order by r.created_at desc, p.page_number limit 10;\""
```

期待状態は `drive_ocr_runs.status = 'completed'` で、`drive_ocr_pages.raw_text` に文字列が保存されていることです。

## Troubleshooting

### `call LM Studio OCR: Post "http://127.0.0.1:1234/v1/chat/completions": context deadline exceeded`

HaoHao が `OCR timeout seconds per page` 内に LM Studio の response を受け取れなかった状態です。

対応手順:

1. LM Studio を起動し直し、Developer server が `http://127.0.0.1:1234` で有効なことを確認する。
2. `curl -sS -m 5 http://127.0.0.1:1234/v1/models` が成功することを確認する。
3. `/v1/models` に出ている vision model id を Tenant Admin の `LM Studio model` に設定する。
4. 小さい画像付き `/v1/chat/completions` を直接投げ、timeout せず 2xx response が返ることを確認する。
5. `OCR timeout seconds per page` を `180` から `300` の範囲で調整する。
6. `OCR max pages` を `1` にして小さい PNG/JPEG から再実行する。

`/v1/models` は成功するのに画像付き chat completion で止まる場合は、LM Studio の model load、VRAM/RAM、または vision 非対応 model の可能性を先に疑ってください。timeout を伸ばしても、text-only model は OCR 用としては安定しません。

### `LM Studio OCR failed: status 400`

多くの場合、画像入力の request shape を model または runtime が受け付けていません。

- Tenant Admin の model id が `/v1/models` の id と一致しているか確認する。
- LM Studio に vision model がロード済みか確認する。
- 上記の最小画像付き curl を実行し、HaoHao 外でも同じ status になるか確認する。

### Runtime Status が reachable だが OCR が失敗する

Tenant Admin の OCR runtime status は `/v1/models` と設定 model の存在を確認します。これは model が vision input に対応していることまでは保証しません。

画像付き chat completion の最小 curl を通してから OCR job を再実行してください。
