package contentengine

import (
	"strings"
	"testing"
)

func TestTemplatePlatformsAndMissingFacts(t *testing.T) {
	in := Input{CatalogTitle: "折叠收纳盒", Category: "家居收纳", SKUs: []SKU{{Code: "W-S", Name: "白色 / 小号"}}}
	x, _ := ProfileFor("xianyu")
	taobao, _ := ProfileFor("taobao")
	xout := GenerateTemplate(in, x)
	tout := GenerateTemplate(in, taobao)
	if xout.Title == "" || tout.Title == "" || len(xout.OmittedInformation) == 0 {
		t.Fatal("platform templates or missing facts not generated")
	}
	if strings.Contains(xout.Description, "航空铝合金") || strings.Contains(xout.Description, "当天发货") {
		t.Fatal("template fabricated facts")
	}
}

func TestGuardBlocksFabricatedFactsAndWarnsBrand(t *testing.T) {
	p, _ := ProfileFor("taobao")
	in := Input{CatalogTitle: "手机支架"}
	out, err := ParseAI(`{"title":"Apple官方正品支架","description":"航空铝合金，国家认证，当天发货","sellingPoints":["疗效保证"],"keywords":["Apple"]}`, in, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Blockers) < 3 || len(out.Warnings) == 0 {
		t.Fatalf("expected blockers/warnings: %#v", out)
	}
}

func TestInvalidAIJSONFallsBackAtServiceBoundary(t *testing.T) {
	p, _ := ProfileFor("xianyu")
	if _, err := ParseAI("not-json", Input{}, p); err == nil {
		t.Fatal("expected invalid json")
	}
}

func TestSKUFormattingDoesNotCreateCombinations(t *testing.T) {
	p, _ := ProfileFor("xianyu")
	in := Input{CatalogTitle: "杯子", SKUs: []SKU{{Code: "A", Name: "白色 / 小号"}, {Code: "B", Name: "黑色 / 大号"}}}
	out := GenerateTemplate(in, p)
	if len(out.SKUContent) != 2 {
		t.Fatalf("expected 2 exact skus, got %d", len(out.SKUContent))
	}
}
