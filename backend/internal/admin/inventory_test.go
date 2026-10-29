package admin

import "testing"

func TestValidateInventoryQuantity(t *testing.T) {
	for _, test := range []struct {
		value int
		valid bool
	}{
		{value: -1},
		{value: 0, valid: true},
		{value: 1000000, valid: true},
		{value: 1000001},
	} {
		if err := ValidateInventoryQuantity(test.value); (err == nil) != test.valid {
			t.Errorf("ValidateInventoryQuantity(%d) error=%v, valid=%v", test.value, err, test.valid)
		}
	}
}
