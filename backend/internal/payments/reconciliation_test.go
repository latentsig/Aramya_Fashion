package payments

import "testing"

func TestSelectReconciliationPaymentRequiresExactOrderAmountAndCurrency(t *testing.T) {
	candidate := reconciliationCandidate{providerOrderID: "order_test_123456", amount: 125050, currency: "INR"}
	tests := []struct {
		name         string
		storedID     string
		payments     []Payment
		wantStatus   string
		wantID       string
		wantCaptures int
		wantValid    bool
	}{
		{name: "captured outranks authorization", payments: []Payment{
			{ID: "pay_test_111111", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "authorized"},
			{ID: "pay_test_222222", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "captured", Captured: true},
		}, wantStatus: "captured", wantID: "pay_test_222222", wantCaptures: 1, wantValid: true},
		{name: "preserve known payment among multiple captures", storedID: "pay_test_222222", payments: []Payment{
			{ID: "pay_test_111111", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "captured", Captured: true},
			{ID: "pay_test_222222", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "captured", Captured: true},
		}, wantStatus: "captured", wantID: "pay_test_222222", wantCaptures: 2, wantValid: true},
		{name: "authorization remains unresolved", payments: []Payment{
			{ID: "pay_test_111111", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "authorized"},
			{ID: "pay_test_222222", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "failed"},
		}, wantStatus: "authorized", wantID: "pay_test_111111", wantCaptures: 0, wantValid: true},
		{name: "all failed can fail attempt", payments: []Payment{
			{ID: "pay_test_111111", OrderID: candidate.providerOrderID, Amount: candidate.amount, Currency: candidate.currency, Status: "failed"},
		}, wantStatus: "failed", wantID: "pay_test_111111", wantCaptures: 0, wantValid: true},
		{name: "empty list stays pending", wantValid: true},
		{name: "amount mismatch rejected", payments: []Payment{
			{ID: "pay_test_111111", OrderID: candidate.providerOrderID, Amount: candidate.amount + 1, Currency: candidate.currency, Status: "captured", Captured: true},
		}, wantValid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate.providerPaymentID = test.storedID
			payment, status, captureCount, valid := selectReconciliationPayment(test.payments, candidate)
			if valid != test.wantValid || status != test.wantStatus || captureCount != test.wantCaptures {
				t.Fatalf("selection status=%q captures=%d valid=%t; want %q captures=%d valid=%t", status, captureCount, valid, test.wantStatus, test.wantCaptures, test.wantValid)
			}
			if test.wantID == "" && payment != nil {
				t.Fatalf("unexpected payment selected: %+v", *payment)
			}
			if test.wantID != "" && (payment == nil || payment.ID != test.wantID) {
				t.Fatalf("selected payment=%+v; want ID %s", payment, test.wantID)
			}
		})
	}
}
