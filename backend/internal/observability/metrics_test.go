package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInventoryReservationMetricUsesOnlyBoundedOutcomes(t *testing.T) {
	metrics := New()
	metrics.RecordInventoryReservation("reserved")
	metrics.RecordInventoryReservation("rejected")
	metrics.RecordInventoryReservation("failed")
	metrics.RecordInventoryReservation("variant-9d6d7a1b")

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`aramya_inventory_reservation_attempts_total{outcome="reserved"} 1`,
		`aramya_inventory_reservation_attempts_total{outcome="rejected"} 1`,
		`aramya_inventory_reservation_attempts_total{outcome="failed"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metric sample %q missing from scrape", expected)
		}
	}
	if strings.Contains(body, "variant-9d6d7a1b") {
		t.Fatal("unbounded identifier was exposed as a metric label")
	}
}

func TestPaymentWebhookMetricUsesOnlyBoundedOutcomes(t *testing.T) {
	metrics := New()
	metrics.RecordPaymentWebhook("processed")
	metrics.RecordPaymentWebhook("duplicate")
	metrics.RecordPaymentWebhook("invalid_signature")
	metrics.RecordPaymentWebhook("failed")
	metrics.RecordPaymentWebhook("evt_sensitive_123456")
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`aramya_payment_webhook_events_total{outcome="processed"} 1`,
		`aramya_payment_webhook_events_total{outcome="duplicate"} 1`,
		`aramya_payment_webhook_events_total{outcome="invalid_signature"} 1`,
		`aramya_payment_webhook_events_total{outcome="failed"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metric sample %q missing from scrape", expected)
		}
	}
	if strings.Contains(body, "evt_sensitive_123456") {
		t.Fatal("webhook metric exposed an event ID as a label")
	}
}
