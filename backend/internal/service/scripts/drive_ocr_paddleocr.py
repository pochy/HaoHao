#!/usr/bin/env python3
import importlib.metadata
import importlib.util
import json
import os
import statistics
import sys
from contextlib import redirect_stdout


def main() -> int:
    if len(sys.argv) >= 2 and sys.argv[1] == "--check":
        return check()
    if len(sys.argv) >= 2 and sys.argv[1] == "extract":
        return extract()
    print("usage: drive_ocr_paddleocr.py --check | extract", file=sys.stderr)
    return 2


def check() -> int:
    missing = []
    if importlib.util.find_spec("paddleocr") is None:
        missing.append("paddleocr")
    if importlib.util.find_spec("paddle") is None:
        missing.append("paddlepaddle")
    if missing:
        print(f"paddleocr dependency unavailable: missing {', '.join(missing)}", file=sys.stderr)
        return 2

    parts = [
        f"paddleocr {package_version('paddleocr')}",
        f"paddle {package_version('paddlepaddle')}",
    ]
    print(" / ".join(parts))
    return 0


def extract() -> int:
    try:
        with redirect_stdout(sys.stderr):
            from paddleocr import PaddleOCR
    except Exception as exc:
        print(f"paddleocr dependency unavailable: {exc}", file=sys.stderr)
        return 2

    try:
        payload = json.load(sys.stdin)
        image_path = clean_string(payload.get("imagePath"))
        if not image_path:
            fail("imagePath is required")
        if not os.path.exists(image_path):
            fail("imagePath does not exist")
        lang = clean_string(payload.get("lang")) or "en"
        device = clean_string(payload.get("device")) or "cpu"
        use_textline_orientation = bool(payload.get("useTextlineOrientation", True))

        with redirect_stdout(sys.stderr):
            ocr = create_ocr(PaddleOCR, lang, device, use_textline_orientation)
            results = predict(ocr, image_path, use_textline_orientation)
        entries = collect_entries(results)
        text_lines = [entry["text"] for entry in entries if entry.get("text")]
        scores = [entry["score"] for entry in entries if isinstance(entry.get("score"), (int, float))]
        avg = statistics.fmean(scores) if scores else None
        response = {
            "text": "\n".join(text_lines),
            "averageConfidence": avg,
            "layout": {
                "engine": "paddleocr",
                "lang": lang,
                "device": device,
                "lineCount": len(text_lines),
            },
            "boxes": entries,
        }
        print(json.dumps(response, ensure_ascii=False, separators=(",", ":")))
        return 0
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1


def create_ocr(PaddleOCR, lang: str, device: str, use_textline_orientation: bool):
    try:
        return PaddleOCR(
            lang=lang,
            device=device,
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=use_textline_orientation,
        )
    except TypeError:
        return PaddleOCR(
            lang=lang,
            use_gpu=device.startswith("gpu"),
            use_angle_cls=use_textline_orientation,
        )
    except ValueError as exc:
        if "Unknown argument" not in str(exc) and "unexpected" not in str(exc).lower():
            raise
        return PaddleOCR(
            lang=lang,
            use_gpu=device.startswith("gpu"),
            use_angle_cls=use_textline_orientation,
        )


def predict(ocr, image_path: str, use_textline_orientation: bool):
    if hasattr(ocr, "predict"):
        return ocr.predict(image_path)
    return ocr.ocr(image_path, cls=use_textline_orientation)


def collect_entries(results):
    entries = []
    if results is None:
        return entries
    for result in ensure_list(results):
        collect_from_plain(to_plain_result(result), entries)
    return entries


def collect_from_plain(value, entries):
    value = to_plain(value)
    if isinstance(value, dict):
        texts = value.get("rec_texts")
        if isinstance(texts, list):
            scores = value.get("rec_scores")
            boxes = value.get("rec_polys") or value.get("rec_boxes") or value.get("dt_polys")
            for index, text in enumerate(texts):
                text = clean_string(text)
                if not text:
                    continue
                entries.append({
                    "text": text,
                    "score": numeric_at(scores, index),
                    "box": item_at(boxes, index),
                })
            return
        for item in value.values():
            collect_from_plain(item, entries)
        return
    if isinstance(value, list):
        if looks_like_v2_line(value):
            text = clean_string(value[1][0])
            if text:
                entries.append({
                    "text": text,
                    "score": clean_number(value[1][1]),
                    "box": to_plain(value[0]),
                })
            return
        for item in value:
            collect_from_plain(item, entries)


def to_plain_result(result):
    if isinstance(result, dict):
        return result.get("res", result)
    value = getattr(result, "json", None)
    if value is not None:
        if callable(value):
            value = value()
        if isinstance(value, str):
            try:
                value = json.loads(value)
            except json.JSONDecodeError:
                pass
        if isinstance(value, dict):
            return value.get("res", value)
    to_dict = getattr(result, "to_dict", None)
    if callable(to_dict):
        value = to_dict()
        if isinstance(value, dict):
            return value.get("res", value)
    return result


def looks_like_v2_line(value):
    return (
        len(value) >= 2
        and isinstance(value[1], (list, tuple))
        and len(value[1]) >= 2
        and isinstance(value[1][0], str)
    )


def to_plain(value):
    if hasattr(value, "tolist"):
        return value.tolist()
    if isinstance(value, dict):
        return {str(key): to_plain(item) for key, item in value.items()}
    if isinstance(value, tuple):
        return [to_plain(item) for item in value]
    if isinstance(value, list):
        return [to_plain(item) for item in value]
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    return str(value)


def ensure_list(value):
    if isinstance(value, list):
        return value
    return [value]


def numeric_at(values, index):
    values = to_plain(values)
    if isinstance(values, list) and index < len(values):
        return clean_number(values[index])
    return None


def item_at(values, index):
    values = to_plain(values)
    if isinstance(values, list) and index < len(values):
        return values[index]
    return None


def clean_number(value):
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def package_version(package_name: str) -> str:
    try:
        return importlib.metadata.version(package_name)
    except importlib.metadata.PackageNotFoundError:
        return "unknown"


def clean_string(value):
    if not isinstance(value, str):
        return ""
    return " ".join(value.replace("\x00", " ").split())


def fail(message: str):
    raise ValueError(message)


if __name__ == "__main__":
    raise SystemExit(main())
