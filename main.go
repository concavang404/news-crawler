package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

// ── config ────────────────────────────────────────────────────────────────────

var feeds = map[string][]string{
	"vietnam": {
		"https://vnexpress.net/rss/tin-moi-nhat.rss",
		"https://tuoitre.vn/rss/tin-moi-nhat.rss",
		"https://thanhnien.vn/rss/home.rss",
	},
	"cybersecurity": {
		"https://feeds.feedburner.com/TheHackersNews",
		"https://www.bleepingcomputer.com/feed/",
		"https://threatpost.com/feed/",
	},
}

var systemPrompts = map[string]string{
	"vietnam": "Bạn là biên tập viên tin tức. " +
		"Tóm tắt bài báo sau trong 5-8 câu bằng tiếng Việt tự nhiên, đầy đủ chi tiết. " +
		"Nêu rõ: chuyện gì xảy ra, ai liên quan, con số/dữ liệu cụ thể nếu có, " +
		"nguyên nhân hoặc bối cảnh, và kết quả hoặc diễn biến tiếp theo. " +
		"Không dùng gạch đầu dòng hay tiêu đề mục. " +
		"Không thêm thông tin không có trong bài.",
	"cybersecurity": "You are a cybersecurity analyst. " +
		"Summarize the article in 5-8 concise English sentences with full technical detail. " +
		"Cover: what the threat/vulnerability is, attack vector and technique, " +
		"which specific systems/versions/products are affected, who is behind it (if known), " +
		"severity and CVSS score (if available), real-world impact or exploitation status, " +
		"and remediation — only if the article mentions it: include specific patch version, upgrade path, or workaround. " +
		"Keep all technical terms as-is (CVE, RCE, PoC, CVSS, TTPs, IOCs, etc.). " +
		"No bullet points or section headers. " +
		"Do not add information not present in the article.",
}

var labels = map[string]string{
	"vietnam":       "🇻🇳 Tin Việt Nam",
	"cybersecurity": "🔐 Tin Bảo Mật",
}

type botConfig struct {
	token  string
	chatID string
}

var bots = map[string]botConfig{
	"vietnam": {
		token:  os.Getenv("VN_BOT_TOKEN"),
		chatID: os.Getenv("VN_CHAT_ID"),
	},
	"cybersecurity": {
		token:  os.Getenv("SEC_BOT_TOKEN"),
		chatID: os.Getenv("SEC_CHAT_ID"),
	},
}

// delay range between requests (seconds)
const (
	minDelay = 3
	maxDelay = 8
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0",
}

// ── http client ───────────────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func randomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func randomDelay() {
	d := minDelay + rand.Intn(maxDelay-minDelay)
	log.Printf("waiting %ds...", d)
	time.Sleep(time.Duration(d) * time.Second)
}

func fetchPage(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		log.Printf("rate limited on %s, waiting 30s", url)
		time.Sleep(30 * time.Second)
		return "", fmt.Errorf("rate limited")
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return extractText(body), nil
}

// ── html text extractor ───────────────────────────────────────────────────────

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return strings.TrimSpace(sb.String())
}

func extractText(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "footer", "header", "aside", "form":
				return
			}
		}

		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "article") {
			text := nodeText(n)
			if len(text) > 40 {
				sb.WriteString(text + "\n")
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return truncate(sb.String(), 6000)
}

// ── seen store ────────────────────────────────────────────────────────────────

const seenFile = "seen.json"
const maxSeen = 5000

func loadSeen() map[string]bool {
	seen := map[string]bool{}
	data, err := os.ReadFile(seenFile)
	if err != nil {
		return seen
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return seen
	}
	for _, id := range ids {
		seen[id] = true
	}
	return seen
}

func saveSeen(seen map[string]bool) {
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) > maxSeen {
		ids = ids[len(ids)-maxSeen:]
	}
	data, _ := json.Marshal(ids)
	os.WriteFile(seenFile, data, 0644)
}

func articleID(link string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(link)))
}

// ── llm providers ─────────────────────────────────────────────────────────────

type LLMProvider struct {
	name    string
	apiKeys []string
	keyIdx  int
}

func (p *LLMProvider) nextKey() string {
	key := p.apiKeys[p.keyIdx%len(p.apiKeys)]
	p.keyIdx++
	return key
}

func filterEmpty(keys []string) []string {
	var out []string
	for _, k := range keys {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

var providers = []*LLMProvider{
	{
		name: "groq",
		apiKeys: filterEmpty([]string{
			os.Getenv("GROQ_API_KEY_1"),
			os.Getenv("GROQ_API_KEY_2"),
			os.Getenv("GROQ_API_KEY_3"),
		}),
	},
	{
		name: "gemini",
		apiKeys: filterEmpty([]string{
			os.Getenv("GEMINI_API_KEY_1"),
			os.Getenv("GEMINI_API_KEY_2"),
		}),
	},
}

var providerIdx int

func summarize(title, content, category string) (string, error) {
	prompt := fmt.Sprintf("Title: %s\n\n%s", title, content)

	for range providers {
		p := providers[providerIdx%len(providers)]
		providerIdx++

		if len(p.apiKeys) == 0 {
			continue
		}

		var result string
		var err error

		switch p.name {
		case "groq":
			result, err = callGroq(p.nextKey(), systemPrompts[category], prompt)
		case "gemini":
			result, err = callGemini(p.nextKey(), systemPrompts[category], prompt)
		}

		if err != nil {
			log.Printf("[llm] %s failed: %v, trying next provider", p.name, err)
			continue
		}

		log.Printf("[llm] used %s", p.name)
		return result, nil
	}

	return "", fmt.Errorf("all providers failed")
}

// ── groq ──────────────────────────────────────────────────────────────────────

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []groqMessage `json:"messages"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func callGroq(apiKey, systemPrompt, userPrompt string) (string, error) {
	body := groqRequest{
		Model:     "llama-3.3-70b-versatile",
		MaxTokens: 800,
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("groq rate limited")
	}

	var gr groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return "", err
	}
	if gr.Error != nil {
		return "", fmt.Errorf("groq error: %s", gr.Error.Message)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("groq empty response")
	}

	return strings.TrimSpace(gr.Choices[0].Message.Content), nil
}

// ── gemini ────────────────────────────────────────────────────────────────────

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiRequest struct {
	SystemInstruction geminiContent   `json:"system_instruction"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generation_config"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func callGemini(apiKey, systemPrompt, userPrompt string) (string, error) {
	body := geminiRequest{
		SystemInstruction: geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: userPrompt}}},
		},
		GenerationConfig: geminiGenConfig{
			MaxOutputTokens: 800,
		},
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s",
		apiKey,
	)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("gemini rate limited")
	}

	var gr geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return "", err
	}
	if gr.Error != nil {
		return "", fmt.Errorf("gemini error %d: %s", gr.Error.Code, gr.Error.Message)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini empty response")
	}

	return strings.TrimSpace(gr.Candidates[0].Content.Parts[0].Text), nil
}

// ── telegram ──────────────────────────────────────────────────────────────────

type telegramRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

func send(text, category string) error {
	bot := bots[category]
	body := telegramRequest{
		ChatID:                bot.chatID,
		Text:                  text,
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	}

	b, _ := json.Marshal(body)
	resp, err := http.Post(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", bot.token),
		"application/json",
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// ── main ──────────────────────────────────────────────────────────────────────

type article struct {
	title   string
	link    string
	source  string
	summary string
}

func main() {
	// filter to single category if CATEGORY env is set
	category := os.Getenv("CATEGORY")
	targets := feeds
	if category != "" {
		if f, ok := feeds[category]; ok {
			targets = map[string][]string{category: f}
		} else {
			log.Fatalf("unknown category: %s", category)
		}
	}

	seen := loadSeen()
	fp := gofeed.NewParser()

	for cat, feedURLs := range targets {
		var newItems []article

		for _, feedURL := range feedURLs {
			feed, err := fp.ParseURL(feedURL)
			if err != nil {
				log.Printf("[%s] failed to parse %s: %v", cat, feedURL, err)
				continue
			}

			count := 0
			for _, entry := range feed.Items {
				if count >= 5 {
					break
				}

				id := articleID(entry.Link)
				if seen[id] {
					continue
				}
				seen[id] = true

				randomDelay()

				content, err := fetchPage(entry.Link)
				if err != nil {
					log.Printf("[%s] fetch failed %s: %v, falling back to RSS description", cat, entry.Link, err)
					content = entry.Description
				}
				if len(content) < 100 {
					content = entry.Description
				}

				summary, err := summarize(entry.Title, content, cat)
				if err != nil {
					log.Printf("[%s] summarize error: %v", cat, err)
					summary = entry.Title
				}

				newItems = append(newItems, article{
					title:   entry.Title,
					link:    entry.Link,
					source:  feed.Title,
					summary: summary,
				})
				count++
			}

			randomDelay()
		}

		if len(newItems) == 0 {
			log.Printf("[%s] no new articles", cat)
			continue
		}

		if err := send(fmt.Sprintf("<b>%s — %d bài mới</b>", labels[cat], len(newItems)), cat); err != nil {
			log.Printf("[%s] send header error: %v", cat, err)
		}

		for _, item := range newItems {
			msg := fmt.Sprintf(
				"%s | <b>%s</b>\n<i>%s</i>\n\n%s\n\n🔗 <a href='%s'>Đọc thêm</a>",
				labels[cat], item.title, item.source, item.summary, item.link,
			)
			if err := send(msg, cat); err != nil {
				log.Printf("[%s] send error: %v", cat, err)
			}
			log.Printf("[%s] sent: %s", cat, item.title)
		}
	}

	saveSeen(seen)
}
