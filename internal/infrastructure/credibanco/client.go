package credibanco

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"applegacy/backend/internal/config"
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
)

type credibancoClient struct {
	httpClient *http.Client
	config     *config.Config
}

func NewCredibancoClient(cfg *config.Config) ports.PaymentGateway {
	return &credibancoClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		config:     cfg,
	}
}

type registerResponse struct {
	OrderID      string `json:"orderId"`
	FormURL      string `json:"formUrl"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (c *credibancoClient) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber string, returnUrl string) (string, string, error) {
	endpoint := c.config.CredibanCo.BaseURL + "register.do"

	// CredibanCo amounts are in cents/smallest unit without decimals usually, but as per typical implementations we convert to string
	amountStr := strconv.FormatFloat(amount, 'f', 2, 64)
	amountCentsStr := strings.ReplaceAll(amountStr, ".", "")

	data := url.Values{}
	data.Set("userName", c.config.CredibanCo.Username)
	data.Set("password", c.config.CredibanCo.Password)
	data.Set("orderNumber", orderNumber)
	data.Set("amount", amountCentsStr)
	data.Set("returnUrl", returnUrl)
	data.Set("terminal", c.config.CredibanCo.Terminal)
	data.Set("merchant", c.config.CredibanCo.Merchant)
	data.Set("currency", "170") // 170 for COP

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	var regResp registerResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	if regResp.ErrorCode != "" && regResp.ErrorCode != "0" {
		return "", "", fmt.Errorf("credibanco error %s: %s", regResp.ErrorCode, regResp.ErrorMessage)
	}

	return regResp.OrderID, regResp.FormURL, nil
}

type statusResponse struct {
	OrderStatus  int    `json:"orderStatus"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (c *credibancoClient) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	endpoint := c.config.CredibanCo.BaseURL + "getOrderStatusExtended.do"

	data := url.Values{}
	data.Set("userName", c.config.CredibanCo.Username)
	data.Set("password", c.config.CredibanCo.Password)
	data.Set("orderId", orderId)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var statResp statusResponse
	if err := json.Unmarshal(body, &statResp); err != nil {
		// Try parsing as array if format differs, simple fallback
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if statResp.ErrorCode != "" && statResp.ErrorCode != "0" {
		return domain.TxStatusFailed, fmt.Errorf("credibanco error %s: %s", statResp.ErrorCode, statResp.ErrorMessage)
	}

	// Status codes: 
	// 0 - order registered but not paid
	// 1 - pre-authorized
	// 2 - fully authorized/paid
	// 3 - canceled
	// 4 - refunded
	// 5 - auth via issuer
	// 6 - rejected
	switch statResp.OrderStatus {
	case 2:
		return domain.TxStatusApproved, nil
	case 6:
		return domain.TxStatusDeclined, nil
	case 0, 1:
		return domain.TxStatusPending, nil
	default:
		return domain.TxStatusFailed, nil
	}
}
