package service

import (
	"strings"
	"testing"
)

func TestRulesDriveProductExtractorExtractsIDEAFields(t *testing.T) {
	extractor := NewRulesDriveProductExtractor()
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		File: DriveFile{
			PublicID: "file-public-id",
		},
		FullText: `<section>
商品名：ＡＧＦ ブレンディ スティック カフェオレ １００本
ブランド：ＡＧＦ
メーカー：味の素AGF
JANコード：４９０１１１１１１１１１１
SKU：BLS-100
型番：CAF-100
カテゴリ：飲料
内容量：１００本
サイズ：M
カラー：ブラック
税込 １,２８０円
まろやかな味わいのスティックタイプのカフェオレです。毎日の休憩に使いやすい大容量です。
</section>`,
		Policy: defaultDriveOCRPolicy(),
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Name != "AGF ブレンディ スティック カフェオレ 100本" {
		t.Fatalf("Name = %q", item.Name)
	}
	if item.Brand != "AGF" {
		t.Fatalf("Brand = %q, want AGF", item.Brand)
	}
	if item.Manufacturer != "味の素AGF" {
		t.Fatalf("Manufacturer = %q, want 味の素AGF", item.Manufacturer)
	}
	if item.JANCode != "4901111111111" {
		t.Fatalf("JANCode = %q, want 4901111111111", item.JANCode)
	}
	if item.SKU != "BLS-100" {
		t.Fatalf("SKU = %q, want BLS-100", item.SKU)
	}
	if item.Model != "CAF-100" {
		t.Fatalf("Model = %q, want CAF-100", item.Model)
	}
	if item.Category != "飲料" {
		t.Fatalf("Category = %q, want 飲料", item.Category)
	}
	if item.Price["amount"] != 1280 {
		t.Fatalf("price amount = %#v, want 1280", item.Price["amount"])
	}
	if item.Attributes["capacity"] != "100本" {
		t.Fatalf("capacity attribute = %#v, want 100本", item.Attributes["capacity"])
	}
	if item.Attributes["size"] != "M" {
		t.Fatalf("size attribute = %#v, want M", item.Attributes["size"])
	}
	if item.Attributes["color"] != "ブラック" {
		t.Fatalf("color attribute = %#v, want ブラック", item.Attributes["color"])
	}
	if item.Attributes["nameDerivedFrom"] != "label" {
		t.Fatalf("nameDerivedFrom = %#v, want label", item.Attributes["nameDerivedFrom"])
	}
	if item.Confidence == nil || *item.Confidence <= 0.8 {
		t.Fatalf("Confidence = %#v, want > 0.8", item.Confidence)
	}
}

func TestRulesProductBlockScoring(t *testing.T) {
	negative := scoreRulesProductBlock("会社概要\n利用規約\nお問い合わせ\n送料と返品について", true)
	positive := scoreRulesProductBlock("商品名: テスト商品\nJANコード: 4901111111111\n価格: 1,280円\n内容量: 500g\nメーカー: Example", true)

	if negative >= 0 {
		t.Fatalf("negative score = %d, want below 0", negative)
	}
	if positive < 10 {
		t.Fatalf("positive score = %d, want high product score", positive)
	}
}

func TestRulesDriveProductExtractorMergesDuplicateJAN(t *testing.T) {
	extractor := NewRulesDriveProductExtractor()
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		File: DriveFile{
			PublicID: "file-public-id",
		},
		FullText: `商品名: AGF ブレンディ カフェオレ
JANコード: 4901111111111
価格: 1,280円

JANコード: 4901111111111
ブランド: AGF
内容量: 100本
まろやかな味わいのスティックタイプのカフェオレです。`,
		Policy: defaultDriveOCRPolicy(),
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Name != "AGF ブレンディ カフェオレ" {
		t.Fatalf("Name = %q, want label-derived product name", item.Name)
	}
	if item.Brand != "AGF" {
		t.Fatalf("Brand = %q, want merged AGF", item.Brand)
	}
	if item.Attributes["capacity"] != "100本" {
		t.Fatalf("capacity attribute = %#v, want merged 100本", item.Attributes["capacity"])
	}
}

func TestRulesDriveProductExtractorHandlesLargeInput(t *testing.T) {
	noise := strings.Repeat("会社概要 お問い合わせ 利用規約\n", 10000)
	text := noise + "\n商品名: 大容量テスト商品\nJANコード: 4902222222222\n価格: 2,980円\n内容量: 500g\n"
	extractor := NewRulesDriveProductExtractor()
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		FullText: text,
		Policy:   defaultDriveOCRPolicy(),
		File:     DriveFile{PublicID: "file-public-id"},
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if len([]rune(text)) < 140000 {
		t.Fatalf("test setup has %d runes, want at least 140000", len([]rune(text)))
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	if result.Items[0].JANCode != "4902222222222" {
		t.Fatalf("JANCode = %q, want 4902222222222", result.Items[0].JANCode)
	}
}
