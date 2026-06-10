# News Crawler — Telegram Bot

Tự động crawl tin tức, tóm tắt bằng AI, gửi vào Telegram. Chạy hoàn toàn trên GitHub Actions — không cần server.

---

## Tính năng

- Crawl RSS feed → truy cập link → lấy full content
- Tóm tắt bằng AI (Groq + Gemini, rotate key để tránh rate limit)
- Gửi vào 2 Telegram group riêng biệt theo category
- Dedup bằng `seen.json` — không gửi lại bài cũ
- Chống block: random delay, rotate user agent, fallback khi rate limit

---

## Kiến trúc tổng quan

```
GitHub Actions (cron)
        │
        ▼
   main.go (Go binary)
        │
        ├── Parse RSS feeds
        │       │
        │       └── Fetch full article content (polite crawl)
        │
        ├── Summarize via LLM
        │       ├── Groq (llama-3.1-8b-instant) — primary
        │       └── Gemini (gemini-1.5-flash)   — fallback
        │
        ├── Send to Telegram Bot API
        │       ├── 🇻🇳 Vietnam group
        │       └── 🔐 Cybersecurity group
        │
        └── Save seen.json → GitHub Actions Cache
```

---

## Cấu trúc thư mục

```
news-crawler/
├── main.go                          # toàn bộ logic crawler
├── go.mod                           # Go dependencies
├── seen.json                        # danh sách bài đã gửi (managed by GH cache)
├── .github/
│   └── workflows/
│       ├── vietnam_news.yml         # chạy mỗi 1 giờ
│       └── security_news.yml       # chạy mỗi 6 giờ
├── README.md
├── DESIGN.md                        # thiết kế chi tiết
└── SECRETS.md                       # hướng dẫn setup secrets
```

---

## Cài đặt nhanh

### 1. Tạo Telegram Bot

1. Mở Telegram, tìm `@BotFather`
2. Gõ `/newbot` → đặt tên → lấy **Bot Token**
3. Tạo 2 bot: 1 cho Vietnam news, 1 cho Security news
4. Add bot vào group/channel tương ứng, set bot làm **Admin**
5. Lấy Chat ID bằng cách nhắn tin vào group rồi truy cập:
   ```
   https://api.telegram.org/bot<TOKEN>/getUpdates
   ```

### 2. Lấy API Keys miễn phí

| Provider | Link đăng ký | Free limit |
|---|---|---|
| Groq | https://console.groq.com | 14,400 req/ngày |
| Gemini | https://aistudio.google.com | 1,500 req/ngày |

Tạo 3 Groq key + 2 Gemini key từ các tài khoản Google khác nhau để tối đa free tier.

### 3. Setup GitHub Secrets

Vào repo → **Settings → Secrets and variables → Actions → New repository secret**

| Secret | Mô tả |
|---|---|
| `VN_BOT_TOKEN` | Token bot Telegram cho Vietnam news |
| `VN_CHAT_ID` | Chat ID của Vietnam news group |
| `SEC_BOT_TOKEN` | Token bot Telegram cho Security news |
| `SEC_CHAT_ID` | Chat ID của Security news group |
| `GROQ_API_KEY_1` | Groq API key #1 |
| `GROQ_API_KEY_2` | Groq API key #2 |
| `GROQ_API_KEY_3` | Groq API key #3 |
| `GEMINI_API_KEY_1` | Gemini API key #1 |
| `GEMINI_API_KEY_2` | Gemini API key #2 |

### 4. Push lên GitHub

```bash
git init
git add .
git commit -m "init news crawler"
git remote add origin https://github.com/<username>/<repo>.git
git push -u origin main
```

### 5. Test thủ công

Vào **Actions → Vietnam News Crawler → Run workflow** để test trước khi cron chạy.

---

## Thêm/bớt RSS feeds

Sửa trong `main.go`:

```go
var feeds = map[string][]string{
    "vietnam": {
        "https://vnexpress.net/rss/tin-moi-nhat.rss",
        // thêm feed mới vào đây
    },
    "cybersecurity": {
        "https://feeds.feedburner.com/TheHackersNews",
        // thêm feed mới vào đây
    },
}
```

---

## Output mẫu

**Vietnam news:**
```
🇻🇳 Tin Việt Nam | Chính phủ thông qua nghị định mới
VnExpress

📌 Sự kiện: Chính phủ thông qua nghị định sửa đổi Luật Đất đai, siết chặt quy định chuyển nhượng.
👥 Liên quan: Bộ Tài nguyên Môi trường, áp dụng toàn quốc.
📍 Bối cảnh: Ban hành ngày 09/06 tại Hà Nội nhằm giải quyết tranh chấp đất đai kéo dài.

🔗 Đọc thêm
```

**Cybersecurity news:**
```
🔐 Tin Bảo Mật | Critical RCE in Apache Struts
The Hacker News

🔍 Chuyện gì: CVE-2024-53677 trong Apache Struts cho phép upload file độc hại và RCE không cần xác thực.
🎯 Ảnh hưởng: Struts 2.0.0–2.5.32 và 6.0.0–6.3.0.1, đặc biệt hệ thống có file upload endpoint public.
⚠️ Mức độ: Nghiêm trọng — CVSS 9.5, exploit đang được khai thác thực tế.
🛠️ Cần làm gì: Upgrade lên Struts 2.5.33 hoặc 6.3.0.2, workaround là block file upload endpoint.

🔗 Đọc thêm
```
