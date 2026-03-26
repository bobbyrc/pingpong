package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AppriseClient struct {
	baseURL string
	urls    string
	client  *http.Client
}

type appriseRequest struct {
	URLs  string `json:"urls"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Type  string `json:"type"`
}

func NewAppriseClient(baseURL, urls string) *AppriseClient {
	return &AppriseClient{
		baseURL: baseURL,
		urls:    urls,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *AppriseClient) send(title, body string) error {
	payload := appriseRequest{
		URLs:  a.urls,
		Title: title,
		Body:  body,
		Type:  "warning",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal apprise request: %w", err)
	}

	resp, err := a.client.Post(
		a.baseURL+"/notify",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("send apprise notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apprise returned status %d", resp.StatusCode)
	}

	return nil
}
