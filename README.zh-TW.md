<div align="center">

[English](README.md) | [繁體中文](README.zh-TW.md) | [简体中文](README.zh-CN.md)

</div>

# Awesome Taiwan MCP

自動化爬蟱工具，用於發現、分析和驗證與台灣相關的 MCP Servers。

## 概述

**Awesome Taiwan MCP** 從多個來源（GitHub、官方註冊表）爬取 MCP（Model Context Protocol）伺服器，判斷其台灣相關性，進行標準化、去重、品質評分、健康檢查，並匯出標準化的註冊表。

透過關鍵字匹配、官方網域（如 `.gov.tw`、`.org.tw`）、政府 API、金融 API（TWSE、TPEx）、不動產資料，以及繁體中文語言檢測來識別台灣相關的伺服器。

**Pipeline:** Discovery → Normalize → Taiwan Scoring → LLM Classification → Dedup → Verify → Health Check → Quality Score → Persist → Export

## 功能

- **多來源發現**: GitHub 倉庫、官方註冊表、mcpservers.org (Sitemap)
- **品質評分**: 10 項維度評分 (A-F 等級)
- **安全掃描**: 注入模式、不安全傳輸、Fork 檢測
- **Protocol 驗證**: 完整 MCP Protocol (initialize, tools/list, resources/list, prompts/list)
- **健康檢查**: 端點延遲與可用性監測
- **增量爬蟲**: `--incremental` 標誌僅重新爬取有變更的候選
- **LLM 分類**:模糊候選 (分數 20-55) 透過 OpenAI-compatible LLM API 進行分類
- **JSON 註冊表匯出**: registry.json、registry.min.json、categories.json、sources.json、statistics.json、health.json
- **Markdown 匯出**: 包含完整伺服器資訊的 REGISTRY.md
- **搜尋**: 文字搜尋與 Capability 搜尋
- **SQLite 持久化**: 所有資料存儲於 SQLite (modernc.org/sqlite，純 Go，無 CGO)

## 架構

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI (cmd/crawler)                      │
│              指令: crawl, export, search, stats             │
└─────────────┬───────────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────────┐
│                   CrawlCoordinator                           │
│  協調 8 個階段的爬蟲 pipeline:                                 │
│  1. Discover + Fetch  2. Normalize  3. Taiwan Score           │
│  3b. LLM Classification  4. Quality Score  5. Identity        │
│  6. Dedup  6.5. Verify  7. Persist  8. Finish                │
└───────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┬──────────────────┬──────────────────┐
    │                   │                  │                  │
    ▼                   ▼                  ▼                  ▼
  Sources           Storage            Verify           Export
  (adapters)    (SQLite store)      (repo/proto)     (JSON + MD)
    │                   │                  │
    ▼                   ▼                  ▼
  GitHub          SQLite DB           Health
  Registry                           Security
    mcpservers.org                   Scoring
  (Sitemap-based)
  modelcontextprotocol/servers-archived
                                          │
                                          ▼
                                       Scoring
```

## 專案結構

```
├── cmd/crawler/
│   └── main.go               # CLI 進入點 (cobra)
├── internal/
│   ├── classify/             # 台灣相關性分類
│   │   ├── keywords.go       # 關鍵字匹配 (嵌入式配置)
│   │   ├── llm.go            # LLM 分類器 (OpenAI-compatible API)
│   │   └── rules.go          # 評分規則 (官方網域、政府 API 等)
│   ├── crawler/              # Pipeline 協調
│   │   ├── coordinator.go    # CrawlCoordinator (8-stage pipeline)
│   │   ├── incremental.go    # IncrementalCrawler
│   │   └── run/              # 爬蟲運行管理
│   ├── dedupe/               # 去重引擎
│   ├── evidence/             # 證據收集
│   ├── health/               # 端點健康檢查
│   ├── manifest/             # MCP manifest 偵測
│   ├── metrics/              # 結構化日誌 + 爬蟲指標
│   ├── models/               # 資料模型 (MCPServer 等)
│   ├── normalize/            # 正規化 (RawRecord → MCPServer)
│   ├── retry/                # 重試客戶端 (指數退避)
│   ├── scoring/              # 品質評分引擎 (10 項組件)
│   ├── search/               # 搜尋引擎 (文字 + capability)
│   ├── security/             # 安全掃描
│   ├── sources/              # 資料源 adapters
│   │   ├── github/           # GitHub 倉庫發現
│   │   ├── githubrepo/       # GitHub 目錄型 (modelcontextprotocol/servers, servers-archived)
│   │   ├── mcpmarket/        # mcpmarket.com (骨架，被 Vercel WAF 擋)
│   │   ├── mcpserversorg/    # mcpservers.org (Sitemap + goquery)
│   │   └── registry/         # 官方註冊表 adapter
│   └── verify/               # 倉庫 + MCP protocol 驗證
├── config/
│   ├── keywords.yaml         # 台灣關鍵字矩陣
│   └── domains.yaml          # 官方台灣網域
├── tests/
│   ├── fixtures/             # JSON 測試固件
│   ├── integration/          # E2E pipeline 測試
│   ├── unit/                 # 單元 + golden regression 測試
│   └── benchmarks/           # 效能基準測試
├── Dockerfile                # 多階段建構: golang:1.26-alpine → alpine:latest
├── docker-compose.yaml       # crawler 服務
└── .golangci.yml             # linter 配置
```

## 需求

- **Go** 1.25+
- **GITHUB_TOKEN** — 用於 GitHub API 搜尋與抓取
- **OPENAI_API_KEY** — (選選) 用於模糊候選的 LLM 分類
- **OPENAI_BASE_URL** — (選填) OpenAI-compatible API 端點，預設為 `https://api.openai.com/v1`
- **Docker** — 用於容器建構

## 安裝

### 從原始碼建構

```bash
go build -o crawler ./cmd/crawler
```

### Docker

```bash
docker build -t awesome-taiwan-mcp .
```

## 配置

| 環境變數 | 必填 | 預設 | 說明 |
|---|---|---|---|
| `GITHUB_TOKEN` | 是 | — | GitHub API Token，用於倉庫搜尋與抓取 |
| `OPENAI_API_KEY` | 否 | — | OpenAI-compatible API Key，用於 LLM 分類 |
| `OPENAI_BASE_URL` | 否 | `https://opencode.ai/zen/v1` | OpenAI-compatible API 基礎 URL |
CLI 標誌:

| 標誌 | 預設 | 說明 |
|---|---|---|
| `--source` | `all` | 要爬取的資料源: `github`, `registry`, `mcpserversorg`, `mcpmarket`, 或 `all` |
| `--max-per-source` | `10` | 每個資料源的最大候選數量 (0=無限制) |
| `--full` | `false` | 強制完整爬取 |
| `--incremental` | `false` | 執行增量爬取 (檢查上次爬取時間) |
| `--db` | `./data/registry.db` | SQLite 資料庫路徑 |
| `--config` | `config/sources.yaml` | 配置文件路徑 |
| `--markdown` | `false` | (export 子指令) 同時生成 REGISTRY.md |
| `--capability` | — | (search 子指令) 以 capability 關鍵字搜尋 |
| `--min-score` | `0` | 最小品質分數過濾 |
| `--level` | — | 依台灣相關性等級過濾 (T0-T5) |
| `--category` | — | 依分類過濾 |

## 快速開始

```bash
# 1. 爬取台灣 MCP Servers
export GITHUB_TOKEN=your_github_token_here
./crawler crawl --source github --workers 4 --max-per-source 10

# 同時爬取 mcpservers.org (10k+ servers via Sitemap)
./crawler crawl --source mcpserversorg --workers 2 --max-per-source 100

# 2. 匯出註冊表
./crawler export --markdown
```
# 3. 搜尋伺服器
./crawler search "taiwan"
./crawler search --capability "filesystem"

# 4. 查看統計
./crawler stats
```

## 使用方法

### Crawl

```bash
# 完整爬取 (強制刷新全部)
./crawler crawl --full --source all

# 增量爬取 (僅檢查更新)
./crawler crawl --incremental --source github

# 限制每個資料源的候選數量
./crawler crawl --source github --max-per-source 20
```

### Export

```bash
# 匯出 JSON 註冊表 (6 個檔案)
./crawler export

# 同時生成人類可讀的 Markdown
./crawler export --markdown
```

輸出檔案在 `registry/` 目錄下：
- `registry.json` — 完整註冊表 (所有伺服器資料)
- `registry.min.json` — 精簡版本 (供網頁客戶端使用)
- `categories.json` — 分類分布
- `sources.json` — 資料源分布
- `statistics.json` — 聚合統計
- `health.json` — 每個伺服器的健康狀態
- `REGISTRY.md` — 人類可讀的 Markdown (需要 `--markdown`)

### Markdown 註冊表

`--markdown` 標誌會生成包含以下內容的人類可讀 `REGISTRY.md`:

- **統計資料**: 總伺服器數、台灣相關性分布 (T0-T5)、健康狀態、品質等級分布
- **🇹🇼 Taiwan-relevant Servers**: T1-T5 相關性的伺服器，依功能分類分組 (Finance, Government, Real Estate 等)
- **🌍 International Servers**: T0 非台灣特定但 MCP 相容的伺服器
- **每個伺服器的詳細資訊**: 倉庫連結 (含星星數)、語言 (連結到 GitHub 搜尋)、台灣相關性等級 + 分數、分類證據、健康狀態、品質、Tools、Endpoints

分類對照定義於 `config/categories.yaml` (單一真相來源)。子分類如 `stock`、`etf`、`banking` 正規化為父分類 `finance`，`land`/`housing` → `real-estate` 等。`Other` 分類僅收集完全無匹配分類的伺服器。
等級說明:
- **T5**: 絕對台灣導向 — 官方政府或金融 API，具備台灣特定資料
- **T4**: 非常強台灣相關 — 台灣資料來源，明確本地導向
- **T3**: 強台灣相關 — 台灣特定資料或服務 (房地產、金融等)
- **T2**: 中等台灣相關 — 檢測到部分台灣內容或關鍵字
- **T1**: 弱台灣相關 — 最小台灣連接
- **T0**: 無台灣相關 — 國際或一般用途伺服器

```bash
# 文字搜尋
./crawler search "financial"
./crawler search --level T3

# Capability 搜尋
./crawler search --capability "filesystem"
./crawler search --capability "database"

# 品質過濾
./crawler search --min-score 70

# JSON 輸出
./crawler search "taiwan" --json
```

### Stats

```bash
./crawler stats
```

## 資料模型

### MCPServer

| 欄位 | 類型 | 說明 |
|---|---|---|
| `id` | `string` | 正規化 repo URL 的 SHA256 (CanonicalID) |
| `name` | `string` | 顯示名稱 |
| `slug` | `string` | URL-safe slug |
| `description` | `string` | 簡短描述 |
| `taiwan_relevance` | `TaiwanRelevance` | 台灣分類 (等級 T0-T5, 分數, 信心度, 證據) |
| `repository` | `RepositoryInfo` | GitHub 倉庫元數據 |
| `endpoints` | `[]Endpoint` | MCP 端點 (URL, transport, TLS) |
| `tools` | `[]Tool` | 擷取的 Tools |
| `resources` | `[]Resource` | 擷取的 Resources |
| `prompts` | `[]Prompt` | 擷取的 Prompts |
| `quality` | `QualityScore` | 100 分品質評估 (分數, A-F 等級) |
| `security` | `[]SecurityFinding` | 安全發現 |
| `health` | `HealthStatus` | HEALTHY, DEGRADED, UNAVAILABLE, UNKNOWN |

## 評分

### 台灣相關性 (§17)

| 規則 | 分數 | 證據類型 |
|---|---|---|
| 官方台灣網域 (.gov.tw, .org.tw) | +40 | `official_domain` |
| 台灣政府 API 檢測 | +40 | `official_gov_api` |
| 台灣金融 API (TWSE, TPEx, FinMind) | +35 | `taiwan_financial_api` |
| 台灣特定資料集檢測 | +30 | `taiwan_dataset` |
| 台灣關鍵字出現在 repo 名稱/描述 | +20 | `repository_keyword` |
| 台灣語言 (zh-TW, 繁體中文) | +15 | `taiwan_language` |
| 台灣公司/服務提及 | +15 | `taiwan_company` |
| README 提及台灣 | +5 | `readme_mention` |

等級: T0 (0-19), T1 (20-35), T2 (36-55), T3 (56-70), T4 (71-85), T5 (86-100)

### 品質評分 (§31)

10 項組件，每項 0-10 分：
- 倉庫星數
- 倉庫活躍度 (90 天內更新)
- README 完整度
- 文檔 (CONTRIBUTING, LICENSE 等)
- Manifest 檔案存在 (claude.json, config.json 等)
- MCP Protocol 符合度
- Transport 支援 (stdio + HTTP)
- Tool 數量 (>0)
- Resource 數量 (>0)
- Prompt 數量 (>0)

等級: A (90-100), B (80-89), C (70-79), D (60-69), F (0-59)

## 錯誤處理

- **Rate 限制**: 指數退避 (1s → 2s → 4s → 8s, 上限 30s, 最大 3 次重試)
- **資料源降級**: 失敗的資料源會被記錄並跳過，pipeline 繼續執行
- **LLM 失敗**: 回退到確定性 T0 分類，伺服器元數據保持不變
- **網路逾時**: 全程支援 Context 取消
- **SQL 錯誤**: 單個伺服器儲存失敗會被記錄，pipeline 繼續

## 測試

```bash
# 所有測試
go test ./... -count=1 -timeout=120s

# 帶 Race 檢測器
go test -race ./internal/... -count=1 -timeout=120s

# 覆蓋率 (按套件)
go test ./... -count=1 -cover

# Golden regression 測試
go test ./tests/unit/ -v -run Golden

# 效能基準測試
go test ./tests/benchmarks/ -bench=. -benchmem

# 整合測試
go test ./tests/integration/ -v
```

覆蓋率 (按套件):

| 套件 | 覆蓋率 |
|---|---|
| `internal/classify` | 87.6% |
| `internal/dedupe` | 90.7% |
| `internal/evidence` | 100.0% |
| `internal/export` | 87.0% |
| `internal/health` | 91.7% |
| `internal/manifest` | 92.9% |
| `internal/metrics` | 100.0% |
| `internal/scoring` | 94.0% |
| `internal/security` | 93.2% |
| `internal/storage` | 85.9% |
| `internal/verify` | 94.7% |

## 建構

```bash
# 標準建構
go build ./...

# Vet
go vet ./...

# 模組驗證
go mod verify

# Docker
docker build -t awesome-taiwan-mcp .
docker compose up
```

### Docker

Dockerfile 使用多階段建構：
1. **Builder**: `golang:1.26-alpine3.24` —編譯二進制檔
2. **Runtime**: `alpine:latest` —以非 root 使用者執行

安全性：非 root 使用者 (uid 1000)，無 privileged，資源限制。

```bash
docker build -t awesome-taiwan-mcp .
docker run --rm \
  -e GITHUB_TOKEN=your_token \
  -v $(pwd)/data:/data \
  awesome-taiwan-mcp crawl --db /data/registry.db
```

## 開發

```bash
# 安裝依賴
go mod download

# 執行 linter
golangci-lint run

# 格式化
gofmt -s -w .

# 執行單元測試 (詳細輸出)
go test ./internal/classify/ -v
```

## 已知限制

- **官方註冊表資料源**: `api.mcp-servers.dev` 在沙箱環境中可能無法解析 DNS
- **GitHub 速率限制**: 每 Token 5000 requests/小時；50 個關鍵字 × 2s ≈ 88s 進行完整發現
- **LLM 分類器**: 需要 `OPENAI_API_KEY` 環境變數；在無 API Key 時會優雅降級到確定性分類
- **增量爬蟲**: 依據 SQLite 中的上次爬取時間戳記；需要先有爬取資料
- **Docker Compose**: 預設執行 `--help`，需要覆寫指令才能進行實際爬蟲
- **MCP Protocol 驗證**: 需要公開可存取的 HTTP 端點；本地端點 (localhost) 可能失敗

## 授權

請參閱 `LICENSE` 檔案。
