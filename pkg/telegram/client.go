package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"tg-movie-bot/internal/model"
	"time"
)

type TelegramClient struct {
	token      string
	apiURL     string
	httpClient *http.Client
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
		Ok     bool `json:"ok"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, err
	}

	if !resp.Ok {
		return false, fmt.Errorf("telegram getChatMember error")
	}

	status := resp.Result.Status
	return status == "creator" || status == "administrator" || status == "member", nil
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
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram API bad status: %d", resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// SendInlineKeyboard - Sends a message with native inline keyboard buttons
func (c *TelegramClient) SendInlineKeyboard(ctx context.Context, chatID int64, text string, buttons [][]model.InlineButton) error {
	url := fmt.Sprintf("%s/sendMessage", c.apiURL)

	var inlineKeyboard [][]map[string]interface{}
	for _, row := range buttons {
		var btnRow []map[string]interface{}
		for _, btn := range row {
			btnRow = append(btnRow, map[string]interface{}{
				"text":          btn.Text,
				"callback_data": btn.Data,
			})
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

// SendWebAppButton - Sends a message with a WebApp button to initialize TMA session
func (c *TelegramClient) SendWebAppButton(ctx context.Context, chatID int64, text string, yesText, webAppURL, noText, noData string) error {
	url := fmt.Sprintf("%s/sendMessage", c.apiURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{
						"text": yesText,
						"web_app": map[string]interface{}{
							"url": webAppURL,
						},
					},
					{
						"text":          noText,
						"callback_data": noData,
					},
				},
			},
		},
	}

	return c.postJSON(ctx, url, payload)
}

// AnswerCallbackQuery - Acknowledges the callback query to remove button loading state
func (c *TelegramClient) AnswerCallbackQuery(ctx context.Context, callbackID string) error {
	url := fmt.Sprintf("%s/answerCallbackQuery", c.apiURL)

	payload := map[string]interface{}{
		"callback_query_id": callbackID,
	}

	return c.postJSON(ctx, url, payload)
}

// DeleteMessage - Expunges a specific message from the chat logs
func (c *TelegramClient) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	url := fmt.Sprintf("%s/deleteMessage", c.apiURL)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}

	return c.postJSON(ctx, url, payload)
}
