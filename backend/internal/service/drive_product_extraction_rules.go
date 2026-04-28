package service

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	rulesProductExtractionItemLimit = 100
	rulesSourceTextLimit            = 2000
)

type RulesDriveProductExtractor struct{}

func NewRulesDriveProductExtractor() RulesDriveProductExtractor {
	return RulesDriveProductExtractor{}
}

func (RulesDriveProductExtractor) Name() string {
	return "rules"
}

type rulesProductBlock struct {
	PageNumber int
	StartLine  int
	Text       string
	Score      int
}

type rulesProductCandidate struct {
	item DriveProductExtractionItem
	key  string
}

type rulesPriceMatch struct {
	Amount      int
	TaxIncluded bool
	Raw         string
}

var (
	rulesHTMLTagPattern      = regexp.MustCompile(`(?s)<[^>]+>`)
	rulesBlankLinePattern    = regexp.MustCompile(`\n{3,}`)
	rulesJANPattern          = regexp.MustCompile(`\b(?:\d{13}|\d{8})\b`)
	rulesJAN13Pattern        = regexp.MustCompile(`\b\d{13}\b`)
	rulesPricePattern        = regexp.MustCompile(`(?i)(税込|税抜|本体価格|価格)?\s*[:：]?\s*(?:¥\s*([0-9][0-9,]*)|([0-9][0-9,]*)\s*円)`)
	rulesUnitPattern         = regexp.MustCompile(`(?i)\b[0-9]+(?:\.[0-9]+)?\s*(?:g|kg|mg|ml|mL|l|L|cm|mm|m)\b|[0-9]+(?:本|枚|個|袋|錠|包|箱|巻)`)
	rulesModelValuePattern   = regexp.MustCompile(`(?i)\b[A-Z0-9][A-Z0-9_.\-\/]{2,}\b`)
	rulesHeadingNoisePattern = regexp.MustCompile(`^[\d\s\-–—|/\\:：.。,、]+$`)
)

var (
	rulesProductNameLabels  = []string{"商品名", "品名", "製品名", "名称"}
	rulesBrandLabels        = []string{"ブランド", "Brand"}
	rulesManufacturerLabels = []string{"メーカー", "製造元", "販売元", "発売元", "Manufacturer"}
	rulesSKULabels          = []string{"SKU", "品番", "商品コード", "管理番号", "製品番号"}
	rulesModelLabels        = []string{"型番", "形名", "Model No.", "Model", "モデル"}
	rulesCategoryLabels     = []string{"カテゴリ", "カテゴリー", "分類"}
	rulesCapacityLabels     = []string{"内容量", "容量"}
	rulesSizeLabels         = []string{"サイズ", "寸法"}
	rulesColorLabels        = []string{"カラー", "色"}
	rulesAllFieldLabels     = appendStringSlices(rulesProductNameLabels, rulesBrandLabels, rulesManufacturerLabels, rulesSKULabels, rulesModelLabels, rulesCategoryLabels, rulesCapacityLabels, rulesSizeLabels, rulesColorLabels, []string{"JAN", "JANコード", "バーコード", "価格", "本体価格", "税込"})
	rulesNegativeTerms      = []string{"会社概要", "お問い合わせ", "利用規約", "プライバシーポリシー", "返品", "送料", "配送", "特定商取引法", "ログイン", "会員登録", "カート", "お気に入り", "レビュー一覧", "FAQ"}
)

func (RulesDriveProductExtractor) ExtractProducts(_ context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	policy := normalizeDriveOCRPolicy(input.Policy)
	blocks := buildRulesProductBlocks(input, policy.Rules)
	candidates := make([]rulesProductCandidate, 0, len(blocks))
	for _, block := range blocks {
		block.Score = scoreRulesProductBlock(block.Text, policy.Rules.PriceExtractionEnabled)
		if block.Score < policy.Rules.CandidateScoreThreshold {
			continue
		}
		candidate, ok := buildRulesProductCandidate(input, policy.Rules, block)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
	}
	items := mergeRulesProductCandidates(candidates)
	if len(items) > rulesProductExtractionItemLimit {
		items = items[:rulesProductExtractionItemLimit]
	}
	return DriveProductExtractionResult{Items: items}, nil
}

func buildRulesProductBlocks(input DriveProductExtractionInput, policy DriveOCRRulesPolicy) []rulesProductBlock {
	type source struct {
		pageNumber int
		text       string
	}
	sources := make([]source, 0, len(input.Pages)+1)
	for _, page := range input.Pages {
		if strings.TrimSpace(page.RawText) == "" {
			continue
		}
		sources = append(sources, source{pageNumber: page.PageNumber, text: page.RawText})
	}
	if len(sources) == 0 && strings.TrimSpace(input.FullText) != "" {
		sources = append(sources, source{pageNumber: 1, text: input.FullText})
	}

	blocks := make([]rulesProductBlock, 0)
	seen := map[string]struct{}{}
	for _, src := range sources {
		normalized := normalizeRulesText(src.text)
		if normalized == "" {
			continue
		}
		lines := strings.Split(normalized, "\n")
		addBlock := func(startLine int, blockLines []string) {
			text := strings.TrimSpace(strings.Join(blockLines, "\n"))
			if text == "" {
				return
			}
			for _, part := range splitRulesBlockText(text, policy.MaxBlockRunes) {
				key := strconv.Itoa(src.pageNumber) + "|" + part
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				blocks = append(blocks, rulesProductBlock{PageNumber: src.pageNumber, StartLine: startLine, Text: part})
			}
		}

		current := make([]string, 0, 16)
		startLine := 1
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				addBlock(startLine, current)
				current = current[:0]
				startLine = i + 2
				continue
			}
			if len(current) == 0 {
				startLine = i + 1
			}
			current = append(current, line)
		}
		addBlock(startLine, current)

		nonEmptyLines := make([]string, 0, len(lines))
		lineNumbers := make([]int, 0, len(lines))
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			nonEmptyLines = append(nonEmptyLines, line)
			lineNumbers = append(lineNumbers, i+1)
		}
		for i, line := range nonEmptyLines {
			if !rulesLineHasAnchor(line, policy.PriceExtractionEnabled) {
				continue
			}
			window := rulesContextLines(nonEmptyLines, i, policy.ContextWindowRunes)
			if len(window) == 0 {
				continue
			}
			addBlock(lineNumbers[max(0, i-len(window))], window)
		}
	}
	return blocks
}

func normalizeRulesText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = rulesHTMLTagPattern.ReplaceAllString(text, " ")
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\u3000':
			return ' '
		case r == '￥':
			return '¥'
		case r == '，':
			return ','
		case r == '：':
			return ':'
		case r == '－' || r == '―':
			return '-'
		case r >= '０' && r <= '９':
			return r - '０' + '0'
		case r >= 'Ａ' && r <= 'Ｚ':
			return r - 'Ａ' + 'A'
		case r >= 'ａ' && r <= 'ｚ':
			return r - 'ａ' + 'a'
		default:
			return r
		}
	}, text)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		out = append(out, line)
	}
	return strings.TrimSpace(rulesBlankLinePattern.ReplaceAllString(strings.Join(out, "\n"), "\n\n"))
}

func splitRulesBlockText(text string, limit int) []string {
	if limit <= 0 || len([]rune(text)) <= limit {
		return []string{text}
	}
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines)/4+1)
	current := make([]string, 0, 16)
	currentRunes := 0
	for _, line := range lines {
		lineRunes := len([]rune(line))
		if len(current) > 0 && currentRunes+lineRunes+1 > limit {
			parts = append(parts, strings.TrimSpace(strings.Join(current, "\n")))
			current = current[:0]
			currentRunes = 0
		}
		if lineRunes > limit {
			runes := []rune(line)
			for len(runes) > limit {
				parts = append(parts, string(runes[:limit]))
				runes = runes[limit:]
			}
			if len(runes) > 0 {
				current = append(current, string(runes))
				currentRunes = len(runes)
			}
			continue
		}
		current = append(current, line)
		currentRunes += lineRunes + 1
	}
	if len(current) > 0 {
		parts = append(parts, strings.TrimSpace(strings.Join(current, "\n")))
	}
	return parts
}

func rulesLineHasAnchor(line string, priceEnabled bool) bool {
	return rulesJANPattern.MatchString(line) ||
		(priceEnabled && rulesPricePattern.MatchString(line)) ||
		rulesContainsAny(line, rulesAllFieldLabels)
}

func rulesContextLines(lines []string, index, limit int) []string {
	if len(lines) == 0 {
		return nil
	}
	start := index
	end := index + 1
	total := len([]rune(lines[index]))
	for (start > 0 || end < len(lines)) && total < limit {
		if start > 0 {
			start--
			total += len([]rune(lines[start])) + 1
		}
		if total >= limit {
			break
		}
		if end < len(lines) {
			total += len([]rune(lines[end])) + 1
			end++
		}
	}
	return lines[start:end]
}

func scoreRulesProductBlock(text string, priceEnabled bool) int {
	score := 0
	if rulesJAN13Pattern.MatchString(text) {
		score += 5
	} else if rulesJANPattern.MatchString(text) {
		score += 3
	}
	if priceEnabled && rulesPricePattern.MatchString(text) {
		score += 4
	}
	if rulesLabelValue(text, rulesProductNameLabels) != "" {
		score += 4
	}
	if rulesLabelValue(text, rulesBrandLabels) != "" || rulesLabelValue(text, rulesManufacturerLabels) != "" {
		score += 3
	}
	if rulesLabelValue(text, rulesCapacityLabels) != "" || rulesLabelValue(text, rulesSizeLabels) != "" || rulesLabelValue(text, rulesColorLabels) != "" {
		score += 3
	}
	if rulesLabelValue(text, rulesSKULabels) != "" || rulesLabelValue(text, rulesModelLabels) != "" {
		score += 3
	}
	if rulesUnitPattern.MatchString(text) {
		score += 2
	}
	if rulesLooksLikeDescription(text) {
		score += 2
	}
	if rulesNounishTokenCount(text) >= 3 {
		score++
	}
	for _, term := range rulesNegativeTerms {
		if !strings.Contains(text, term) {
			continue
		}
		switch term {
		case "利用規約", "プライバシーポリシー":
			score -= 5
		case "会社概要", "お問い合わせ":
			score -= 4
		default:
			score -= 3
		}
	}
	return score
}

func buildRulesProductCandidate(input DriveProductExtractionInput, policy DriveOCRRulesPolicy, block rulesProductBlock) (rulesProductCandidate, bool) {
	text := block.Text
	price, hasPrice := rulesExtractPrice(text, policy.PriceExtractionEnabled)
	jan := rulesJANPattern.FindString(text)
	name, nameSource := rulesProductName(text, jan, hasPrice)
	brand := rulesCleanFieldValue(rulesLabelValue(text, rulesBrandLabels))
	manufacturer := rulesCleanFieldValue(rulesLabelValue(text, rulesManufacturerLabels))
	model := rulesExtractCodeValue(text, rulesModelLabels)
	sku := rulesExtractCodeValue(text, rulesSKULabels)
	category := rulesCleanFieldValue(rulesLabelValue(text, rulesCategoryLabels))
	capacity := rulesCleanFieldValue(rulesLabelValue(text, rulesCapacityLabels))
	size := rulesCleanFieldValue(rulesLabelValue(text, rulesSizeLabels))
	color := rulesCleanFieldValue(rulesLabelValue(text, rulesColorLabels))

	if name == "" {
		switch {
		case model != "":
			name = model
			nameSource = "model"
		case sku != "":
			name = sku
			nameSource = "sku"
		case jan != "":
			name = jan
			nameSource = "janCode"
		default:
			return rulesProductCandidate{}, false
		}
	}

	confidence := rulesConfidence(block.Score, policy.CandidateScoreThreshold, name, brand, jan, sku, model, hasPrice, capacity, size, color)
	sourceText := truncateRunes(text, rulesSourceTextLimit)
	attributes := map[string]any{
		"schemaVersion":           1,
		"extractor":               "rules",
		"rulesScore":              block.Score,
		"rulesCandidateThreshold": policy.CandidateScoreThreshold,
		"rulesBlockRunes":         len([]rune(text)),
		"rulesBlockPageNumber":    block.PageNumber,
		"rulesBlockStartLine":     block.StartLine,
		"nameDerivedFrom":         nameSource,
		"priceExtractionEnabled":  policy.PriceExtractionEnabled,
		"rulesMaxBlockRunes":      policy.MaxBlockRunes,
		"rulesContextWindowRunes": policy.ContextWindowRunes,
	}
	if size != "" {
		attributes["size"] = size
	}
	if color != "" {
		attributes["color"] = color
	}
	if capacity != "" {
		attributes["capacity"] = capacity
	}

	item := DriveProductExtractionItem{
		TenantID:     input.TenantID,
		FilePublicID: input.File.PublicID,
		ItemType:     "product",
		Name:         name,
		Brand:        brand,
		Manufacturer: manufacturer,
		Model:        model,
		SKU:          sku,
		JANCode:      jan,
		Category:     category,
		Description:  rulesDescription(text, name),
		Price:        rulesPriceJSON(price, hasPrice),
		Promotion:    rulesPromotionJSON(text),
		Availability: map[string]any{},
		SourceText:   sourceText,
		Evidence: []map[string]any{{
			"pageNumber": block.PageNumber,
			"text":       sourceText,
		}},
		Attributes: attributes,
		Confidence: &confidence,
		CreatedAt:  time.Now(),
	}
	return rulesProductCandidate{item: item, key: rulesDedupKey(item)}, true
}

func rulesProductName(text, jan string, hasPrice bool) (string, string) {
	if value := rulesCleanProductName(rulesLabelValue(text, rulesProductNameLabels)); value != "" {
		return value, "label"
	}
	lines := ocrCandidateLines(text)
	for i, line := range lines {
		if jan != "" && strings.Contains(line, jan) {
			if name := rulesNearbyName(lines, i); name != "" {
				return name, "nearby"
			}
		}
		if hasPrice && rulesPricePattern.MatchString(line) {
			if name := rulesNearbyName(lines, i); name != "" {
				return name, "nearby"
			}
		}
	}
	for _, line := range lines {
		if name := rulesCleanProductName(line); name != "" {
			return name, "heading"
		}
	}
	return "", ""
}

func rulesNearbyName(lines []string, index int) string {
	if name := rulesCleanProductName(lines[index]); name != "" {
		return name
	}
	for i := index - 1; i >= 0 && i >= index-3; i-- {
		if name := rulesCleanProductName(lines[i]); name != "" {
			return name
		}
	}
	for i := index + 1; i < len(lines) && i <= index+2; i++ {
		if name := rulesCleanProductName(lines[i]); name != "" {
			return name
		}
	}
	return ""
}

func rulesCleanProductName(value string) string {
	value = rulesPricePattern.ReplaceAllString(value, "")
	value = rulesJANPattern.ReplaceAllString(value, "")
	value = rulesRemoveLabeledSegments(value)
	value = strings.Trim(value, " -:：|/　\t")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || rulesHeadingNoisePattern.MatchString(value) || rulesContainsAny(value, rulesNegativeTerms) {
		return ""
	}
	runes := []rune(value)
	if len(runes) > 120 {
		return ""
	}
	return value
}

func rulesLabelValue(text string, labels []string) string {
	for _, line := range ocrCandidateLines(text) {
		for _, label := range labels {
			idx := strings.Index(strings.ToLower(line), strings.ToLower(label))
			if idx < 0 {
				continue
			}
			value := strings.TrimSpace(line[idx+len(label):])
			value = strings.TrimLeft(value, " :：-=/")
			value = rulesTrimBeforeNextLabel(value)
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func rulesTrimBeforeNextLabel(value string) string {
	cut := len(value)
	lower := strings.ToLower(value)
	for _, label := range rulesAllFieldLabels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx > 0 && idx < cut {
			before := strings.TrimSpace(value[:idx])
			if before != "" {
				cut = idx
			}
		}
	}
	return strings.TrimSpace(value[:cut])
}

func rulesRemoveLabeledSegments(value string) string {
	for _, label := range rulesAllFieldLabels {
		for {
			lower := strings.ToLower(value)
			idx := strings.Index(lower, strings.ToLower(label))
			if idx < 0 {
				break
			}
			value = strings.TrimSpace(value[:idx])
		}
	}
	return value
}

func rulesCleanFieldValue(value string) string {
	value = rulesPricePattern.ReplaceAllString(value, "")
	value = rulesJANPattern.ReplaceAllString(value, "")
	value = strings.Trim(value, " -:：|/　\t")
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > 120 {
		return ""
	}
	return value
}

func rulesExtractCodeValue(text string, labels []string) string {
	value := rulesCleanFieldValue(rulesLabelValue(text, labels))
	if value == "" {
		return ""
	}
	if match := rulesModelValuePattern.FindString(value); match != "" {
		return match
	}
	return value
}

func rulesExtractPrice(text string, enabled bool) (rulesPriceMatch, bool) {
	if !enabled {
		return rulesPriceMatch{}, false
	}
	match := rulesPricePattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return rulesPriceMatch{}, false
	}
	value := ""
	for _, part := range match[2:] {
		if strings.TrimSpace(part) != "" {
			value = strings.ReplaceAll(part, ",", "")
			break
		}
	}
	amount, err := strconv.Atoi(value)
	if err != nil {
		return rulesPriceMatch{}, false
	}
	return rulesPriceMatch{
		Amount:      amount,
		TaxIncluded: strings.Contains(match[0], "税込"),
		Raw:         strings.TrimSpace(match[0]),
	}, true
}

func rulesPriceJSON(price rulesPriceMatch, ok bool) map[string]any {
	if !ok {
		return map[string]any{}
	}
	return map[string]any{
		"amount":      price.Amount,
		"currency":    "JPY",
		"taxIncluded": price.TaxIncluded,
		"source":      price.Raw,
	}
}

func rulesPromotionJSON(text string) map[string]any {
	for _, label := range []string{"特価", "セール", "割引", "ポイント"} {
		if strings.Contains(text, label) {
			return map[string]any{"label": label}
		}
	}
	return map[string]any{}
}

func rulesDescription(text, name string) string {
	for _, line := range ocrCandidateLines(text) {
		if line == name || rulesContainsAny(line, rulesNegativeTerms) || rulesLineHasAnchor(line, true) {
			continue
		}
		if len([]rune(line)) >= 24 {
			return truncateRunes(line, 300)
		}
	}
	return ""
}

func rulesConfidence(score, threshold int, name, brand, jan, sku, model string, hasPrice bool, capacity, size, color string) float64 {
	confidence := 0.05
	if jan != "" {
		confidence += 0.25
	}
	if hasPrice {
		confidence += 0.15
	}
	if name != "" {
		confidence += 0.25
	}
	if brand != "" {
		confidence += 0.10
	}
	if sku != "" || model != "" {
		confidence += 0.10
	}
	if capacity != "" || size != "" || color != "" {
		confidence += 0.10
	}
	if score >= threshold+4 {
		confidence += 0.15
	} else if score >= threshold {
		confidence += 0.07
	}
	return clampConfidence(confidence)
}

func rulesDedupKey(item DriveProductExtractionItem) string {
	if item.JANCode != "" {
		return "jan:" + item.JANCode
	}
	if item.SKU != "" {
		return "sku:" + strings.ToLower(item.SKU)
	}
	if item.Model != "" {
		return "model:" + strings.ToLower(item.Model)
	}
	if item.Name != "" && item.Brand != "" {
		return "name-brand:" + rulesNormalizeDedupText(item.Name+"|"+item.Brand)
	}
	if item.Name != "" {
		if amount := rulesPriceAmount(item.Price); amount != "" {
			return "name-price:" + rulesNormalizeDedupText(item.Name+"|"+amount)
		}
	}
	return "name:" + rulesNormalizeDedupText(item.Name)
}

func mergeRulesProductCandidates(candidates []rulesProductCandidate) []DriveProductExtractionItem {
	items := make([]DriveProductExtractionItem, 0, len(candidates))
	indexByKey := map[string]int{}
	for _, candidate := range candidates {
		if candidate.key == "" {
			items = append(items, candidate.item)
			continue
		}
		if index, ok := indexByKey[candidate.key]; ok {
			items[index] = mergeRulesProductItem(items[index], candidate.item)
			continue
		}
		indexByKey[candidate.key] = len(items)
		items = append(items, candidate.item)
	}
	return items
}

func mergeRulesProductItem(existing, incoming DriveProductExtractionItem) DriveProductExtractionItem {
	existingConfidence := confidenceValue(existing.Confidence)
	incomingConfidence := confidenceValue(incoming.Confidence)
	preferIncoming := incomingConfidence > existingConfidence
	if preferIncoming {
		existing.SourceText = incoming.SourceText
		existing.Evidence = incoming.Evidence
	}
	existing.Name = mergeRulesName(existing, incoming, preferIncoming)
	existing.Brand = mergeStringField(existing.Brand, incoming.Brand, preferIncoming)
	existing.Manufacturer = mergeStringField(existing.Manufacturer, incoming.Manufacturer, preferIncoming)
	existing.Model = mergeStringField(existing.Model, incoming.Model, preferIncoming)
	existing.SKU = mergeStringField(existing.SKU, incoming.SKU, preferIncoming)
	existing.JANCode = mergeStringField(existing.JANCode, incoming.JANCode, preferIncoming)
	existing.Category = mergeStringField(existing.Category, incoming.Category, preferIncoming)
	existing.Description = mergeStringField(existing.Description, incoming.Description, preferIncoming)
	if len(existing.Price) == 0 || (len(incoming.Price) > 0 && preferIncoming) {
		existing.Price = incoming.Price
	}
	if len(existing.Promotion) == 0 || (len(incoming.Promotion) > 0 && preferIncoming) {
		existing.Promotion = incoming.Promotion
	}
	if existing.Attributes == nil {
		existing.Attributes = map[string]any{}
	}
	for key, value := range incoming.Attributes {
		if key == "nameDerivedFrom" && existing.Name != incoming.Name {
			continue
		}
		if _, ok := existing.Attributes[key]; !ok || preferIncoming {
			existing.Attributes[key] = value
		}
	}
	if incomingConfidence > existingConfidence {
		value := incomingConfidence
		existing.Confidence = &value
	}
	return existing
}

func mergeRulesName(existing, incoming DriveProductExtractionItem, preferIncoming bool) string {
	if strings.TrimSpace(existing.Name) == "" {
		return incoming.Name
	}
	if strings.TrimSpace(incoming.Name) == "" {
		return existing.Name
	}
	existingPriority := rulesNameSourcePriority(existing.Attributes["nameDerivedFrom"])
	incomingPriority := rulesNameSourcePriority(incoming.Attributes["nameDerivedFrom"])
	if incomingPriority > existingPriority || (incomingPriority == existingPriority && preferIncoming) {
		return incoming.Name
	}
	return existing.Name
}

func rulesNameSourcePriority(value any) int {
	source, _ := value.(string)
	switch strings.TrimSpace(source) {
	case "label":
		return 5
	case "nearby":
		return 4
	case "heading":
		return 3
	case "model", "sku":
		return 2
	case "janCode":
		return 1
	default:
		return 0
	}
}

func mergeStringField(existing, incoming string, preferIncoming bool) string {
	if strings.TrimSpace(existing) == "" {
		return incoming
	}
	if strings.TrimSpace(incoming) != "" && preferIncoming {
		return incoming
	}
	return existing
}

func confidenceValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func rulesPriceAmount(price map[string]any) string {
	if price == nil {
		return ""
	}
	switch value := price["amount"].(type) {
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case string:
		return value
	default:
		return ""
	}
}

func rulesNormalizeDedupText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), "")
	return value
}

func rulesLooksLikeDescription(text string) bool {
	for _, line := range ocrCandidateLines(text) {
		if len([]rune(line)) >= 40 && strings.ContainsAny(line, "。、ですます") {
			return true
		}
	}
	return false
}

func rulesNounishTokenCount(text string) int {
	count := 0
	inToken := false
	for _, r := range text {
		isToken := unicode.Is(unicode.Han, r) || unicode.Is(unicode.Katakana, r) || unicode.IsLetter(r) || unicode.IsDigit(r)
		if isToken && !inToken {
			count++
			inToken = true
			continue
		}
		if !isToken {
			inToken = false
		}
	}
	return count
}

func rulesContainsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func appendStringSlices(values ...[]string) []string {
	total := 0
	for _, value := range values {
		total += len(value)
	}
	out := make([]string, 0, total)
	for _, value := range values {
		out = append(out, value...)
	}
	return out
}
