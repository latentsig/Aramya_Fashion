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
