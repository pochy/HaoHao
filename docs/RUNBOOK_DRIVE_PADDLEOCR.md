# Drive PaddleOCR Runbook

HaoHao Drive OCR の `paddleocr` engine は、Go process からローカル Python helper を起動して PaddleOCR Python API を呼びます。Tesseract と同じく optional local runtime であり、外部OCR APIには送信しません。

## Runtime

PaddleOCR 公式ドキュメントでは、推論エンジンを準備してから `paddleocr` Python package を入れる手順になっています。
`paddleocr` package と PaddlePaddle wheel の対応 Python version がずれる場合は、PaddlePaddle 公式 installer が対応する Python version で venv を作ってください。

```bash
python3 -m venv .venv-paddleocr
. .venv-paddleocr/bin/activate
python -m pip install --upgrade pip
python -m pip install paddleocr
```

PaddlePaddle / Transformers などの推論エンジンは、実行環境ごとに公式の installer / compatibility table に従って入れてください。CPU local smoke の最小確認は次です。

```bash
export HAOHAO_DRIVE_PADDLEOCR_PYTHON="$PWD/.venv-paddleocr/bin/python"
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

helper path を標準配置から変える場合だけ次を設定します。

```bash
export HAOHAO_DRIVE_PADDLEOCR_HELPER="$PWD/backend/internal/service/scripts/drive_ocr_paddleocr.py"
```

## Tenant Policy

Tenant admin の Drive policy で次を設定します。

- OCR enabled: on
- OCR engine: `PaddleOCR`
- OCR languages: 日本語資料なら `jpn, eng`
- max pages / timeout: PaddleOCR は初回 model load が重いため、まず小さい値で smoke します

`jpn` は helper 内で PaddleOCR の `lang="japan"` に変換されます。

## Smoke

依存関係だけ確認します。

```bash
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

アプリからは tenant admin の OCR runtime table で `paddleocr` が available になっていることを確認します。その後、PNG / JPEG / PDF を Drive にアップロードして OCR job を作成します。

PaddleOCR 3.x は公式 model をローカル cache に置いて使います。offline 運用では、事前に対象 model を warm up / cache 配置してから backend を起動してください。

## Sources

- https://github.com/PaddlePaddle/PaddleOCR
- https://www.paddleocr.ai/latest/en/version3.x/installation.html
- https://www.paddleocr.ai/latest/en/version3.x/pipeline_usage/OCR.html
- https://www.paddleocr.ai/latest/en/version3.x/inference_engine.html
