# DESIGN.md — Thiết kế hệ thống News Crawler

## 1. Tổng quan

Hệ thống crawl tin tức tự động, tóm tắt bằng LLM, gửi vào Telegram. Toàn bộ chạy trên GitHub Actions — serverless, miễn phí.

---

## 2. Luồng xử lý

```
┌─────────────────────────────────────────────────────────────┐
│                     GitHub Actions                           │
│                                                             │
│  cron trigger                                               │
│       │                                                     │
│       ▼                                                     │
│  ┌─────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │  Parse  │───▶│ Fetch full   │───▶│  Dedup check     │   │
│  │  RSS    │    │ article page │    │  (seen.json)     │   │
│  └─────────┘    └──────────────┘    └────────┬─────────┘   │
│                                              │ new only     │
│                                              ▼             │
│                                    ┌──────────────────┐   │
│                                    │  LLM Summarize   │   │
│                                    │  Groq → Gemini   │   │
│                                    │  (rotate keys)   │   │
│                                    └────────┬─────────┘   │
│                                             │             │
│                                             ▼             │
│                                    ┌──────────────────┐   │
│                                    │ Send Telegram    │   │
│                                    │ Bot API          │   │
│                                    └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Thành phần

### 3.1 RSS Parser
- Dùng `github.com/mmcdole/gofeed`
- Parse feed → lấy title + link của từng bài
- Tối đa 5 bài/feed để kiểm soát rate

### 3.2 Web Crawler
- Từ link trong RSS → fetch full HTML content
- Extract text từ `<p>` và `<article>` tags
- Skip `<script>`, `<style>`, `<nav>`, `<footer>`, `<header>`, `<aside>`, `<form>`
- Truncate về 3000 ký tự (đủ cho LLM, tránh token dài)

**Chống block:**
- Random delay 3–8 giây giữa mỗi request
- Rotate 3 user agents (Chrome, Safari, Firefox)
- Set browser-like headers (Accept, Accept-Language, v.v.)
- Bắt HTTP 429 → chờ 30 giây → skip bài đó
- Fallback về RSS description nếu fetch thất bại

### 3.3 LLM Summarizer
- **Primary:** Groq (llama-3.1-8b-instant) — nhanh, 14,400 req/ngày free
- **Fallback:** Gemini (gemini-1.5-flash) — 1,500 req/ngày free
- Rotate round-robin qua các API keys
- Tự động thử provider tiếp theo nếu fail

**Tổng capacity free tier:**
```
Groq:   3 keys × 14,400 = 43,200 req/ngày
Gemini: 2 keys × 1,500  =  3,000 req/ngày
Total:                    46,200 req/ngày
```
Thực tế dùng: ~50 req/ngày → dư sức.

### 3.4 Dedup Store (`seen.json`)
- Lưu MD5 hash của article URL
- Persist giữa các runs bằng `actions/cache`
- `restore-keys: seen-articles-<category>-` → luôn lấy cache mới nhất
- Format: `["hash1", "hash2", ...]`

### 3.5 Telegram Sender
- Dùng Bot API `sendMessage`
- Parse mode: `HTML` (hỗ trợ `<b>`, `<i>`, `<a>`)
- `disable_web_page_preview: true` — gọn hơn
- 2 bot riêng biệt cho 2 category

---

## 4. Category & Schedule

| Category | Schedule | Bot | Feeds |
|---|---|---|---|
| `vietnam` | Mỗi 1 giờ | VN Bot | VnExpress, Tuổi Trẻ, Thanh Niên |
| `cybersecurity` | Mỗi 6 giờ | SEC Bot | TheHackerNews, BleepingComputer, Threatpost |

Schedule khác nhau vì:
- Tin Việt Nam cập nhật liên tục cần check thường xuyên hơn
- Tin security ít hơn, 6 tiếng/lần là đủ

---

## 5. Prompt Design

### Vietnam news
```
📌 Sự kiện:  chuyện gì xảy ra        (≤50 từ)
👥 Liên quan: ai/tổ chức nào          (≤50 từ)
📍 Bối cảnh: khi nào/ở đâu/tại sao   (≤50 từ)
```

### Cybersecurity news
```
🔍 Chuyện gì:  CVE + hệ thống + tác động    (≤50 từ)
🎯 Ảnh hưởng: phiên bản + đối tượng         (≤50 từ)
⚠️ Mức độ:    severity + CVSS + lý do       (≤50 từ)
🛠️ Cần làm gì: patch version + workaround   (≤50 từ)
```

Nguyên tắc:
- Tiếng Việt nhưng giữ thuật ngữ kỹ thuật tiếng Anh
- Tối đa 50 từ/mục — đủ ý, không dư
- Không bịa thông tin không có trong bài gốc

---

## 6. Error Handling

| Tình huống | Xử lý |
|---|---|
| RSS feed lỗi | Log + skip feed đó, tiếp tục feed khác |
| Fetch article 429 | Chờ 30s + skip bài đó |
| Fetch article lỗi khác | Fallback về RSS description |
| Content quá ngắn (<100 ký tự) | Fallback về RSS description |
| LLM provider fail | Thử provider tiếp theo |
| Tất cả LLM fail | Dùng title làm summary |
| Telegram send fail | Log error, tiếp tục bài tiếp theo |

---

## 7. Mở rộng

**Thêm category mới:**
1. Thêm feeds vào `var feeds` trong `main.go`
2. Thêm system prompt vào `var systemPrompts`
3. Thêm label vào `var labels`
4. Thêm bot config vào `var bots`
5. Tạo workflow file mới trong `.github/workflows/`

**Thêm LLM provider:**
1. Thêm `LLMProvider` vào `var providers`
2. Implement `callXxx()` function
3. Thêm case vào switch trong `summarize()`

**Thêm RSS feed:**
- Chỉ cần thêm URL vào slice tương ứng trong `var feeds`
