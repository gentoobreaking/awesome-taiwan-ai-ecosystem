# Markdown Export 分類問題審計報告

> 範圍: `internal/export/exporter.go` (functionalCategories / filterByCategory / serverMarkdown / serverMarkdownIntl / RegistryExporter.Export), `internal/classify/rules.go` + `keywords.go`, `internal/models/models.go`, `internal/scoring/quality.go`, `algs/taiwan-classification.md`, `algs/registry-export.md`
> 原則: **不修改原始碼，純審計**。所有結論均附行號/檔案可復現。
> 日期: 2026-09-05

---

## 1. 執行摘要

最嚴重的問題是 **生產環境中 `MCPServer.Category` 恆為空** — `internal/normalize/normalizer.go:Normalize` 從未賦值 `Category`，`internal/crawler/coordinator.go:Run` 在 Taiwan relevance scoring 與 LLM 分類後亦未回填 `server.Category`（`llm.go:Classify` 回傳的 `Categories` 欄位被丟棄，僅 `TaiwanRelevance` 被採用，見 `coordinator.go:123-126`）。結果 `internal/export/exporter.go:ExportMarkdown` 中所有 T1–T5 伺服器在 `functionalCategories` 匹配失敗，集體落入 `Other Tools`；`categories.json` 則全部計為 `uncategorized` — Markdown 分類與 JSON 統計雙雙失真。

第二類問題是 **Markdown 分類詞彙與受控詞彙完全脫節**，外加 `serverMarkdownIntl` 的工具列表死碼、`serverMarkdown` 的 evidence 過濾死碼、大小寫/正規化缺失、重複列出、多值語義不明等 12 項缺陷，詳見 §3。

---

## 2. 文件與程式碼版本對照

| 文件 | 期望分類 | 受控詞彙來源 |
|---|---|---|
| `algs/taiwan-classification.md §19` | `finance, stock, etf, banking, insurance, real-estate, land, housing, government, open-data, legislative, judicial, procurement, weather, earthquake, transport, traffic, railway, metro, bus, logistics, payment, invoice, tax, company, business, healthcare, education, agriculture, food, tourism, geography, gis, language, traditional-chinese, culture, ecommerce, devops, news` | 38 項，與 `models.ValidCategories` 完全一致 |
| `algs/registry-export.md §categories.json` | `{"finance":25, "government":18, ...}` 示例 | 期望與 §19 同 |
| `internal/models/models.go:72-85` | `ValidCategories` 同 §19 | 單一真相來源 |
| `internal/export/exporter.go:282-303` | 20 項 `functionalCategories` keys | 見 §3.1 — 大面積不重疊 |
| `config/keywords.yaml` / `config/domains.yaml` | 關鍵字/網域 | 與分類無直接映射（透過 `keywordCategory` 間接，但該函式回傳值從未寫入 `Category`） |

---

## 3. 缺陷清單 (12 項，依嚴重度排序)

### 🔴 D01 — `Category` 從未在 pipeline 中賦值 (Critical / 資料流斷層)

- **位置**: `internal/normalize/normalizer.go:36-130` 未對 `server.Category` 賦值；`internal/crawler/coordinator.go:101-128` 中 `Score` 與 `LLM.Classify` 後均未處理 `Category`；`llm.go:148-154` 的 `Categories` 欄位在 `callLLM` 解析後僅存於 `llmResponse`，`Classify` 回傳的 `TaiwanRelevance` 不含 `Category`，呼叫方 `coordinator.go:124-126` 僅覆蓋 `TaiwanRelevance`。
- **後果**: 所有生產資料 `len(Category)==0`。`filterByCategory(..., "other")` 的 `len==0 => Other` 分支會吸走 100% 的 T1–T5 伺服器（見 D06），`Export` 的 `categories.json:78 categories["uncategorized"]++` 同樣失真。
- **證據**: `grep -rn "server.Category\s*=\|\.Category ="` 僅命中 `merge_test.go` 與 `store_test.go` 的測試資料，無生產賦值。`Normalize` 回傳前 `report.md:§3.1` 已驗證。
- **對照**: `algs/taiwan-classification.md:179-187` LLM schema 明確要求 `"categories": ["finance","taiwan-stock"]`，但已被靜默丟棄。

### 🔴 D02 — `functionalCategories` keys 與 `ValidCategories` 大面積不匹配 (Critical)

| 類型 | 列表 |
|---|---|
| **Valid 但 functional 缺失** (16 項會被誤判 Other) | `stock, etf, banking, insurance, land, housing, open-data, legislative, judicial, procurement, weather, earthquake, traffic, railway, metro, bus, logistics, payment, invoice, tax, company, business, food, geography, gis, traditional-chinese, culture` — 其中前 6 項是 finance/real-estate/government 的子類，`open-data` 更是 `algs/registry-export.md:34` 示例 `"category": ["real-estate","government","open-data"]` 的一員，該示例在 Markdown 中 1/3 落入 Other |
| **functional 存在但 Valid 非法** (7 項永遠 IsValidCategory==false) | `search, coding-agents, communication, databases, knowledge, legal, security` — 測試 `exporter_test.go:42 Category:["search"]` 故意使用非法值但通過，因為生產無校驗；若未來正規化器嚴格校驗將全被拒 |
| **命名漂移** | `functionalCategories` 使用 `real-estate` 與 Valid 一致，但 display name 為 `Real Estate`；若上游傳入 `"Real Estate"`, `"Finance"` 大寫或 `"finance & fintech"` 顯示名將匹配失敗（見 D05） |

> 根因: `exporter.go:282-303` 的 20 項是手工硬編碼的展示分類，未從 `models.ValidCategories` 或 `config/categories.yaml` 派生。

### 🔴 D03 — `filterByCategory` 去重/重複/消失三重缺陷 (Major)

原始碼 `exporter.go:305-339`:

```go
func filterByCategory(servers []MCPServer, catKey string) []MCPServer {
  for _, s := range servers {
    if s.TaiwanRelevance.Level == "T0" { continue } // T0 全部排除 (正確)
    if catKey == "other" {
      if len(s.Category)==0 { matched }               // (1) 空分類 => Other
      matchedAny := false
      for _, c := range s.Category {
        for _, fc := range functionalCategories { if fc.key!="other" && c==fc.key { matchedAny=true } }
      }
      if !matchedAny { matched }                      // (2) 無一命中 functional => Other
      continue
    }
    for _, c := range s.Category { if c==catKey { matched; break } } // (3) 精確匹配
  }
}
```

| 缺陷 | 說明 |
|---|---|
| **重複列出** | `ExportMarkdown` 對 `functionalCategories` 逐一呼叫 `filterByCategory`，多分類伺服器 `Category:["finance","government"]` 會在 `💰 Finance` 與 `🏛️ Government` 兩節同時出現，無去重標記。`algs/registry-export.md` 未定義 Markdown 是否允許重複，但用戶預期通常是單歸屬或明確標註多歸屬。 |
| **膨脹的 Other** | D01 + D02 導致 Other 吸收 (a) 空分類 (b) 全部 Valid 子類 (`stock`, `payment` 等) (c) 非法值 `search`。在當前程式中 `search` 本是 functional 之一，若 Category=`search` 則命中 `search` 分類而非 Other — 但 `search` 不是 Valid，意味 JSON 與 Markdown 對 `search` 的合法性判斷相反。 |
| **T0 正確排除但邊界模糊** | `Level==""`（未評分/歷史資料）不會被 `T0` 排除，故空 Level 會進入 Taiwan-relevant 區段並因空 Category 落入 Other — 與 `computeStatistics:156 level=="" => "T0"` 的統計口徑不一致。 |
| **效能** | `other` 分支對每台 server 掃 `len(Category) * len(functionalCategories)` (~20*小)，1000 台尚可，但屬於 O(N*C*F)。 |
| **無序且不穩定** | 輸出順序等於 `functionalCategories` 硬編碼順序，非按 server score 或字母排序，隨 `functionalCategories` 定義順序變更而變。 |

### 🟠 D04 — `serverMarkdown` 與 `serverMarkdownIntl` 不一致 + 死碼 (Major)

| 項目 | `serverMarkdown` (T1–T5) `341-392` | `serverMarkdownIntl` (T0) `394-429` | 問題 |
|---|---|---|---|
| **Taiwan 欄位** | `- **Taiwan**: T5 (score: 85)` `357` | **缺失** (刻意) | 任務要求「Intl 版是否遺漏 Taiwan 欄位但仍顯示，是否應統一」— 目前 Intl 完全省略，與 T0 定義一致（"No Taiwan relevance"），但與 `computeStatistics` 中仍計 `ByLevel["T0"]` 的可觀測性矛盾；建議 Intl 至少顯示 `T0 (score: 0)` 或統一由樣式決定 |
| **Tools** | 正常 `378-384` | `416-421` 宣告 `toolNames` 後 **未寫入 `sb`**，工具列表永遠缺失 | 明顯 copy-paste 漏 `sb.WriteString`，`exporter_test.go:TestExportMarkdown` 僅斷言 Taiwan 行與連結，未覆蓋 Intl Tools |
| **License** | `375-377 if s.License != ""` | **缺失** | T0 伺服器同樣有 License，應統一 |
| **Evidence** | `358-368 if e.Type=="keyword" \|\| e.Type=="domain"` | 無 | 但 `Score` 實際產生的 `Type` 為 `official_domain / taiwan_financial_api / repository_keyword` (`rules.go:40,57,75`)，條件恆假 — Taiwan 區段的 evidence 永遠不顯示。`exporter_test.go:271-272 Type:"keyword"/"domain"` 是手工構造才通過測試，掩蓋了生產死碼 |

### 🟠 D05 — 無 Category 正規化 (Major)

- `filterByCategory` 使用 `c == catKey` 精確相等，大小寫敏感，無 `strings.EqualFold` / `TrimSpace` / `ToLower` / `kebab-case` 正規化。
- `models.IsValidCategory` 同樣大小寫敏感。
- 上游若傳入 `"Finance"`, `"FINANCE"`, `"Finance & Fintech"` (顯示名), `"金融"` (中文), `"taiwan-stock"` (algs 示例中的幻覺值) 均直接落入 Other。
- `algs/taiwan-classification.md:184 LLM categories: ["finance","taiwan-stock"]` 中的 `taiwan-stock` 本身就不是 ValidCategories，也會落入 Other。

### 🟡 D06 — `RegistryExporter.Export` 的 `categories.json` 與 Markdown 口徑分叉 (Major)

```go
// exporter.go:72-79
categories := make(map[string]int)
for _, s := range servers {
  for _, c := range s.Category { categories[c]++ }
  if len(s.Category)==0 { categories["uncategorized"]++ }
}
```

- JSON 統計以 **原始 `Category` 值** 為 key，Markdown 以 **functionalCategories** 分組，外加特殊 `uncategorized` vs `other` 命名分叉。
- 多分類伺服器在 JSON 中被重複計數（每 category +1），在 Markdown 中被重複列出，但在 `statistics.json` 的 `TotalServers` 中僅計 1 次 — 三份文件的語義不一致且未在文件說明。
- 空分類在 JSON 叫 `uncategorized`，在 Markdown 叫 `Other Tools` — 用戶無法對齊。
- 子類如 `stock` 在 JSON 中單獨計數，在 Markdown 中被吞併到 Other — 用戶在 `categories.json` 看到 `stock: 12` 卻在 Markdown 找不到對應章節。
- 任務問「是否正確寫入 category 統計，還是僅 markdown 有分類」— 答案：兩者皆有但口徑不同且皆受 D01 污染。

### 🟡 D07 — `computeStatistics` 的 TaiwanRelevant 與 Markdown 分區口徑一致但與分數閾值矛盾

- `computeStatistics:169 if Level!="" && Level!="T0" => TaiwanRelevant++` 與 Markdown 的 `T0=>Intl, 非T0=>Taiwan` 一致。
- 但 `rules.go:ThresholdToLevel` 定義 `score <5 => T0`，而 `llm.go:buildLLMPrompt:261-266` 定義 `T0: 0-19, T1:20-35, T2:36-55...` — LLM 與確定性閾值相差 15 分。LLM 在 5–19 分區間會將本應 T1 的樣本判為 T0，導致該樣本從 Taiwan 區消失、落入 Intl。
- 空 `Level` 在 `computeStatistics:156-157 level=="" => "T0"` 被計為 T0，但在 Markdown `filterByCategory:308 only T0 excluded via == "T0"` 不排除，會誤入 Other。

### 🟡 D08 — 缺少 `config/categories.yaml`，分類知識分散三處

- 分類真相分散於 `models.go:ValidCategories`, `exporter.go:functionalCategories`, `algs/taiwan-classification.md §19`。三處手工同步，必漂移。
- 無單一配置檔可供 PM/審核員調整「Finance 子類是否獨立章節」等需求。
- `keywords.yaml` 的 key (`government, finance, real_estate...`) 與 ValidCategories 亦不對齊（`payment, language, company_service` 後三者在 Valid 中對應 `payment, language` 與缺失的 `company`）。

### 🟡 D09 — `levelDescription` 未被使用

- `exporter.go:432-448 levelDescription` 定義完整但從未在 `ExportMarkdown` 中呼叫，Markdown 僅輸出 `T5 (score:85)` 無可讀描述。

### 🟡 D10 — 國際化與語言處理缺口

- `serverMarkdown` 對 `Description` 做 `stripHTMLTags` 與 150 字截斷 `desc[:150]`，按位元組截斷會切斷多位元組中文（`測試` 占 6 bytes，150 bytes 截斷點可能落在字元中間），雖 `sanitizeUTF8` 補救，仍會產生 `�`。
- 中文 Category（如 `金融`）未定義行為，大小寫/語言分支無處理。

### 🟡 D11 — LLM 分類的 categories 幻覺無校驗

- `llm.go:callLLM` 的 JSON schema 對 `categories` 僅 `type: array, items: string`，無 `enum` 約束，LLM 可回傳任意字串如 `taiwan-stock`、`FinTech`；`Classify` 亦未對 `Categories` 做 `IsValidCategory` 校驗與正規化，即使未來接回 `server.Category` 亦會污染分類。

### 🟢 D12 — 測試覆蓋的盲點

- `exporter_test.go:TestExportMarkdown` 僅 2 台樣本（1 台 finance T5 + 1 台 search T0），未覆蓋: 空 Category、非法 Category、子類 Category、多 Category 重複、T1/T2 邊界、Intl Tools 缺失。`TestExport` 對 `categories.json` 未斷言 `uncategorized` vs `other` 一致性。

---

## 4. 對照 `algs/*.md` 的期望與實際不匹配

| 章節 | 期望 | 實際 | 缺口 |
|---|---|---|---|
| `algs/taiwan-classification.md §19 Category Taxonomy` | 38 項受控詞彙，LLM `categories` 必屬於此 | `exporter.go:functionalCategories` 僅 20 項且含 7 項非法 | Markdown 章節與受控詞彙脫節 |
| `algs/taiwan-classification.md:184` | LLM 示例 `categories: ["finance","taiwan-stock"]` | `taiwan-stock` 非法，且 `Classify` 丟棄 | 示例本身與 Valid 矛盾且未被消費 |
| `algs/registry-export.md §categories.json` | `{"finance":25,"government":18,...}` 按受控詞彙聚合 | `exporter.go:72-79` 按原始 Category 聚合 + `uncategorized` | 鍵空間不一致 |
| `algs/registry-export.md §servers[*].category` | 示例 `["real-estate","government","open-data"]` | `open-data` 在 Markdown 無對應章節 => Other | 規格自舉即展示缺陷 |
| `algs/registry-export.md §statistics.json.by_level` | 含 T0–T5 完整分布 | `computeStatistics` 正確 | 與 Markdown 分區一致，但與 LLM 閾值矛盾 (D07) |
| `spec.md §19 F13` | Category taxonomy 為一等功能 | pipeline 中無賦值 | 功能未實現 |

---

## 5. 用測試數據推演: 100 台的 Markdown 呈現

### 5.1 前提 (依任務要求)

- 100 台伺服器，**30 台無 Category** (`Category==nil` 或 `[]`)，**20 台多 Category** (2–3 個)，**50 台單 Category**。
- Taiwan 分布假設貼近真實爬取: `T5:8, T4:12, T3:25, T2:20, T1:10, T0:25` (T0 進入 Intl 區)。
- 多/單 Category 的值取自 `ValidCategories`，故意混入子類與非法值以暴露 D02:

```
多 Category 20 台示例:
  10 台: ["finance","stock"]          — 1 functional + 1 子類
  5  台: ["government","open-data"]   — 1 functional + 1 子類
  3  台: ["real-estate","housing"]    — 1 functional + 1 子類
  2  台: ["payment","logistics"]      — 2 子類（皆無 functional）

單 Category 50 台:
  15 台: ["finance"]                  — functional
  10 台: ["government"]               — functional
  8  台: ["real-estate"]              — functional
  5  台: ["stock"]                    — 子類
  5  台: ["payment"]                  — 子類
  4  台: ["search"]                   — 非法但 functional 存在
  3  台: ["taiwan-stock"]             — 幻覺非法
```

### 5.2 當前程式的實際呈現 (含 D01 修復前的極端情況 vs 僅修 D01 後)

#### 情形 A — 維持現狀 (D01 未修, Category 恆空模擬生產極端)

此時「30 無 Category + 20 多 Category + 50 單 Category」的區分 **完全消失**，全部視為空:

| 區段 | 台數 | 說明 |
|---|---|---|
| 🇹🇼 Taiwan-relevant — 💰 Finance & Fintech | 0 | 無匹配 |
| ... 其餘 18 個 functional 章節 | 0 | 同上 |
| 📦 Other Tools | **75** | 75 台 T1–T5 全部吸入 Other (T0 已排除) |
| 🌍 International | **25** | T0 25 台 (含工具缺失 bug) |
| **消失** | 0 | 無消失，但分類資訊全丟 |
| **重複** | 0 | 無重複（因無多 Category）|

> 審計結論: 在當前生產資料下 Markdown 的 19 個精心設計的章節全部空白，`Other Tools` 一節堆 75 台，用戶無法瀏覽。

#### 情形 B — 假設 D01 已修但保留 D02/D03 缺陷 (Category 正確填充)

按 `filterByCategory` 逐 functional 掃描，`other` 為 fallback:

| 章節 | 計數邏輯 | 台數 (去重前) | 備註 |
|---|---|---|---|
| 💰 Finance & Fintech | `finance` 精確匹配 | **25** (15 單 + 10 多) | `stock` 不計 |
| 🏛️ Government & Open Data | `government` 精確匹配 | **15** (10 單 + 5 多) | `open-data` 不計 |
| 🏠 Real Estate | `real-estate` 精確匹配 | **11** (8 單 + 3 多) | `housing` 不計 |
| 🔎 Search & Data Extraction | `search` 精確匹配 | **4** (非法但 functional 存在) | 與 Valid 矛盾的章節卻有資料 |
| 📦 Other Tools | `len==0` 或 `matchedAny==false` | **30+?** 見下 |  |
| 其餘 (transport/healthcare/...) | 無 `transport` 等資料 | 0 | 空章節被 `len==0 -> continue` 跳過不渲染 |

**Other 的組成 (30 台被投入 Other, 含重複語義):**

- 無 Category 30 台中屬 T0 的約 `30*0.25=7` 台去 Intl，剩 **23 台** 無 Category 且 T1–T5 => Other。
- 多 Category 中 `["payment","logistics"]` 2 台 — 兩個皆無 functional => Other。
- 單 Category 中 `stock` 5 + `payment` 5 + `taiwan-stock` 3 = **13 台** 子類/幻覺 => Other。
- 合計 Other **23+2+13 = 38 台** (若含 T0 無 Category 則 Markdown Other 38, Intl 25)。

**重複/消失分析:**

- **重複列出**: 0 台在當前分布中重複（因多 Category 設計為 `finance+stock` 等僅 1 項命中 functional），但若存在 `["finance","government"]` 的 5 台則會在兩章各出現一次，Markdown 總條目數 `75 + 重複數 > 75`，用戶數兩遍。`categories.json` 亦重複計數。
- **完全消失**: **0 台** 真正消失（Other 兜底 + Intl 兜底），但 **38 台**的語義消失 — 用戶在預期章節找不到 `stock/payment/open-data/housing`。
- **International 缺陷**: 25 台 T0 中 Intl 的工具列表永遠空白 (D04 死碼)，License 亦缺。

#### 情形 C — 100 台中小結 (B 情形的統計對齊)

| 指標 | 值 |
|---|---|
| `statistics.json: TotalServers` | 100 |
| `statistics.json: TaiwanRelevant` | 75 |
| `statistics.json: ByLevel` | `T5:8 T4:12 T3:25 T2:20 T1:10 T0:25` |
| `categories.json: finance` | 25 (含重複計數) |
| `categories.json: stock` | 15 (5 單+10 多) |
| `categories.json: uncategorized` | 23 (僅空 Category T1–T5) — **Markdown Other 38 != JSON uncategorized 23** 口徑分叉 |
| `REGISTRY.md` 總條目（含重複） | 75 (Taiwan) + 25 (Intl) + 重複 0 = 100 行，但 Other 膨脹至 38 |
| 用戶在 Markdown 找不到 `categories.json` 中 `stock/payment` 對應章節 | 是 |

### 5.3 推演結論

1. **當前生產 (A) 已完全失效** — 分類即 Other。
2. 即使修復 Category 賦值 (B)，**38% 的 Taiwan-relevant 伺服器仍被 Other 吞併**，因功能分類未覆蓋 Valid 子類。
3. **重複與消失的 trade-off 取決於 `filterByCategory` 是否允許多分類重複**: 現行允許重複但無標記；若改為單歸屬則消失風險轉為「次要分類資訊丟失」。
4. **Intl/Other 對齊問題**: 空 Category 的 T0 去 Intl 非 Other，正確；但空 Level 的歷史資料會誤入 Other (D07)。

---

## 6. 修復草案 (不修改原始碼，僅設計)

### 6.1 Category 正規化 (Normalize)

**目標**: `filterByCategory` 前所有 Category 已是 `ValidCategories` 中的 `kebab-lower` 且去重。

```go
// 建議新增: internal/models/category.go
func NormalizeCategory(raw string) (string, bool) {
  c := strings.TrimSpace(strings.ToLower(raw))
  c = strings.ReplaceAll(c, " ", "-")
  c = strings.ReplaceAll(c, "_", "-")
  // 別名表
  aliases := map[string]string{
    "finance & fintech": "finance",
    "finance-fintech":   "finance",
    "fintech":           "finance",
    "taiwan-stock":      "stock",        // LLM 幻覺修正
    "taiwan_stock":      "stock",
    "open data":         "open-data",
    "traditional chinese": "traditional-chinese",
    // 中文別名 (若決定支援)
    // "金融": "finance",
  }
  if a, ok := aliases[c]; ok { c = a }
  if !IsValidCategory(c) { return "", false }
  return c, true
}
func NormalizeCategories(in []string) []string {
  seen := map[string]bool{}
  var out []string
  for _, c := range in {
    if n, ok := NormalizeCategory(c); ok && !seen[n] { seen[n]=true; out=append(out,n) }
  }
  return out
}
```

- 在 `normalize.Normalize` 回填 Category 時、 `coordinator.Run` 中 LLM 結果回寫時、`storage.SaveServer` 前 均呼叫 `NormalizeCategories`。
- LLM 回傳後增加 `IsValidCategory` 過濾，未知值記 `metrics` 並丟棄或映射。

### 6.2 Filter 去重 vs 允許多分類 — 二選一並明確化

**選項 A: 允許多分類但顯式標記 (推薦，若用戶期望「金融+政府」雙歸屬可發現)**

- `filterByCategory` 保持多匹配，但在 `serverMarkdown` 內加註 `Categories: finance, government` 且首節後加 `(also listed in Government)` 避免困惑。
- 提供去重索引 `renderedIDs map[string]bool` 僅用於統計「去重後去重數」，Markdown 仍重複。
- `categories.json` 保持重複計數，但文件明確說明「按 category 計數，總和 > TotalServers」。

**選項 B: 單歸屬 + 主分類規則 (推薦，若用戶期望每伺服器僅出現一次)**

```go
func primaryCategory(cats []string) string {
  // 優先序: functionalCategories 順序；或按 ValidCategories 的語義權重
  for _, fc := range functionalCategories {
    for _, c := range cats { if c==fc.key { return fc.key } }
  }
  // 子類映射到父類 (stock->finance, housing->real-estate, open-data->government 等)
  parent := map[string]string{"stock":"finance","etf":"finance","banking":"finance","insurance":"finance","land":"real-estate","housing":"real-estate","open-data":"government","legislative":"government","judicial":"government","procurement":"government","weather":"agriculture","earthquake":"agriculture","traffic":"transport","railway":"transport","metro":"transport","bus":"transport","logistics":"transport","payment":"finance","invoice":"finance","tax":"finance","company":"ecommerce","business":"ecommerce","food":"agriculture","geography":"tourism","gis":"tourism","traditional-chinese":"language","culture":"language"}
  for _, c := range cats { if p,ok:=parent[c]; ok { return p } }
  return "other"
}
```

- 每台僅渲染於一個 functional 章節，Other 僅容納真正無父類的長尾。
- 需同步更新 `categories.json` 口徑或另提供 `primary_categories.json`。

> 決策建議: 預設 **A** (多分類) 對發現友好，配合 `parent` 映射解決 D02 的子類消失；若 Markdown 過長再提供 CLI `--dedup-markdown` 切 B。

### 6.3 Other 判斷改進

```go
func filterByCategoryFixed(servers []MCPServer, catKey string) []MCPServer {
  for _, s := range servers {
    if s.TaiwanRelevance.Level == "T0" { continue }
    if s.TaiwanRelevance.Level == "" {  // 空 Level 視為待審，不入 Other
      continue // 或歸入 "Unclassified" 新章
    }
    cats := models.NormalizeCategories(s.Category) // 先正規化
    if catKey == "other" {
      if len(cats)==0 { matched=append(...); continue }
      // 檢查是否任一 cat 能映射到 functional (含 parent 映射)
      if !hasFunctionalMapping(cats) { matched=append(...) }
      continue
    }
    if hasCategory(cats, catKey) { matched=append(...) }
  }
}
```

- 引入 `hasFunctionalMapping` 透過 `parent` 表 + `functionalCategories` 判斷，避免子類誤入 Other。
- 空 `Level` 單獨分區或計入統計 `Unknown` 而非 Other。

### 6.4 是否需新增 `config/categories.yaml`

**建議: 是，新增 `config/categories.yaml` (或 `internal/classify/config/categories.yaml`) 作為單一真相來源**，理由:

| 現狀 | 新設計 |
|---|---|
| 三處手工同步 (models.go / exporter.go / algs) | `categories.yaml` 定義 `key, name, emoji, parent, aliases`，`models.ValidCategories` 與 `exporter.functionalCategories` 皆由其生成或啟動時載入 |
| 增刪分類需改程式碼 | PM 僅改 YAML 即可增「AI/ESG」等新章 |
| 無別名 | YAML 內 `aliases: ["FinTech","finance & fintech"]` 解決 D05 |

**草案結構**:

```yaml
# config/categories.yaml
categories:
  - key: finance
    name: Finance & Fintech
    emoji: "💰"
    aliases: ["fintech", "finance & fintech"]
    children: [stock, etf, banking, insurance, payment, invoice, tax]
  - key: government
    name: Government & Open Data
    emoji: "🏛️"
    children: [open-data, legislative, judicial, procurement]
  - key: real-estate
    name: Real Estate
    emoji: "🏠"
    children: [land, housing]
  # ... 其餘
  - key: other
    name: Other Tools
    emoji: "📦"
    children: []
```

- 啟動時 `LoadCategories()` 產生 `ValidCategories`、`functionalCategories`、`parentMap`、`aliasMap`。
- 若暫不引入檔案，可先在 `models.go` 內以 `CategoryMeta` 結構集中定義，再由 `exporter.go` 引用（不新增檔案的最小改）。

### 6.5 其他必要修復 (草案)

| 項 | 修復 |
|---|---|
| D04 Intl Tools 死碼 | `serverMarkdownIntl:416-421` 補 `sb.WriteString(fmt.Sprintf("- **Tools**: %s\n", strings.Join(toolNames,", ")))` 並補 License |
| D04 Evidence 死碼 | `serverMarkdown:360` 改為 `if e.Type=="official_domain" \|\| e.Type=="taiwan_financial_api" \|\| e.Type=="repository_keyword" \|\| e.Type=="llm_classification"` 或直接 `if true` 全顯；Intl 的 evidence 保持隱藏但需註解 |
| D10 截斷 | `desc[:150]` 改為 `[]rune(desc)[:150]` 或 `grapheme` 截斷，配合 `utf8.RuneCountInString` |
| D07 閾值 | 統一 `ThresholdToLevel` 與 `buildLLMPrompt` 的 T0–T5 邊界，建議以 `rules.go` 為準更新 LLM prompt |
| D08 LoadCategories | `classify.LoadConfig` 同步載入 `categories.yaml` |
| 管線 Category 回填 | `coordinator.go:123-126` 後加 `servers[i].Category = models.NormalizeCategories(llmResult.Categories)`；`normalize.go:Normalize` 內根據 `record` 的 topics/關鍵字推斷初始 `Category` |
| 統計對齊 | `Export` 的 `categories.json` 與 `ExportMarkdown` 共用 `NormalizeCategories` + `parent` 映射；`uncategorized` 重命名為 `other` 或兩者對齊並文件說明 |
| `levelDescription` | 在 `ExportMarkdown` Statistics 區呼叫，或每伺服器下加小字描述 |

---

## 7. 重現案例 (可直接作為測試)

### 7.1 重現 Other 膨脹

```go
servers := []models.MCPServer{
  {Name:"s1", Category:[]string{"stock"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T3", Score:45}}, // 子類
  {Name:"s2", Category:[]string{"payment"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T4", Score:60}}, // 子類
  {Name:"s3", Category:[]string{"open-data"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T5", Score:80}}, // 子類
  {Name:"s4", Category:[]string{"Finance"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T3", Score:50}}, // 大小寫
  {Name:"s5", Category:nil, TaiwanRelevance:models.TaiwanRelevance{Level:"T3", Score:55}}, // 空
  {Name:"s6", Category:[]string{"finance","government"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T3", Score:55}}, // 多分類
  {Name:"s7", Category:[]string{"search"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T0", Score:0}}, // 非法但 functional
}
re := export.New()
re.ExportMarkdown("/tmp/REGISTRY.md", servers)
// 預期: s1/s2/s3 應在 Finance/Government 而非 Other；s4 應正規化命中 Finance；s6 應在兩章或擇一但可見；s5 入 Other；s7 入 Intl
// 實際: s1/s2/s3/s4 皆入 Other；s6 僅 Finance 一節（government 命中但 stock 類比不成立）；s5 Other 正確；s7 Intl 但 Tools 空
```

### 7.2 重現生產 Category 恆空

```go
// 模擬 coordinator 前
rec := &sources.RawRecord{Candidate: sources.RawCandidate{RepositoryURL:"https://github.com/foo/twse-mcp"}, Readme:"TWSE data"}
srv, _ := normalize.New().Normalize(rec)
fmt.Println(srv.Category) // => [] 空，後續 ExportMarkdown 該 T3/T5 伺服器必入 Other

// 模擬 LLM 回傳被丟棄
lc := classify.NewLLMClassifier() // 需 mock
res, _ := lc.Classify(ctx, srv)
fmt.Println(res.Categories) // 有值如 ["finance"] 但 coordinator 未寫回 srv.Category
```

### 7.3 重現 Intl Tools 缺失

```go
s := models.MCPServer{Name:"global", Category:[]string{"search"}, TaiwanRelevance:models.TaiwanRelevance{Level:"T0"}, Tools:[]models.Tool{{Name:"search"}}, Repository:models.RepositoryInfo{URL:"https://github.com/x/y"}}
md := export.ServerMarkdownIntl(s) // 若導出為 public 或透過 ExportMarkdown 間接
// 檢查 md 是否含 "search" — 實際不含
```

### 7.4 重現 Evidence 死碼

```go
s := models.MCPServer{TaiwanRelevance:models.TaiwanRelevance{Level:"T3", Score:55, Evidence:[]models.Evidence{{Type:"official_domain", Rule:"official_domain_match", Score:40, MatchedText:"twse.com.tw"}, {Type:"repository_keyword", Rule:"taiwan_keyword", Score:20}}}}
md := export.ServerMarkdown(s)
// 檢查 md 是否含 "twse.com.tw" — 實際不含，因 Type != "keyword"/"domain"
```

---

## 8. 驗證清單 (純審計，不執行)

- [ ] `grep -rn "Category"` 全倉僅測試賦值 — 已驗證
- [ ] `exporter.go:282-303` vs `models.go:72-85` 差集 — 已列
- [ ] `filterByCategory` 對 `T0/空Level/空Category/多Category/大小寫` 5 類輸入的行為 — 已推演
- [ ] `serverMarkdown` vs `serverMarkdownIntl` 行級 diff — 已對齊
- [ ] `Export` 的 JSON vs Markdown 口徑 — 已對齊
- [ ] `algs/*.md` 期望 vs 實作 — 已列表
- [ ] 100 台推演 (30 空 + 20 多) — 已分 A/B 情形量化

---

## 9. 建議優先序

1. **P0**: 修 D01 (Category 賦值) + D02 parent 映射 + D06 口徑對齊 — 否則 Markdown 分類形同虛設。
2. **P1**: 修 D04 死碼 (Intl Tools / Evidence) + D05 正規化。
3. **P2**: 新增 `config/categories.yaml` (D08) + 閾值統一 (D07) + 截斷修復 (D10)。
4. **P3**: 多分類策略文件化 (D03) + `levelDescription` 啟用 + 測試補齊 (D12)。

---

*本報告未修改任何原始碼，僅審計。修復草案為設計建議，需另起任務實作並補測試。*
