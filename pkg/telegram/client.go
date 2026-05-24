package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"tg-movie-bot/internal/model"
	"time"
)

type TelegramClient struct {
	token      string
	apiURL     string
	httpClient *http.Client
}

type botAPIResponse struct {
	Ok          bool            `json:"ok"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Result      json.RawMessage `json:"result"`
}

func NewTelegramClient(token string) *TelegramClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &TelegramClient{
		token:  token,
		apiURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

func (c *TelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("%s/sendMessage", c.apiURL)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendVideo(ctx context.Context, chatID int64, fileID string, caption string) error {
	url := fmt.Sprintf("%s/sendVideo", c.apiURL)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"video":   fileID,
		"caption": caption,
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) IsChatMember(ctx context.Context, channelID int64, userID int64) (bool, error) {
	url := fmt.Sprintf("%s/getChatMember", c.apiURL)
	payload := map[string]interface{}{
		"chat_id": channelID,
		"user_id": userID,
	}

	body, err := c.postJSONWithResponse(ctx, url, payload)
	if err != nil {
		return false, err
	}

	var resp struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Status   string `json:"status"`
			IsMember *bool  `json:"is_member,omitempty"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("json unmarshal error: %w", err)
	}

	if !resp.Ok {
		return false, fmt.Errorf("telegram getChatMember error: %s", resp.Description)
	}

	status := resp.Result.Status

	if status == "creator" || status == "administrator" || status == "member" {
		return true, nil
	}

	if status == "restricted" {
		if resp.Result.IsMember != nil && *resp.Result.IsMember {
			return true, nil
		}
	}

	return false, nil
}

func (c *TelegramClient) SendInlineKeyboard(ctx context.Context, chatID int64, text string, buttons [][]model.InlineButton) error {
	url := fmt.Sprintf("%s/sendMessage", c.apiURL)

	var inlineKeyboard [][]map[string]interface{}
	for _, row := range buttons {
		var btnRow []map[string]interface{}
		for _, btn := range row {
			b := map[string]interface{}{
				"text": btn.Text,
			}

			if btn.URL != "" {
				if btn.IsWebApp {
					b["web_app"] = map[string]interface{}{
						"url": btn.URL,
					}
				} else {
					b["url"] = btn.URL
				}
			} else if btn.Data != "" {
				b["callback_data"] = btn.Data
			}

			btnRow = append(btnRow, b)
		}
		inlineKeyboard = append(inlineKeyboard, btnRow)
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"inline_keyboard": inlineKeyboard,
		},
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendReplyKeyboard(ctx context.Context, chatID int64, text string, buttons [][]string) error {
	url := fmt.Sprintf("%s/sendMessage", c.apiURL)

	var keyboard [][]map[string]interface{}
	for _, row := range buttons {
		var btnRow []map[string]interface{}
		for _, btnText := range row {
			btnRow = append(btnRow, map[string]interface{}{
				"text": btnText,
			})
		}
		keyboard = append(keyboard, btnRow)
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"keyboard":        keyboard,
			"resize_keyboard": true,
			"is_persistent":   true,
		},
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SetMenuButtonForChat(ctx context.Context, chatID int64, webAppURL string) error {
	url := fmt.Sprintf("%s/setChatMenuButton", c.apiURL)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"menu_button": map[string]interface{}{
			"type": "web_app",
			"text": "Open",
			"web_app": map[string]interface{}{
				"url": webAppURL,
			},
		},
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) AnswerCallbackQuery(ctx context.Context, callbackID string) error {
	url := fmt.Sprintf("%s/answerCallbackQuery", c.apiURL)
	payload := map[string]interface{}{
		"callback_query_id": callbackID,
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	url := fmt.Sprintf("%s/deleteMessage", c.apiURL)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) postJSON(ctx context.Context, url string, payload interface{}) error {
	_, err := c.postJSONWithResponse(ctx, url, payload)
	return err
}

func (c *TelegramClient) postJSONWithResponse(ctx context.Context, url string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[TG API ERROR] request failed: %s: %v", url, err)
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		log.Printf("[TG API ERROR] body read failed: %s: %v", url, err)
		return nil, err
	}

	body := buf.Bytes()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("telegram API bad status: %d body=%s", resp.StatusCode, string(body))
		log.Printf("[TG API ERROR] %v", err)
		return nil, err
	}

	var apiResp botAPIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && !apiResp.Ok {
		err = fmt.Errorf("telegram API error %d: %s", apiResp.ErrorCode, apiResp.Description)
		log.Printf("[TG API ERROR] endpoint=%s error=%v payload=%s", url, err, string(data))
		return nil, err
	}

	return body, nil
}
