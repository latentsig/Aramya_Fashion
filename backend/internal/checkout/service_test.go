package checkout

import "testing"

func validDetails() Details {
	return Details{Email: "customer@example.test", Phone: "+919876543210", Address: Address{RecipientName: "Customer", Phone: "+919876543210", Line1: "12 Example Road", City: "Ankola", Region: "Karnataka", PostalCode: "581314", CountryCode: "IN"}}
}

func TestValidateCheckoutDetails(t *testing.T) {
	if err := validateDetails(validDetails(), false); err != nil {
		t.Fatalf("valid guest checkout details rejected: %v", err)
	}
	if err := validateDetails(Details{Phone: "+919876543210", Address: validDetails().Address}, true); err != nil {
		t.Fatalf("valid authenticated checkout without repeated email rejected: %v", err)
	}
	invalid := validDetails()
	invalid.Address.CountryCode = "IND"
	if err := validateDetails(invalid, false); err == nil {
		t.Fatal("invalid country code accepted")
	}
	invalid = validDetails()
	invalid.Email = "not an email"
	if err := validateDetails(invalid, false); err == nil {
		t.Fatal("invalid guest email accepted")
	}
}

func TestNormalizeDetails(t *testing.T) {
	details := validDetails()
	details.Email = " Customer@Example.Test "
	details.Address.CountryCode = "in "
	details.Address.City = " Ankola "
	got, err := normalizeDetails(details, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "customer@example.test" || got.Address.CountryCode != "IN" || got.Address.City != "Ankola" {
		t.Fatalf("checkout normalization mismatch: %#v", got)
	}
}
