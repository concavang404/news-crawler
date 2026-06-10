# SECRETS.md — Hướng dẫn setup API keys & Secrets

## 1. Telegram Bots

### Tạo bot

1. Mở Telegram → tìm `@BotFather`
2. Gõ `/newbot`
3. Đặt tên hiển thị, ví dụ: `Vietnam News Bot`
4. Đặt username (phải kết thúc bằng `bot`), ví dụ: `my_vn_news_bot`
5. BotFather trả về **Bot Token** dạng: `7123456789:AAFxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

Làm lại để tạo bot thứ 2 cho Security news.

### Lấy Chat ID

**Với group:**
1. Add bot vào group
2. Set bot làm Admin (để có quyền gửi tin)
3. Gửi bất kỳ tin nhắn nào vào group
4. Truy cập: `https://api.telegram.org/bot<TOKEN>/getUpdates`
5. Tìm `"chat": {"id": -1001234567890}` — đó là Chat ID (số âm với group)

**Với channel:**
1. Add bot vào channel làm Admin
2. Chat ID dạng `@ten_channel` hoặc số âm `-100xxxxxxxxxx`

---

## 2. Groq API Keys

1. Truy cập https://console.groq.com
2. Đăng ký tài khoản (miễn phí)
3. Vào **API Keys → Create API Key**
4. Copy key dạng: `gsk_xxxxxxxxxxxxxxxxxxxxxxxxxxxx`

**Tạo 3 keys từ 3 tài khoản khác nhau** để có 3× free tier:
- Tài khoản 1 → `GROQ_API_KEY_1`
- Tài khoản 2 → `GROQ_API_KEY_2`
- Tài khoản 3 → `GROQ_API_KEY_3`

Free tier: 14,400 requests/ngày/key

---

## 3. Gemini API Keys

1. Truy cập https://aistudio.google.com
2. Đăng nhập bằng Google account
3. Vào **Get API Key → Create API key**
4. Copy key dạng: `AIzaSyxxxxxxxxxxxxxxxxxxxxxxxxxx`

**Tạo 2 keys từ 2 Google account khác nhau:**
- Account 1 → `GEMINI_API_KEY_1`
- Account 2 → `GEMINI_API_KEY_2`

Free tier: 1,500 requests/ngày/key

---

## 4. Thêm Secrets vào GitHub

1. Vào repo GitHub
2. **Settings → Secrets and variables → Actions**
3. Click **New repository secret**
4. Thêm lần lượt các secrets sau:

| Name | Value |
|---|---|
| `VN_BOT_TOKEN` | Token bot Vietnam news |
| `VN_CHAT_ID` | Chat ID group Vietnam news |
| `SEC_BOT_TOKEN` | Token bot Security news |
| `SEC_CHAT_ID` | Chat ID group Security news |
| `GROQ_API_KEY_1` | Groq key từ account 1 |
| `GROQ_API_KEY_2` | Groq key từ account 2 |
| `GROQ_API_KEY_3` | Groq key từ account 3 |
| `GEMINI_API_KEY_1` | Gemini key từ Google account 1 |
| `GEMINI_API_KEY_2` | Gemini key từ Google account 2 |

---

## 5. Kiểm tra secrets hoạt động

Sau khi add secrets, chạy thủ công:

1. Vào tab **Actions** trên GitHub
2. Chọn **Vietnam News Crawler**
3. Click **Run workflow → Run workflow**
4. Xem log để confirm không có lỗi auth

Nếu thấy log `[llm] used groq` hoặc `[llm] used gemini` và bot gửi được tin vào Telegram là thành công.

---

## 6. Lưu ý bảo mật

- Không commit API keys vào code
- Không share secrets ra ngoài
- Nếu key bị lộ: revoke ngay trên console của provider và tạo key mới
- GitHub Secrets được mã hóa, chỉ accessible trong Actions runner
