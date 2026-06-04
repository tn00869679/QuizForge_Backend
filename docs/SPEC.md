# 證券營業員刷題網站 — 系統規格書 v1.0

> 後端：Golang + Gin　·　前端：單頁應用（框架不限）　·　題庫來源：證基會歷屆試題（證券商業務員）

---

## 1. 專案概述

本系統為「證券商業務員」證照考試的線上刷題平台，參考 AIAP 練習站的互動模式，提供歷屆題練習、模擬考、錯題複習等功能。後端採 Golang + Gin 提供 RESTful API 與題庫管理，前端為單頁應用（框架不限）。

與參考站不同處：參考站為純前端、資料存 localStorage；本系統改為**後端集中管理題庫**（因題庫來自 PDF 需解析入庫），並支援使用者作答紀錄的伺服器端儲存（可選），同時保留 localStorage 作為未登入時的本機紀錄。

## 2. 名詞定義

- **考試類別（Category）**：證券商業務員等證照類別。
- **科目（Subject）**：類別下的考科。
- **考次（ExamSession）**：某一年度某一梯次的測驗，例如 114 年第 3 次。
- **題目（Question）**：單一選擇題，含題幹、四選項、正解、解析、標籤。
- **題組（QuestionSet）**：一次練習或模擬考所抽取的題目集合。
- **作答紀錄（Attempt）**：使用者對單題的作答結果。

## 3. 系統架構

前端 SPA 透過 HTTPS 呼叫後端 Gin API。後端連接關聯式資料庫（建議 PostgreSQL）儲存題庫與作答紀錄。另有一個獨立的「題庫匯入工具」（CLI 或後台），負責下載官方 PDF、解析成結構化資料、人工校對後寫入資料庫。前端與後端分離部署，API 以 `/api/v1` 為前綴。

## 4. 資料模型

核心資料表設計（PostgreSQL 語意）：

| 資料表 | 主要欄位 |
|--------|----------|
| categories（考試類別） | id (PK)、code（官方類別代碼，如 "02"）、name、description、created_at |
| subjects（科目） | id (PK)、category_id (FK)、name、order_index、created_at |
| exam_sessions（考次） | id (PK)、category_id (FK)、year（民國年）、term（梯次）、source_url、label、created_at |
| questions（題目） | id (PK)、subject_id (FK)、exam_session_id (FK)、number、stem、options（JSONB `[{key,text}]`）、answer、explanation、tags（text[]）、difficulty、is_active、created_at、updated_at |
| attempts（作答紀錄） | id (PK)、user_id (FK，可空)、question_id (FK)、selected、is_correct、is_marked_uncertain、is_favorite、mode（practice/exam）、created_at |
| users（使用者，選配） | id、email、display_name、created_at（建議 OAuth/SSO，不做密碼登入） |

題目唯一性建議以 `(exam_session_id, subject_id, number)` 建立 unique index，避免重複匯入。

## 5. 後端 API 規格（Gin）

所有回應為 JSON，統一外層格式為 `{ "data": ..., "error": null }`，錯誤時為 `{ "data": null, "error": { "code", "message" } }`。分頁採 `page` 與 `page_size`（預設 20，上限 100）。

### 5.1 題庫查詢類

| 方法 / 路徑 | 說明 |
|------------|------|
| `GET /api/v1/categories` | 所有考試類別 |
| `GET /api/v1/categories/:id/subjects` | 該類別科目 |
| `GET /api/v1/exam-sessions?category_id=` | 考次清單（供年度篩選） |
| `GET /api/v1/questions` | 核心查詢。參數：category_id、subject_id（多值）、session_ids（多值）、keyword、status（unanswered/wrong/favorite）、limit（5/10/20/50/all）、random、page、page_size |
| `GET /api/v1/questions/:id` | 單題。可帶 include_answer=false；模擬考模式不回傳 answer 與 explanation |

### 5.2 題組／模擬考類

| 方法 / 路徑 | 說明 |
|------------|------|
| `POST /api/v1/practice/generate` | 依篩選條件回傳隨機題組 id 清單 |
| `POST /api/v1/exam/start` | 每科隨機抽取 N 題，回傳題目（不含答案）、exam_token、考試秒數 |
| `POST /api/v1/exam/grade` | 送出作答，回傳逐題對錯、正解、解析、總分與各科分數 |

### 5.3 作答紀錄類（需登入；未登入用 localStorage）

| 方法 / 路徑 | 說明 |
|------------|------|
| `POST /api/v1/attempts` | 寫入單題作答 |
| `PATCH /api/v1/attempts/:question_id` | 更新收藏／不確定標記 |
| `GET /api/v1/stats` | 統計（已作答、正確率、錯題數、收藏數） |
| `GET /api/v1/attempts?status=wrong\|favorite\|uncertain` | 錯題本／收藏／不確定頁查詢 |

### 5.4 管理／匯入類（後台權限）

| 方法 / 路徑 | 說明 |
|------------|------|
| `POST /api/v1/admin/import` | 批次寫入已校對題目 JSON，需後台驗證，不對一般使用者開放 |

## 6. PDF 題庫匯入流程

官方來源為 PDF（試題與解答分檔），需獨立匯入管線：

1. 維運人員手動下載對應類別 PDF。證券商業務員為類別代碼 `02`，路徑形如 `examweb.sfi.org.tw/Download/{01|02}/02.pdf`（`01`=前一次、`02`=前二次，檔名加 `a` 為解答）。
2. 以 PDF 解析（建議 Go 的 `pdfcpu` 或外部 `pdftotext`）抽取文字，依題號切分題幹與選項。
3. 比對解答 PDF 取得正解。
4. 輸出結構化 JSON 供**人工校對**（PDF 排版常有跨頁、表格、公式問題，務必保留人工校對關卡）。
5. 透過 `POST /api/v1/admin/import` 入庫。

解析器需處理：題號偵測、A/B/C/D 選項偵測、跨頁合併、特殊符號。匯入工具應記錄 `source_url` 與抓取時間，以利追溯與授權管理。

> ⚠️ **授權提醒**：證基會提供的歷屆試題著作權通常屬該基金會所有。將題目重製為對外（尤其商業化）題庫網站前，建議先向證基會確認重製與散布的授權範圍。

## 7. 前端頁面規格

整體採深色主題、頂部固定導覽列（首頁／練習／自創題／模擬考／錯題本／收藏／不確定），右上角有「清除本機紀錄」按鈕。

### 7.1 首頁
品牌標題、入口說明、兩個主要 CTA（開始歷屆題練習／進入模擬考），右側題庫統計卡（總題數、科目數），下方功能說明卡片區。

### 7.2 練習頁（三欄式）
- **左欄篩選**：考試年度（多選）、科目（多選）、題目狀態（尚未作答／答錯題目／已收藏）、本次題數（5/10/20/50/全部與自訂）、關鍵字搜尋；底部「產生隨機題組」「從第一題開始」。
- **中欄題目卡**：題目 X/N、來源標籤（考次｜科目｜題號）、收藏星號、看答案；題幹；四選項（作答後正解綠、錯選紅）；展開解析含考點、檢討方向、標籤；底部上一題／標記不確定／下一題。
- **右欄**：練習進度（圓形正確率、目前題組／已完成／不確定數）、本機備份（匯出／匯入 JSON）、快速回顧（列出錯題）、題號盤（可點擊跳題，依狀態著色）。

### 7.3 模擬考頁
進入前顯示考前說明（每科隨機抽取 50 題、120 分鐘、預計題數）。開始後進入考試模式：倒數計時、已作答 X/N、答案檢視「關閉」、交卷按鈕；考試中不顯示答案、解析、收藏與不確定。交卷後呼叫 grade API 顯示成績單（總分、各科分數、逐題檢視正解與解析）。

### 7.4 錯題本／收藏／不確定頁
復用練習頁版型，題庫範圍預先過濾為對應狀態，左欄按鈕改為「重新整理題組」。

### 7.5 自創題頁（選配）
讓使用者新增延伸練習題，存於本機或帳號下，並可在練習中混入。

前端需支援未登入純本機模式（localStorage 存作答紀錄與設定，提供匯出／匯入備份）；若導入登入則同步至後端 attempts。

## 8. 互動與狀態行為

作答後即時回饋為練習模式核心：選項著色、解析展開、統計即時更新、錯題自動加入快速回顧與題號盤著色。模擬考模式則延後回饋至交卷。題號盤需即時反映每題狀態（未作答／已作答／不確定）。所有篩選變更後需重新產生題組。

## 9. 非功能需求

- **效能**：題目查詢需建立適當索引（subject_id、exam_session_id、全文搜尋 GIN index）。
- **安全**：API 需做輸入驗證與速率限制，admin 端點需驗證權限。
- **隱私**：訪客模式不蒐集個資，登入採 OAuth/SSO。
- **可維護性**：PDF 匯入需保留人工校對與來源紀錄。
- **部署**：建議前後端皆以 Docker 容器化部署。

## 10. 建議開發里程碑

| 階段 | 內容 |
|------|------|
| 第一階段 | 建立資料模型、題庫查詢 API 與 PDF 匯入工具，先匯入一個考次驗證流程 |
| 第二階段 | 前端練習頁與本機紀錄 |
| 第三階段 | 模擬考與成績單 |
| 第四階段 | 錯題本／收藏／不確定與備份匯入匯出 |
| 第五階段（選配） | 登入同步與自創題 |
