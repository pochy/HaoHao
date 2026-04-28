#!/usr/bin/env python3
import html
import json
import re
import sys
import unicodedata


HELPER_NAME = "drive_product_extraction_nlp.py"
SCHEMA_VERSION = 1
DEFAULT_RULES = {
    "candidateScoreThreshold": 4,
    "maxBlockRunes": 3000,
    "contextWindowRunes": 800,
    "priceExtractionEnabled": True,
}
DEFAULT_LIMITS = {
    "maxItems": 50,
}

PRODUCT_NAME_LABELS = ["商品名", "品名", "製品名", "名称"]
BRAND_LABELS = ["ブランド", "Brand"]
MANUFACTURER_LABELS = ["メーカー", "製造元", "販売元", "発売元", "Manufacturer"]
SKU_LABELS = ["SKU", "品番", "商品コード", "管理番号", "製品番号"]
MODEL_LABELS = ["型番", "形名", "Model No.", "Model", "モデル"]
CATEGORY_LABELS = ["カテゴリ", "カテゴリー", "分類"]
CAPACITY_LABELS = ["内容量", "容量"]
SIZE_LABELS = ["サイズ", "寸法"]
COLOR_LABELS = ["カラー", "色"]
ALL_FIELD_LABELS = (
    PRODUCT_NAME_LABELS
    + BRAND_LABELS
    + MANUFACTURER_LABELS
    + SKU_LABELS
    + MODEL_LABELS
    + CATEGORY_LABELS
    + CAPACITY_LABELS
    + SIZE_LABELS
    + COLOR_LABELS
    + ["JAN", "JANコード", "バーコード", "価格", "本体価格", "税込"]
)
NEGATIVE_TERMS = [
    "会社概要",
    "お問い合わせ",
    "利用規約",
    "プライバシーポリシー",
    "返品",
    "送料",
    "配送",
    "特定商取引法",
    "ログイン",
    "会員登録",
    "カート",
    "お気に入り",
    "レビュー一覧",
    "FAQ",
]
GENERIC_HEADINGS = {
    "アクセサリ",
    "ペリフェラル",
    "仕様固定",
    "別",
    "注",
    "搭載",
    "ポート",
}

HTML_TAG_RE = re.compile(r"(?s)<[^>]+>")
BLANK_LINE_RE = re.compile(r"\n{3,}")
JAN_RE = re.compile(r"\b(?:\d{13}|\d{8})\b")
JAN13_RE = re.compile(r"\b\d{13}\b")
PRICE_RE = re.compile(r"(?i)(税込|税抜|本体価格|価格)?\s*[:：]?\s*(?:¥\s*([0-9][0-9,]*)|([0-9][0-9,]*)\s*円)")
UNIT_RE = re.compile(r"(?i)\b[0-9]+(?:\.[0-9]+)?\s*(?:g|kg|mg|ml|mL|l|L|cm|mm|m)\b|[0-9]+(?:本|枚|個|袋|錠|包|箱|巻)")
MODEL_VALUE_RE = re.compile(r"(?i)\b[A-Z0-9][A-Z0-9_.\-/]{2,}\b")
MODEL_PRICE_PAIR_RE = re.compile(r"(?i)\b(?P<model>[A-Z0-9][A-Z0-9_.\-/]{2,})\b\s*¥\s*(?P<amount>[0-9][0-9,]*)")
JAN_LABEL_RE = re.compile(r"JANコード\s*[:：]?\s*(\d{13}|\d{8})")
HEADING_NOISE_RE = re.compile(r"^[\d\s\-|/\\:：.。,、]+$")
NOUNISH_RE = re.compile(r"[A-Za-z0-9一-龯ぁ-んァ-ン][A-Za-z0-9一-龯ぁ-んァ-ン・ー\-/ ]{1,}")


def main(argv):
    if len(argv) >= 2 and argv[0] == "--check":
        check_mode(argv[1])
        return 0
    if len(argv) == 1 and argv[0] == "extract":
        payload = json.load(sys.stdin)
        mode = clean_mode(payload.get("mode", "python"))
        engine = load_engine(mode)
        items = extract_items(payload, mode, engine)
        json.dump({"items": items}, sys.stdout, ensure_ascii=False, separators=(",", ":"))
        sys.stdout.write("\n")
        return 0
    fail("usage: drive_product_extraction_nlp.py --check <python|ginza|sudachipy> | extract")


def check_mode(mode):
    mode = clean_mode(mode)
    if mode == "python":
        print("python helper " + sys.version.split()[0])
        return
    load_engine(mode)
    print(mode + " helper available")


def clean_mode(value):
    mode = str(value or "python").strip().lower()
    if mode not in {"python", "ginza", "sudachipy"}:
        fail("unsupported mode: " + mode)
    return mode


def load_engine(mode):
    if mode == "python":
        return None
    if mode == "ginza":
        try:
            import ginza  # noqa: F401
            import spacy
        except Exception as exc:
            fail("ginza dependencies unavailable: " + str(exc))
        try:
            return spacy.load("ja_ginza")
        except Exception as exc:
            try:
                return spacy.load("ja_ginza", config={"components": {"compound_splitter": {"split_mode": "C"}}})
            except Exception as retry_exc:
                fail("ja_ginza model unavailable: " + str(retry_exc or exc))
    if mode == "sudachipy":
        try:
            from sudachipy import dictionary
            from sudachipy import tokenizer
        except Exception as exc:
            fail("sudachipy dependencies unavailable: " + str(exc))
        try:
            sudachi = dictionary.Dictionary().create()
            sudachi.tokenize("商品名 テスト", tokenizer.Tokenizer.SplitMode.C)
            return {"tokenizer": sudachi, "split_mode": tokenizer.Tokenizer.SplitMode.C}
        except Exception as exc:
            fail("sudachipy tokenizer unavailable: " + str(exc))
    fail("unsupported mode: " + mode)


def fail(message):
    print(message, file=sys.stderr)
    raise SystemExit(1)


def extract_items(payload, mode, engine):
    rules = normalize_rules(payload.get("rules") or {})
    limits = normalize_limits(payload.get("limits") or {})
    sources = build_sources(payload)
    blocks = build_blocks(sources, rules)
    candidates = []
    for block in blocks:
        block["score"] = score_block(block["text"], rules["priceExtractionEnabled"])
        if block["score"] < rules["candidateScoreThreshold"]:
            continue
        noun_phrases = extract_noun_phrases(block["text"], mode, engine)
        multi_items = build_multi_product_items(block, rules, mode, noun_phrases)
        if multi_items:
            candidates.extend(multi_items)
            continue
        item = build_item(block, rules, mode, noun_phrases)
        if item is not None:
            candidates.append(item)
    return merge_items(candidates)[: limits["maxItems"]]


def normalize_rules(value):
    rules = dict(DEFAULT_RULES)
    for key in rules:
        if key in value:
            rules[key] = value[key]
    rules["candidateScoreThreshold"] = bounded_int(rules["candidateScoreThreshold"], 0, 20, DEFAULT_RULES["candidateScoreThreshold"])
    rules["maxBlockRunes"] = bounded_int(rules["maxBlockRunes"], 500, 10000, DEFAULT_RULES["maxBlockRunes"])
    rules["contextWindowRunes"] = bounded_int(rules["contextWindowRunes"], 100, 3000, DEFAULT_RULES["contextWindowRunes"])
    rules["priceExtractionEnabled"] = bool(rules["priceExtractionEnabled"])
    return rules


def normalize_limits(value):
    limits = dict(DEFAULT_LIMITS)
    for key in limits:
        if key in value:
            limits[key] = value[key]
    limits["maxItems"] = bounded_int(limits["maxItems"], 1, 100, DEFAULT_LIMITS["maxItems"])
    return limits


def bounded_int(value, minimum, maximum, default):
    try:
        number = int(value)
    except (TypeError, ValueError):
        return default
    return max(minimum, min(maximum, number))


def build_sources(payload):
    sources = []
    pages = payload.get("pages") or []
    for page in pages:
        text = str(page.get("rawText") or "").strip()
        if not text:
            continue
        sources.append({"pageNumber": bounded_int(page.get("pageNumber"), 1, 100000, 1), "text": text})
    if not sources:
        text = str(payload.get("text") or "").strip()
        if text:
            sources.append({"pageNumber": 1, "text": text})
    return sources


def build_blocks(sources, rules):
    blocks = []
    seen = set()
    for source in sources:
        normalized = normalize_text(source["text"])
        if not normalized:
            continue
        lines = normalized.split("\n")

        def add_block(start_line, block_lines):
            text = "\n".join(block_lines).strip()
            if not text:
                return
            for part in split_block_text(text, rules["maxBlockRunes"]):
                key = str(source["pageNumber"]) + "|" + part
                if key in seen:
                    continue
                seen.add(key)
                blocks.append({"pageNumber": source["pageNumber"], "startLine": start_line, "text": part, "score": 0})

        current = []
        start_line = 1
        for index, line in enumerate(lines):
            if not line.strip():
                add_block(start_line, current)
                current = []
                start_line = index + 2
                continue
            if not current:
                start_line = index + 1
            current.append(line)
        add_block(start_line, current)

        non_empty = []
        line_numbers = []
        for index, line in enumerate(lines):
            line = line.strip()
            if not line:
                continue
            non_empty.append(line)
            line_numbers.append(index + 1)
        for index, line in enumerate(non_empty):
            if not line_has_anchor(line, rules["priceExtractionEnabled"]):
                continue
            window, window_start = context_lines(non_empty, index, rules["contextWindowRunes"])
            if window:
                add_block(line_numbers[window_start], window)
    return blocks


def normalize_text(text):
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = html.unescape(HTML_TAG_RE.sub(" ", text))
    text = unicodedata.normalize("NFKC", text)
    text = text.replace("￥", "¥").replace("，", ",").replace("：", ":").replace("－", "-").replace("―", "-")
    lines = [" ".join(line.split()) for line in text.split("\n")]
    return BLANK_LINE_RE.sub("\n\n", "\n".join(lines)).strip()


def split_block_text(text, limit):
    if limit <= 0 or len(text) <= limit:
        return [text]
    parts = []
    current = []
    current_len = 0
    for line in text.split("\n"):
        line_len = len(line)
        if current and current_len + line_len + 1 > limit:
            parts.append("\n".join(current).strip())
            current = []
            current_len = 0
        if line_len > limit:
            while len(line) > limit:
                parts.append(line[:limit])
                line = line[limit:]
            if line:
                current.append(line)
                current_len = len(line)
            continue
        current.append(line)
        current_len += line_len + 1
    if current:
        parts.append("\n".join(current).strip())
    return [part for part in parts if part]


def line_has_anchor(line, price_enabled):
    return bool(JAN_RE.search(line) or (price_enabled and PRICE_RE.search(line)) or contains_any(line, ALL_FIELD_LABELS))


def context_lines(lines, index, limit):
    start = index
    end = index + 1
    total = len(lines[index])
    while (start > 0 or end < len(lines)) and total < limit:
        if start > 0:
            start -= 1
            total += len(lines[start]) + 1
        if total >= limit:
            break
        if end < len(lines):
            total += len(lines[end]) + 1
            end += 1
    return lines[start:end], start


def score_block(text, price_enabled):
    score = 0
    if JAN13_RE.search(text):
        score += 5
    elif JAN_RE.search(text):
        score += 3
    if price_enabled and PRICE_RE.search(text):
        score += 4
    if label_value(text, PRODUCT_NAME_LABELS):
        score += 4
    if label_value(text, BRAND_LABELS) or label_value(text, MANUFACTURER_LABELS):
        score += 3
    if label_value(text, CAPACITY_LABELS) or label_value(text, SIZE_LABELS) or label_value(text, COLOR_LABELS):
        score += 3
    if label_value(text, SKU_LABELS) or label_value(text, MODEL_LABELS):
        score += 3
    if UNIT_RE.search(text):
        score += 2
    if looks_like_description(text):
        score += 2
    if nounish_token_count(text) >= 3:
        score += 1
    for term in NEGATIVE_TERMS:
        if term not in text:
            continue
        if term in {"利用規約", "プライバシーポリシー"}:
            score -= 5
        elif term in {"会社概要", "お問い合わせ"}:
            score -= 4
        else:
            score -= 3
    return score


def build_item(block, rules, mode, noun_phrases):
    text = block["text"]
    price, has_price = extract_price(text, rules["priceExtractionEnabled"])
    jan_match = JAN_RE.search(text)
    jan = jan_match.group(0) if jan_match else ""
    name, name_source = product_name(text, jan, has_price)
    if not name:
        for phrase in noun_phrases:
            candidate = clean_product_name(phrase)
            if candidate:
                name = candidate
                name_source = "nounPhrase"
                break
    brand = inferred_brand(text)
    manufacturer = clean_field_value(label_value(text, MANUFACTURER_LABELS))
    model = extract_code_value(text, MODEL_LABELS)
    sku = extract_code_value(text, SKU_LABELS)
    category = clean_field_value(label_value(text, CATEGORY_LABELS))
    capacity = clean_field_value(label_value(text, CAPACITY_LABELS))
    size = clean_field_value(label_value(text, SIZE_LABELS))
    color = clean_field_value(label_value(text, COLOR_LABELS))

    if not name:
        if model:
            name = model
            name_source = "model"
        elif sku:
            name = sku
            name_source = "sku"
        elif jan:
            name = jan
            name_source = "janCode"
        else:
            return None

    confidence = confidence_score(block["score"], rules["candidateScoreThreshold"], name, brand, jan, sku, model, has_price, capacity, size, color, noun_phrases)
    source_text = truncate(text, 2000)
    attributes = {
        "schemaVersion": SCHEMA_VERSION,
        "extractor": mode,
        "pythonHelper": HELPER_NAME,
        "nlpEngine": mode,
        "rulesScore": block["score"],
        "rulesCandidateThreshold": rules["candidateScoreThreshold"],
        "rulesBlockRunes": len(text),
        "rulesBlockPageNumber": block["pageNumber"],
        "rulesBlockStartLine": block["startLine"],
        "nameDerivedFrom": name_source,
        "priceExtractionEnabled": rules["priceExtractionEnabled"],
        "rulesMaxBlockRunes": rules["maxBlockRunes"],
        "rulesContextWindowRunes": rules["contextWindowRunes"],
        "nounPhrases": noun_phrases[:10],
    }
    if size:
        attributes["size"] = size
    if color:
        attributes["color"] = color
    if capacity:
        attributes["capacity"] = capacity

    return {
        "itemType": "product",
        "name": name,
        "brand": brand,
        "manufacturer": manufacturer,
        "model": model,
        "sku": sku,
        "janCode": jan,
        "category": category,
        "description": description(text, name),
        "price": price,
        "promotion": promotion(text),
        "availability": {},
        "sourceText": source_text,
        "evidence": [{"pageNumber": block["pageNumber"], "text": source_text}],
        "attributes": attributes,
        "confidence": confidence,
    }


def build_multi_product_items(block, rules, mode, noun_phrases):
    lines = candidate_lines(block["text"])
    items = []
    used_models = set()
    for index, line in enumerate(lines):
        product_jans = product_jan_pairs_from_line(line)
        if len(product_jans) < 2:
            continue
        detail_text = " ".join(lines[index + 1 : index + 4])
        model_prices = model_price_pairs(detail_text)
        source_text = truncate("\n".join(lines[index : index + 4]), 2000)
        for offset, product in enumerate(product_jans):
            model = ""
            price = {}
            has_price = False
            if offset < len(model_prices):
                model = model_prices[offset]["model"]
                price = model_prices[offset]["price"]
                has_price = True
            item = build_row_product_item(
                block,
                rules,
                mode,
                noun_phrases,
                product["name"],
                product["janCode"],
                model,
                price,
                has_price,
                source_text,
            )
            if item is not None:
                items.append(item)
                if item.get("model"):
                    used_models.add(item["model"].lower())
    items.extend(build_model_price_jan_items(block, rules, mode, noun_phrases, lines, used_models))
    return items


def product_jan_pairs_from_line(line):
    matches = list(JAN_LABEL_RE.finditer(line))
    pairs = []
    previous_end = 0
    for match in matches:
        segment = line[previous_end : match.start()]
        name = clean_product_name(segment)
        if name:
            pairs.append({"name": name, "janCode": match.group(1)})
        previous_end = match.end()
    return pairs


def model_price_pairs(text):
    pairs = []
    for match in MODEL_PRICE_PAIR_RE.finditer(text):
        amount = int(match.group("amount").replace(",", ""))
        pairs.append(
            {
                "model": match.group("model"),
                "price": {
                    "amount": amount,
                    "currency": "JPY",
                    "taxIncluded": False,
                    "source": match.group(0).strip(),
                },
            }
        )
    return pairs


def build_model_price_jan_items(block, rules, mode, noun_phrases, lines, used_models):
    items = []
    for index, line in enumerate(lines):
        if JAN_LABEL_RE.search(line) or PRICE_RE.search(line):
            continue
        model_match = MODEL_VALUE_RE.search(line)
        if not model_match:
            continue
        model = model_match.group(0)
        if model.lower() in used_models:
            continue
        lookahead = " ".join(lines[index : index + 4])
        jan_match = JAN_LABEL_RE.search(lookahead)
        price, has_price = extract_price(lookahead, rules["priceExtractionEnabled"])
        if not jan_match or not has_price:
            continue
        name = nearest_product_heading(lines, index)
        if not name:
            continue
        source_text = truncate("\n".join(lines[max(0, index - 6) : index + 4]), 2000)
        item = build_row_product_item(
            block,
            rules,
            mode,
            noun_phrases,
            name,
            jan_match.group(1),
            model,
            price,
            has_price,
            source_text,
        )
        if item is not None:
            item["attributes"]["nameDerivedFrom"] = "nearbyHeading"
            items.append(item)
            used_models.add(model.lower())
    return items


def nearest_product_heading(lines, index):
    for current in range(index - 1, max(-1, index - 14), -1):
        line = lines[current]
        if "単位" in line or line_has_anchor(line, True) or MODEL_VALUE_RE.search(line) or UNIT_RE.search(line):
            continue
        name = clean_product_name(line)
        if not name or name in GENERIC_HEADINGS or len(name) > 50:
            continue
        return name
    return ""


def build_row_product_item(block, rules, mode, noun_phrases, name, jan, model, price, has_price, source_text):
    name = clean_product_name(name)
    if not name:
        return None
    text = block["text"]
    brand = inferred_brand(text)
    manufacturer = clean_field_value(label_value(text, MANUFACTURER_LABELS))
    sku = ""
    category = clean_field_value(label_value(text, CATEGORY_LABELS))
    capacity = clean_field_value(label_value(text, CAPACITY_LABELS))
    size = clean_field_value(label_value(text, SIZE_LABELS))
    color = clean_field_value(label_value(text, COLOR_LABELS))
    confidence = confidence_score(block["score"], rules["candidateScoreThreshold"], name, brand, jan, sku, model, has_price, capacity, size, color, noun_phrases)
    attributes = {
        "schemaVersion": SCHEMA_VERSION,
        "extractor": mode,
        "pythonHelper": HELPER_NAME,
        "nlpEngine": mode,
        "rulesScore": block["score"],
        "rulesCandidateThreshold": rules["candidateScoreThreshold"],
        "rulesBlockRunes": len(text),
        "rulesBlockPageNumber": block["pageNumber"],
        "rulesBlockStartLine": block["startLine"],
        "nameDerivedFrom": "row",
        "priceExtractionEnabled": rules["priceExtractionEnabled"],
        "rulesMaxBlockRunes": rules["maxBlockRunes"],
        "rulesContextWindowRunes": rules["contextWindowRunes"],
        "nounPhrases": noun_phrases[:10],
    }
    if size:
        attributes["size"] = size
    if color:
        attributes["color"] = color
    if capacity:
        attributes["capacity"] = capacity
    return {
        "itemType": "product",
        "name": name,
        "brand": brand,
        "manufacturer": manufacturer,
        "model": model,
        "sku": sku,
        "janCode": jan,
        "category": category,
        "description": "",
        "price": price,
        "promotion": promotion(text),
        "availability": {},
        "sourceText": source_text,
        "evidence": [{"pageNumber": block["pageNumber"], "text": source_text}],
        "attributes": attributes,
        "confidence": confidence,
    }


def product_name(text, jan, has_price):
    labeled = clean_product_name(label_value(text, PRODUCT_NAME_LABELS))
    if labeled:
        return labeled, "label"
    lines = candidate_lines(text)
    for index, line in enumerate(lines):
        if jan and jan in line:
            name = nearby_name(lines, index)
            if name:
                return name, "nearby"
        if has_price and PRICE_RE.search(line):
            name = nearby_name(lines, index)
            if name:
                return name, "nearby"
    for line in lines:
        name = clean_product_name(line)
        if name:
            return name, "heading"
    return "", ""


def inferred_brand(text):
    value = clean_field_value(label_value(text, BRAND_LABELS))
    if value:
        return value
    match = re.search(r"([A-Za-z0-9一-龯ァ-ン]{2,24})製品", text)
    if match:
        return clean_field_value(match.group(1))
    return ""


def nearby_name(lines, index):
    name = clean_product_name(lines[index])
    if name:
        return name
    for current in range(index - 1, max(-1, index - 4), -1):
        name = clean_product_name(lines[current])
        if name:
            return name
    for current in range(index + 1, min(len(lines), index + 3)):
        name = clean_product_name(lines[current])
        if name:
            return name
    return ""


def label_value(text, labels):
    for line in candidate_lines(text):
        lower_line = line.lower()
        for label in labels:
            index = lower_line.find(label.lower())
            if index < 0:
                continue
            value = line[index + len(label) :].strip()
            value = value.lstrip(" :：-=/")
            value = trim_before_next_label(value)
            if value.strip():
                return value
    return ""


def trim_before_next_label(value):
    cut = len(value)
    lower_value = value.lower()
    for label in ALL_FIELD_LABELS:
        index = lower_value.find(label.lower())
        if 0 < index < cut and value[:index].strip():
            cut = index
    return value[:cut].strip()


def remove_labeled_segments(value):
    for label in ALL_FIELD_LABELS:
        while True:
            index = value.lower().find(label.lower())
            if index < 0:
                break
            value = value[:index].strip()
    return value


def clean_product_name(value):
    value = PRICE_RE.sub("", value)
    value = JAN_RE.sub("", value)
    value = remove_labeled_segments(value)
    value = " ".join(value.strip(" -:：|/　\t").split())
    if not value or HEADING_NOISE_RE.match(value) or contains_any(value, NEGATIVE_TERMS):
        return ""
    if len(value) > 120:
        return ""
    return value


def clean_field_value(value):
    value = PRICE_RE.sub("", value)
    value = JAN_RE.sub("", value)
    value = " ".join(value.strip(" -:：|/　\t").split())
    if len(value) > 120:
        return ""
    return value


def extract_code_value(text, labels):
    value = clean_field_value(label_value(text, labels))
    if not value:
        return ""
    match = MODEL_VALUE_RE.search(value)
    return match.group(0) if match else value


def extract_price(text, enabled):
    if not enabled:
        return {}, False
    match = PRICE_RE.search(text)
    if not match:
        return {}, False
    raw_amount = match.group(2) or match.group(3) or ""
    try:
        amount = int(raw_amount.replace(",", ""))
    except ValueError:
        return {}, False
    return {
        "amount": amount,
        "currency": "JPY",
        "taxIncluded": "税込" in match.group(0),
        "source": match.group(0).strip(),
    }, True


def promotion(text):
    for label in ["特価", "セール", "割引", "ポイント"]:
        if label in text:
            return {"label": label}
    return {}


def description(text, name):
    for line in candidate_lines(text):
        if line == name or contains_any(line, NEGATIVE_TERMS) or line_has_anchor(line, True):
            continue
        if len(line) >= 24:
            return truncate(line, 300)
    return ""


def confidence_score(score, threshold, name, brand, jan, sku, model, has_price, capacity, size, color, noun_phrases):
    confidence = 0.05
    if jan:
        confidence += 0.25
    if has_price:
        confidence += 0.15
    if name:
        confidence += 0.25
    if brand:
        confidence += 0.10
    if sku or model:
        confidence += 0.10
    if capacity or size or color:
        confidence += 0.10
    if noun_phrases:
        confidence += 0.05
    if score >= threshold + 4:
        confidence += 0.15
    elif score >= threshold:
        confidence += 0.07
    return max(0.0, min(1.0, round(confidence, 4)))


def extract_noun_phrases(text, mode, engine):
    if mode == "ginza":
        return extract_ginza_noun_phrases(text, engine)
    if mode == "sudachipy":
        return extract_sudachipy_noun_phrases(text, engine)
    return extract_regex_noun_phrases(text)


def extract_regex_noun_phrases(text):
    phrases = []
    for line in candidate_lines(text):
        cleaned = clean_product_name(line)
        if cleaned:
            phrases.append(cleaned)
        for match in NOUNISH_RE.finditer(line):
            value = clean_product_name(match.group(0))
            if value:
                phrases.append(value)
    return unique_phrases(phrases)


def extract_ginza_noun_phrases(text, nlp):
    phrases = []
    doc = nlp(truncate(text.replace("\n", " "), 3000))
    try:
        phrases.extend(chunk.text for chunk in doc.noun_chunks)
    except Exception:
        pass
    current = []
    for token in doc:
        if token.pos_ in {"NOUN", "PROPN", "NUM", "SYM"} and not token.is_space:
            current.append(token.text)
            continue
        if current:
            phrases.append("".join(current))
            current = []
    if current:
        phrases.append("".join(current))
    return unique_phrases(phrases)


def extract_sudachipy_noun_phrases(text, engine):
    phrases = []
    current = []
    tokenizer = engine["tokenizer"]
    split_mode = engine["split_mode"]
    for token in tokenizer.tokenize(truncate(text.replace("\n", " "), 3000), split_mode):
        pos = token.part_of_speech()
        if pos and pos[0] == "名詞":
            current.append(token.surface())
            continue
        if current:
            phrases.append("".join(current))
            current = []
    if current:
        phrases.append("".join(current))
    return unique_phrases(phrases)


def unique_phrases(values):
    phrases = []
    seen = set()
    for value in values:
        value = clean_product_name(str(value or ""))
        if len(value) < 2:
            continue
        key = value.lower()
        if key in seen:
            continue
        seen.add(key)
        phrases.append(value)
        if len(phrases) >= 20:
            break
    return phrases


def merge_items(items):
    merged = []
    by_key = {}
    for item in items:
        key = dedup_key(item)
        if not key:
            merged.append(item)
            continue
        existing_index = by_key.get(key)
        if existing_index is None:
            by_key[key] = len(merged)
            merged.append(item)
            continue
        merged[existing_index] = merge_item(merged[existing_index], item)
    return merged


def merge_item(left, right):
    if float(right.get("confidence") or 0) > float(left.get("confidence") or 0):
        primary = right
        secondary = left
    else:
        primary = left
        secondary = right
    for field in ["name", "brand", "manufacturer", "model", "sku", "janCode", "category", "description", "sourceText"]:
        if not primary.get(field) and secondary.get(field):
            primary[field] = secondary[field]
    for field in ["price", "promotion", "availability"]:
        if not primary.get(field) and secondary.get(field):
            primary[field] = secondary[field]
    primary["evidence"] = merge_evidence(primary.get("evidence") or [], secondary.get("evidence") or [])
    primary["attributes"] = merge_attributes(primary.get("attributes") or {}, secondary.get("attributes") or {})
    return primary


def merge_evidence(left, right):
    merged = []
    seen = set()
    for evidence in left + right:
        key = json.dumps(evidence, ensure_ascii=False, sort_keys=True)
        if key in seen:
            continue
        seen.add(key)
        merged.append(evidence)
    return merged


def merge_attributes(left, right):
    merged = dict(right)
    merged.update(left)
    phrases = []
    for value in (left.get("nounPhrases") or []) + (right.get("nounPhrases") or []):
        if value not in phrases:
            phrases.append(value)
    if phrases:
        merged["nounPhrases"] = phrases[:20]
    return merged


def dedup_key(item):
    if item.get("janCode"):
        return "jan:" + item["janCode"]
    if item.get("sku"):
        return "sku:" + item["sku"].lower()
    if item.get("model"):
        return "model:" + item["model"].lower()
    if item.get("name") and item.get("brand"):
        return "name-brand:" + item["name"].lower() + "|" + item["brand"].lower()
    price = item.get("price") or {}
    if item.get("name") and price.get("amount") is not None:
        return "name-price:" + item["name"].lower() + "|" + str(price.get("amount"))
    return ""


def candidate_lines(text):
    return [" ".join(line.split()) for line in text.split("\n") if line.strip()]


def nounish_token_count(text):
    return len(NOUNISH_RE.findall(text))


def looks_like_description(text):
    for line in candidate_lines(text):
        if len(line) >= 24 and not line_has_anchor(line, True) and not contains_any(line, NEGATIVE_TERMS):
            return True
    return False


def contains_any(value, terms):
    return any(term in value for term in terms)


def truncate(value, limit):
    value = str(value or "")
    if len(value) <= limit:
        return value
    return value[:limit]


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
