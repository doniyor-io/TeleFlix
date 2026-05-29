package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"tg-movie-bot/internal/model"
)

type TelegramClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

type TelegramResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Result      json.RawMessage `json:"result"`
}

func NewTelegramClient(token string) *TelegramClient {
	return &TelegramClient{
		token:   token,
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *TelegramClient) SendMessage(
	ctx context.Context,
	chatID int64,
	text string,
) error {

	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendVideo(
	ctx context.Context,
	chatID int64,
	video string,
	caption string,
) error {

	url := fmt.Sprintf("%s/sendVideo", c.baseURL)

	payload := map[string]interface{}{
		"chat_id":            chatID,
		"video":              video,
		"caption":            caption,
		"supports_streaming": true,
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendInlineKeyboard(
	ctx context.Context,
	chatID int64,
	text string,
	buttons [][]model.InlineButton,
) error {

	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"inline_keyboard": buttons,
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendReplyKeyboard(
	ctx context.Context,
	chatID int64,
	text string,
	buttons [][]string,
) error {

	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	var keyboard [][]map[string]interface{}
	for _, row := range buttons {
		var kbRow []map[string]interface{}
		for _, btn := range row {
			kbRow = append(kbRow, map[string]interface{}{"text": btn})
		}
		keyboard = append(keyboard, kbRow)
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"keyboard":          keyboard,
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendContactRequestKeyboard(
	ctx context.Context,
	chatID int64,
	text string,
	buttonText string,
) error {

	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"keyboard": [][]map[string]interface{}{
				{
					{
						"text":            buttonText,
						"request_contact": true,
					},
				},
			},
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SendRemoveKeyboardMessage(
	ctx context.Context,
	chatID int64,
	text string,
) error {

	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]interface{}{
			"remove_keyboard": true,
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) SetMenuButtonForChat(
	ctx context.Context,
	chatID int64,
	webAppURL string,
) error {

	url := fmt.Sprintf("%s/setChatMenuButton", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"menu_button": map[string]interface{}{
			"type": "web_app",
			"text": "Web Panel",
			"web_app": map[string]interface{}{
				"url": webAppURL,
			},
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) ResetMenuButtonForChat(
	ctx context.Context,
	chatID int64,
) error {

	url := fmt.Sprintf("%s/setChatMenuButton", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"menu_button": map[string]interface{}{
			"type": "default",
		},
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) DeleteMessage(
	ctx context.Context,
	chatID int64,
	messageID int,
) error {

	url := fmt.Sprintf("%s/deleteMessage", c.baseURL)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) AnswerCallbackQuery(
	ctx context.Context,
	callbackQueryID string,
) error {

	url := fmt.Sprintf("%s/answerCallbackQuery", c.baseURL)

	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
	}

	return c.postJSON(ctx, url, payload)
}

func (c *TelegramClient) GetChatMember(
	ctx context.Context,
	chatID int64,
	userID int64,
) (bool, error) {

	url := fmt.Sprintf("%s/getChatMember", c.baseURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"user_id": userID,
	}

	body, err := c.postJSONWithResponse(
		ctx,
		url,
		payload,
	)

	if err != nil {
		return false, err
	}

	var response struct {
		OK     bool `json:"ok"`
		Result struct {
			Status   string `json:"status"`
			IsMember bool   `json:"is_member"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return false, err
	}

	status := response.Result.Status

	switch status {
	case "creator", "administrator", "member":
		return true, nil
	case "restricted":
		return response.Result.IsMember, nil
	default:
		return false, nil
	}
}

func (c *TelegramClient) postJSON(
	ctx context.Context,
	url string,
	payload interface{},
) error {

	_, err := c.postJSONWithResponse(
		ctx,
		url,
		payload,
	)

	return err
}

func (c *TelegramClient) postJSONWithResponse(
	ctx context.Context,
	url string,
	payload interface{},
) ([]byte, error) {

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewBuffer(bodyBytes),
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_ = fmt.Errorf("error Closing the Body reader")
		}
	}(resp.Body)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var telegramResponse TelegramResponse

	if err := json.Unmarshal(
		responseBody,
		&telegramResponse,
	); err != nil {
		return nil, err
	}

	if !telegramResponse.OK {
		return nil, fmt.Errorf(
			"telegram api error (%d): %s",
			telegramResponse.ErrorCode,
			telegramResponse.Description,
		)
	}

	return responseBody, nil
}

func (c *TelegramClient) CopyMessage(
	ctx context.Context,
	chatID int64,
	fromChatID int64,
	messageID int64,
) error {
	url := fmt.Sprintf("%s/copyMessage", c.baseURL)
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"from_chat_id": fromChatID,
		"message_id":   messageID,
	}
	_, err := c.postJSONWithResponse(ctx, url, payload)
	return err
}
