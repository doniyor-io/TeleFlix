package instagramwebhook

type MetaWebhookPayload struct {
	Object string             `json:"object"`
	Entry  []MetaWebhookEntry `json:"entry"`
}

type MetaWebhookEntry struct {
	ID        string               `json:"id"`
	Time      int64                `json:"time"`
	Changes   []MetaWebhookChange  `json:"changes"`
	Messaging []MetaWebhookMessage `json:"messaging"`
}

type MetaWebhookChange struct {
	Field string                 `json:"field"`
	Value MetaWebhookChangeValue `json:"value"`
}

type MetaWebhookChangeValue struct {
	ID        string           `json:"id"`
	MediaID   string           `json:"media_id"`
	MediaType string           `json:"media_type"`
	MediaURL  string           `json:"media_url"`
	Permalink string           `json:"permalink"`
	Caption   string           `json:"caption"`
	Username  string           `json:"username"`
	Timestamp string           `json:"timestamp"`
	CommentID string           `json:"comment_id"`
	Text      string           `json:"text"`
	From      MetaWebhookActor `json:"from"`
}

type MetaWebhookMessage struct {
	Sender    MetaWebhookActor    `json:"sender"`
	Recipient MetaWebhookActor    `json:"recipient"`
	Timestamp int64               `json:"timestamp"`
	Message   MetaWebhookText     `json:"message"`
	Postback  MetaWebhookPostback `json:"postback"`
}

type MetaWebhookActor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type MetaWebhookText struct {
	MID  string `json:"mid"`
	Text string `json:"text"`
}

type MetaWebhookPostback struct {
	MID     string `json:"mid"`
	Title   string `json:"title"`
	Payload string `json:"payload"`
}

type ReelEvent struct {
	Permalink string
	Caption   string
}

func (p MetaWebhookPayload) ReelEvents() []ReelEvent {
	events := make([]ReelEvent, 0)

	for _, entry := range p.Entry {
		for _, change := range entry.Changes {
			if change.Value.Permalink == "" || change.Value.Caption == "" {
				continue
			}

			events = append(events, ReelEvent{
				Permalink: change.Value.Permalink,
				Caption:   change.Value.Caption,
			})
		}
	}

	return events
}
