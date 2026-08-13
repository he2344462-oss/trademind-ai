package contentengine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	ModeTemplate = "template_only"
	ModeAI       = "ai_generate"
)

type Profile struct {
	Platform, Version, PromptVersion, Style                      string
	TitleMax, DescriptionMax, MinSellingPoints, MaxSellingPoints int
	AllowEmoji, AllowCTA                                         bool
	BlockedPhrases, WarningPhrases                               []string
}

func ProfileFor(platform string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "xianyu":
		return Profile{Platform: "xianyu", Version: "xianyu-content-v1", PromptVersion: "xianyu-v1", Style: "natural_concise", TitleMax: 60, DescriptionMax: 3000, MinSellingPoints: 3, MaxSellingPoints: 6, AllowEmoji: false, AllowCTA: false, BlockedPhrases: []string{"我自己用了", "老婆买多了", "搬家闲置", "99新自用", "官方正品", "当天发货", "国家认证"}}, nil
	case "taobao":
		return Profile{Platform: "taobao", Version: "taobao-content-v1", PromptVersion: "taobao-v1", Style: "structured_search", TitleMax: 60, DescriptionMax: 5000, MinSellingPoints: 3, MaxSellingPoints: 6, AllowEmoji: false, AllowCTA: false, BlockedPhrases: []string{"销量第一", "全网最低", "官方正品", "当天发货", "国家认证"}}, nil
	default:
		return Profile{}, fmt.Errorf("unsupported platform")
	}
}

type SKU struct {
	Code, Name string
	Attributes map[string]any
}
type Input struct {
	CatalogTitle, SourceTitle, Description, Category, Supplier, SourceURL string
	Images                                                                []string
	SKUs                                                                  []SKU
	Facts                                                                 map[string]string
}
type Output struct {
	Title              string              `json:"title"`
	Description        string              `json:"description"`
	SellingPoints      []string            `json:"sellingPoints"`
	Keywords           []string            `json:"keywords"`
	FAQ                []map[string]string `json:"faq"`
	SKUContent         []map[string]any    `json:"skuContent"`
	Warnings           []string            `json:"warnings"`
	Blockers           []string            `json:"blockers"`
	IncludedKeywords   []string            `json:"includedKeywords"`
	OmittedInformation []string            `json:"omittedInformation"`
}

func GenerateTemplate(in Input, p Profile) Output {
	title := first(in.CatalogTitle, in.SourceTitle, "待补充商品标题")
	title = truncate(strings.Join(strings.Fields(title), " "), p.TitleMax)
	points := make([]string, 0, 6)
	if in.Category != "" {
		points = append(points, "商品类目："+in.Category)
	}
	if len(in.SKUs) > 1 {
		points = append(points, fmt.Sprintf("提供 %d 个真实规格组合可选", len(in.SKUs)))
	}
	for _, key := range sortedKeys(in.Facts) {
		if v := strings.TrimSpace(in.Facts[key]); v != "" {
			points = append(points, key+"："+v)
		}
		if len(points) >= p.MaxSellingPoints {
			break
		}
	}
	if strings.TrimSpace(in.Description) != "" {
		points = append(points, "商品信息已根据货源资料整理")
	}
	for len(points) < p.MinSellingPoints {
		points = append(points, "具体参数请以下方已确认规格为准")
	}
	if len(points) > p.MaxSellingPoints {
		points = points[:p.MaxSellingPoints]
	}
	sections := []string{"商品说明\n" + first(strings.TrimSpace(in.Description), "商品详情待人工补充。"), "核心信息\n- " + strings.Join(points, "\n- ")}
	if len(in.SKUs) > 0 {
		sections = append(sections, "规格说明\n"+formatSKUs(in.SKUs))
	}
	sections = append(sections, "发货与售后\n请运营人员依据实际库存、发货地和售后政策补充，不作未验证承诺。")
	out := Output{Title: title, Description: truncate(strings.Join(sections, "\n\n"), p.DescriptionMax), SellingPoints: points, Keywords: keywords(in), FAQ: []map[string]string{{"question": "有哪些规格？", "answer": first(formatSKUs(in.SKUs), "规格待补充")}}, SKUContent: skuRows(in.SKUs)}
	return Guard(in, p, out)
}

var absoluteRisk = regexp.MustCompile(`(?i)(最[佳好低高]|第一|百分之百|100%|绝对|保证|必[赚卖]|国家级|治愈|疗效|根治|官方正品|当天发货|微信|vx|手机号)`)
var brandRisk = regexp.MustCompile(`(?i)(nike|adidas|apple|华为|耐克|阿迪达斯|苹果)`)

func Guard(in Input, p Profile, out Output) Output {
	joined := strings.Join(append([]string{out.Title, out.Description}, append(out.SellingPoints, out.Keywords...)...), "\n")
	for _, phrase := range p.BlockedPhrases {
		if strings.Contains(joined, phrase) {
			out.Blockers = appendUnique(out.Blockers, "未提供依据的内容："+phrase)
		}
	}
	if absoluteRisk.MatchString(joined) {
		out.Warnings = appendUnique(out.Warnings, "内容包含绝对化、功效、承诺或站外引流风险词，请人工核验")
	}
	if brandRisk.MatchString(joined) {
		out.Warnings = appendUnique(out.Warnings, "内容包含品牌词，仅作为风险提示，请核验授权或合理使用依据")
	}
	known := knownText(in)
	for k, v := range map[string]string{"材质": "航空铝合金", "认证": "国家认证", "物流": "当天发货", "正品": "官方正品"} {
		if strings.Contains(joined, v) && !strings.Contains(known, v) {
			out.Blockers = appendUnique(out.Blockers, "生成内容包含输入中不存在的"+k+"事实："+v)
		}
	}
	for _, missing := range []string{"材质", "尺寸", "品牌", "重量", "发货地"} {
		if strings.TrimSpace(in.Facts[missing]) == "" {
			out.OmittedInformation = append(out.OmittedInformation, missing+"待补充")
		}
	}
	out.Title = truncate(strings.TrimSpace(out.Title), p.TitleMax)
	out.Description = truncate(strings.TrimSpace(out.Description), p.DescriptionMax)
	if len(out.SellingPoints) > p.MaxSellingPoints {
		out.SellingPoints = out.SellingPoints[:p.MaxSellingPoints]
	}
	out.Keywords = uniqueSafe(out.Keywords)
	return out
}

func ParseAI(raw string, in Input, p Profile) (Output, error) {
	var out Output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return Output{}, fmt.Errorf("invalid ai json: %w", err)
	}
	if strings.TrimSpace(out.Title) == "" || strings.TrimSpace(out.Description) == "" {
		return Output{}, fmt.Errorf("ai result missing title or description")
	}
	return Guard(in, p, out), nil
}

func BuildPrompt(in Input, p Profile) string {
	b, _ := json.Marshal(in)
	return "用途=product_content。只使用输入JSON中的事实，不得推测材质、品牌、认证、功效、库存、发货地、物流承诺或个人使用经历。输出JSON字段 title,description,sellingPoints,keywords,faq,skuContent。平台规则=" + p.Version + "。输入=" + string(b)
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}
func first(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func sortedKeys(m map[string]string) []string {
	x := make([]string, 0, len(m))
	for k := range m {
		x = append(x, k)
	}
	sort.Strings(x)
	return x
}
func formatSKUs(s []SKU) string {
	rows := make([]string, 0, len(s))
	for _, v := range s {
		name := first(v.Name, v.Code)
		if name != "" {
			rows = append(rows, name)
		}
	}
	return strings.Join(rows, " / ")
}
func skuRows(s []SKU) []map[string]any {
	out := make([]map[string]any, 0, len(s))
	for _, v := range s {
		out = append(out, map[string]any{"code": v.Code, "displayName": first(v.Name, v.Code), "attributes": v.Attributes})
	}
	return out
}
func keywords(in Input) []string {
	out := strings.Fields(strings.NewReplacer("，", " ", ",", " ", "/", " ", "|", " ").Replace(in.CatalogTitle + " " + in.Category))
	return uniqueSafe(out)
}
func uniqueSafe(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] || brandRisk.MatchString(v) || absoluteRisk.MatchString(v) {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) >= 20 {
			break
		}
	}
	return out
}
func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}
func knownText(in Input) string { b, _ := json.Marshal(in); return string(b) }
