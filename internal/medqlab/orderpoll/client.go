package orderpoll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const saveOrderPath = "/api/v1/saveOrder"

type Client struct {
	BaseURL    string
	Cons       string
	Sign       string
	HTTPClient *http.Client
}

func NewClient(baseURL, cons, sign string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Cons:    cons,
		Sign:    sign,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type saveOrderAPIResponse struct {
	MetaData *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"metaData"`
	Response *struct {
		NoLaboratorium      string `json:"no_laboratorium"`
		NoLaboratoriumCamel string `json:"noLaboratorium"`
	} `json:"response"`
}

// SaveOrder POSTs the ApiMEDQLAB body. X-Time is sent only for ranap.
func (c *Client) SaveOrder(ctx context.Context, body SaveOrderBody, inpatient bool) (string, error) {
	if c == nil || c.HTTPClient == nil {
		return "", fmt.Errorf("medqlab client is not configured")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+saveOrderPath, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-cons", c.Cons)
	req.Header.Set("X-sign", c.Sign)
	if inpatient {
		req.Header.Set("X-Time", strconv.FormatInt(time.Now().Unix(), 10))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read saveOrder response: %w", err)
	}

	var parsed saveOrderAPIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("saveOrder HTTP %d: invalid JSON: %w; body=%s", resp.StatusCode, err, truncate(respBody, 500))
	}
	if parsed.MetaData == nil {
		return "", fmt.Errorf("saveOrder HTTP %d: missing metaData; body=%s", resp.StatusCode, truncate(respBody, 500))
	}
	if !codeIs200(parsed.MetaData.Code) {
		msg := strings.TrimSpace(parsed.MetaData.Message)
		if msg == "" {
			msg = fmt.Sprintf("metaData.code=%v", parsed.MetaData.Code)
		}
		return "", fmt.Errorf("%s", msg)
	}
	labNo := ""
	if parsed.Response != nil {
		labNo = strings.TrimSpace(parsed.Response.NoLaboratorium)
		if labNo == "" {
			labNo = strings.TrimSpace(parsed.Response.NoLaboratoriumCamel)
		}
	}
	return labNo, nil
}

func codeIs200(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case float64:
		return int(t) == 200
	case json.Number:
		n, err := t.Int64()
		return err == nil && n == 200
	case string:
		return strings.TrimSpace(t) == "200"
	default:
		return fmt.Sprint(t) == "200"
	}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
