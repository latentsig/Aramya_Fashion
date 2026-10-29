package admin

import (
	"errors"
	"testing"
)

const testCatalogCategoryID = "fd5a5719-8447-4a0b-9d7b-bcf609e00ad1"

func validProductWrite() ProductWrite {
	return ProductWrite{
		SKU: "AR-TEST-01", Slug: "test-saree", Name: "Test Saree", Description: "A test garment",
		CategoryID: testCatalogCategoryID, PricePaise: 1250000, ImageKey: "image assests/saree1.jpg",
		Status: "draft", InventoryPolicy: "made_to_order",
		Variants: []VariantWrite{{SKU: "AR-TEST-01-MTO", OptionName: "Fit", OptionValue: "Made to measure", Active: true, IsDefault: true}},
	}
}

func TestValidateProductWrite(t *testing.T) {
	tests := []struct {
		name     string
		edit     func(*ProductWrite)
		creating bool
		valid    bool
	}{
		{name: "valid create", creating: true, valid: true},
		{name: "valid versioned edit", edit: func(p *ProductWrite) { p.ExpectedVersion = 2 }, valid: true},
		{name: "reject invalid category UUID", edit: func(p *ProductWrite) { p.CategoryID = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" }, creating: true},
		{name: "reject slug uppercase", edit: func(p *ProductWrite) { p.Slug = "Test Saree" }, creating: true},
		{name: "reject zero active tracked price", edit: func(p *ProductWrite) { p.Status = "active"; p.InventoryPolicy = "tracked"; p.PricePaise = 0 }, creating: true},
		{name: "requires active default variant", edit: func(p *ProductWrite) { p.Variants[0].Active = false }, creating: true},
		{name: "requires exactly one default", edit: func(p *ProductWrite) { p.Variants[0].IsDefault = false }, creating: true},
		{name: "reject duplicate variant option", edit: func(p *ProductWrite) {
			p.Variants = append(p.Variants, VariantWrite{SKU: "AR-TEST-01-S", OptionName: "Fit", OptionValue: "Made to measure", Active: true})
		}, creating: true},
		{name: "reject a variant below zero total price", edit: func(p *ProductWrite) { p.Variants[0].PriceAdjustmentPaise = -1250001 }, creating: true},
		{name: "reject stale edit with no version", edit: func(*ProductWrite) {}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validProductWrite()
			if test.edit != nil {
				test.edit(&input)
			}
			err := ValidateProductWrite(input, test.creating)
			if (err == nil) != test.valid {
				t.Fatalf("error=%v, valid=%v", err, test.valid)
			}
		})
	}
}

func TestNormalizeProductWriteAndValidateCategory(t *testing.T) {
	input := validProductWrite()
	input.Name, input.SKU, input.ImageKey = " Test Saree ", " AR-TEST-01 ", " image assests/saree1.jpg "
	input.Variants[0].OptionValue = " Made to measure "
	clean := NormalizeProductWrite(input)
	if clean.Name != "Test Saree" || clean.SKU != "AR-TEST-01" || clean.Variants[0].OptionValue != "Made to measure" {
		t.Fatalf("normalized input=%+v", clean)
	}
	if err := ValidateProductWrite(clean, true); err != nil {
		t.Fatalf("normalized product invalid: %v", err)
	}
	category := CategoryWrite{Slug: "sarees", Name: "Sarees", Subtitle: "A drape for every celebration", ImageKey: "image assests/saree1.jpg", SortOrder: 1, Active: true}
	if err := ValidateCategoryWrite(category, true); err != nil {
		t.Fatalf("valid category: %v", err)
	}
	category.ExpectedVersion = 1
	if err := ValidateCategoryWrite(category, false); err != nil {
		t.Fatalf("versioned category: %v", err)
	}
	category.Slug = "Upper Case"
	if !errors.Is(ValidateCategoryWrite(category, false), ErrInvalidCatalog) {
		t.Fatal("invalid category slug was accepted")
	}
}
