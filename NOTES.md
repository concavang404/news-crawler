# News Crawler — Notes & Decisions

## Configuration

| Setting | Value |
|---|---|
| Vietnam schedule | Every hour (`0 * * * *`) |
| Cybersecurity schedule | Every hour (`0 * * * *`) |
| Articles per feed per run | Max 5 |
| Article content fed to LLM | 4,000 chars |
| Max output tokens (Groq & Gemini) | 800 |
| Seen articles cap | 5,000 entries |

## LLM Providers

- **Primary**: Groq — `llama-3.3-70b-versatile` (supports up to 3 keys: `GROQ_API_KEY_1/2/3`)
- **Fallback**: Gemini — `gemini-2.0-flash` (supports up to 2 keys: `GEMINI_API_KEY_1/2`)
- Keys rotate round-robin; if one fails, the next provider is tried automatically
- Multiple keys from the same account share the same quota — only useful across different accounts

## GitHub Secrets Required

| Secret | Description |
|---|---|
| `VN_BOT_TOKEN` | Telegram bot token for Vietnam channel |
| `VN_CHAT_ID` | Telegram chat ID for Vietnam channel |
| `SEC_BOT_TOKEN` | Telegram bot token for cybersecurity channel |
| `SEC_CHAT_ID` | Telegram chat ID for cybersecurity channel |
| `GROQ_API_KEY_1` | Groq API key (required) |
| `GROQ_API_KEY_2` | Groq API key (optional) |
| `GROQ_API_KEY_3` | Groq API key (optional) |
| `GEMINI_API_KEY_1` | Gemini API key (optional fallback) |
| `GEMINI_API_KEY_2` | Gemini API key (optional fallback) |

## News Sources

### 🇻🇳 Vietnam
- VnExpress: `https://vnexpress.net/rss/tin-moi-nhat.rss`
- Tuổi Trẻ: `https://tuoitre.vn/rss/tin-moi-nhat.rss`
- Thanh Niên: `https://thanhnien.vn/rss/home.rss`

### 🔐 Cybersecurity
- The Hacker News: `https://feeds.feedburner.com/TheHackersNews`
- BleepingComputer: `https://www.bleepingcomputer.com/feed/`
- Threatpost: `https://threatpost.com/feed/` ⚠️ inactive since 2023, consider replacing

## Summary Prompts

### Vietnam
- Language: Vietnamese
- Length: 5–8 sentences, natural prose
- Covers: what happened, who is involved, specific numbers/data, cause/context, follow-up

### Cybersecurity
- Language: English
- Length: 5–8 sentences, natural prose
- Covers: threat/vulnerability, attack vector, affected systems/versions, who is behind it, CVSS score, exploitation status, remediation (patch/upgrade/workaround — only if mentioned in article)

## Changes Made

### Bug Fixes
- Removed deprecated `rand.Seed` (auto-seeded since Go 1.20)
- Fixed `extractText` to recursively collect all text inside `<p>`/`<article>` nodes (previously only read direct first-child text, missing nested elements)
- Added error logging for Telegram header message send

### Improvements
- `seen.json` pruned to max 5,000 entries on each save to prevent unbounded growth
- Upgraded Groq model from `llama-3.1-8b-instant` to `llama-3.3-70b-versatile` for better Vietnamese and reasoning quality
- Updated Gemini model from `gemini-1.5-flash` (deprecated/404) to `gemini-2.0-flash`
- Rewrote both prompts from rigid 3/4-section format to natural prose for better output quality
- Cybersecurity summaries switched to English
- Increased max output tokens from 400 → 800
- Increased article content input from 3,000 → 3,500 chars
- Remediation only included in cybersecurity summary when article actually mentions it
- Both crawlers set to run every hour

## Trigger Manually
```bash
gh workflow run vietnam_news.yml --repo concavang404/news-crawler
gh workflow run security_news.yml --repo concavang404/news-crawler
```
