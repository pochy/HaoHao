# Drive PaddleOCR Runbook

HaoHao Drive OCR の `paddleocr` engine は、Go process からローカル Python helper を起動して PaddleOCR Python API を呼びます。Tesseract と同じく optional local runtime であり、外部OCR APIには送信しません。

## Runtime

PaddleOCR 公式ドキュメントでは、推論エンジンを準備してから `paddleocr` Python package を入れる手順になっています。
`paddleocr` package と PaddlePaddle wheel の対応 Python version がずれる場合は、PaddlePaddle 公式 installer が対応する Python version で venv を作ってください。
Python 3.14 は PaddlePaddle wheel が未提供のことがあるため、local smoke では Python 3.12 または 3.13 の venv を推奨します。

```bash
python3.12 -m venv .venv-paddleocr
. .venv-paddleocr/bin/activate
python -m pip install --upgrade pip
python -m pip install paddleocr
python -m pip install paddlepaddle==3.2.0 -i https://www.paddlepaddle.org.cn/packages/stable/cpu/
```

PaddlePaddle / Transformers などの推論エンジンは、実行環境ごとに公式の installer / compatibility table に従って入れてください。CPU local smoke の最小確認は次です。

```bash
export HAOHAO_DRIVE_PADDLEOCR_PYTHON="$PWD/.venv-paddleocr/bin/python"
export HAOHAO_DRIVE_PADDLEOCR_DEVICE=cpu
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

GPU で実行する場合は GPU 対応の PaddlePaddle wheel を入れ、device を `gpu` または `gpu:0` にします。CUDA 12.6 の例:

```bash
python -m pip uninstall -y paddlepaddle
python -m pip install paddlepaddle-gpu==3.2.0 -i https://www.paddlepaddle.org.cn/packages/stable/cu126/

export HAOHAO_DRIVE_PADDLEOCR_DEVICE=gpu:0
python -c "import paddle; print(paddle.__version__); print(paddle.device.is_compiled_with_cuda()); paddle.set_device('gpu:0'); print(paddle.get_device())"
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

## Troubleshooting

### `PaddleOCR engine is selected, but the PaddleOCR Python runtime dependencies are not available`

このエラーは tenant policy で `paddleocr` engine は選ばれているが、backend process から PaddleOCR helper の `--check` が成功していない状態です。backend は次を実行して dependency status を判定します。

```bash
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

まずローカル shell で同じ確認を通します。

```bash
python3 -m venv .venv-paddleocr
. .venv-paddleocr/bin/activate
python -m pip install --upgrade pip
python -m pip install paddleocr
python -m pip install paddlepaddle==3.2.0 -i https://www.paddlepaddle.org.cn/packages/stable/cpu/

export HAOHAO_DRIVE_PADDLEOCR_PYTHON="$PWD/.venv-paddleocr/bin/python"
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

成功時は次のような version 行が出ます。

```text
paddleocr 3.x.x / paddle 3.x.x
```

`--check` が成功したら、backend を起動する shell か `.env` に同じ Python path を設定して backend を再起動します。

```dotenv
HAOHAO_DRIVE_PADDLEOCR_PYTHON=/absolute/path/to/HaoHao/.venv-paddleocr/bin/python
HAOHAO_DRIVE_PADDLEOCR_HELPER=/absolute/path/to/HaoHao/backend/internal/service/scripts/drive_ocr_paddleocr.py
HAOHAO_DRIVE_PADDLEOCR_DEVICE=cpu
```

`HAOHAO_DRIVE_PADDLEOCR_HELPER` は標準配置なら省略できます。`HAOHAO_DRIVE_PADDLEOCR_DEVICE` は未指定なら `cpu` です。GPU を使う場合は `gpu` または `gpu:0` を指定します。`make backend-dev` は `.env` を読み込むため、`.env` を更新した場合は backend process を再起動してください。

よくある原因:

- `paddleocr` は入っているが `paddlepaddle` package が入っていない。
- Python 3.14 で venv を作っている。PaddlePaddle の pip wheel は Python 3.14 に未対応のことがあるため、Python 3.12 または 3.13 で作り直してください。
- `python3.12 -m venv` が `ensurepip` で失敗する。OS package の `python3.12-venv` が足りないか、StabilityMatrix などの relocatable Python が `/install` prefix を参照して venv 内で stdlib を見失っています。この場合は OS package の Python を入れるか、conda env を使ってください。
- `paddleocr: exit status 1` の末尾に `(Unimplemented) ConvertPirAttribute2RuntimeAttribute not support ... onednn_instruction.cc` が出る。`paddlepaddle 3.3.x` の CPU / oneDNN 経路で再現することがあるため、PaddleOCR 3.x の install guide と同じ `paddlepaddle==3.2.0` に pin して再試行してください。`ccache` warning と `Model files already exist` はこの失敗の直接原因ではありません。
- venv には入っているが、backend が system `python3` を使っている。
- `.env` の `HAOHAO_DRIVE_PADDLEOCR_PYTHON` が相対 path、古い path、または存在しない Python を指している。
- GPU 版 PaddlePaddle は入っているが、`.env` の `HAOHAO_DRIVE_PADDLEOCR_DEVICE` が未指定または `cpu` のままになっている。
- Python / PaddlePaddle wheel の対応 version が合っていない。
- 初回 model download / load が重く、OCR timeout が短すぎる。Tenant Admin の OCR timeout を大きめにして再試行してください。

conda を使う場合:

```bash
conda create -p "$PWD/.conda-paddleocr" python=3.12 pip -y
conda activate "$PWD/.conda-paddleocr"
python -m pip install --upgrade pip
python -m pip install paddleocr
python -m pip install paddlepaddle==3.2.0 -i https://www.paddlepaddle.org.cn/packages/stable/cpu/

export HAOHAO_DRIVE_PADDLEOCR_PYTHON="$PWD/.conda-paddleocr/bin/python"
export HAOHAO_DRIVE_PADDLEOCR_DEVICE=cpu
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

## Sources

- https://github.com/PaddlePaddle/PaddleOCR
- https://www.paddleocr.ai/latest/en/version3.x/installation.html
- https://www.paddleocr.ai/latest/en/version3.x/pipeline_usage/OCR.html
- https://www.paddleocr.ai/latest/en/version3.x/inference_engine.html
