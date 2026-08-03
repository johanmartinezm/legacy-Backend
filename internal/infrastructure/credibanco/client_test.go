package credibanco

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"applegacy/backend/internal/config"
	"applegacy/backend/internal/core/domain"
)

func TestCreatePaymentIntent(t *testing.T) {
	// Mock server for register.do
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/register.do" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"orderId": "test-order-123", "formUrl": "https://test.form.url", "errorCode": "0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	client := NewCredibancoClient(cfg)
	orderID, formUrl, err := client.CreatePaymentIntent(context.Background(), 1000.50, "txn-1", "https://return.url")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if orderID != "test-order-123" {
		t.Errorf("Expected orderId test-order-123, got %s", orderID)
	}
	if formUrl != "https://test.form.url" {
		t.Errorf("Expected formUrl https://test.form.url, got %s", formUrl)
	}
}

func TestGetPaymentStatus(t *testing.T) {
	// Mock server for getOrderStatusExtended.do
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getOrderStatusExtended.do" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"orderStatus": 2, "errorCode": "0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	client := NewCredibancoClient(cfg)
	status, err := client.GetPaymentStatus(context.Background(), "test-order-123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if status != domain.TxStatusApproved {
		t.Errorf("Expected status APPROVED, got %s", status)
	}
}
