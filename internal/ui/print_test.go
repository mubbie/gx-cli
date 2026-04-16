package ui

import "testing"

func TestPlural(t *testing.T) {
	if Plural(1) != "" {
		t.Error("Plural(1) should be empty")
	}
	if Plural(0) != "s" {
		t.Error("Plural(0) should be 's'")
	}
	if Plural(2) != "s" {
		t.Error("Plural(2) should be 's'")
	}
	if Plural(100) != "s" {
		t.Error("Plural(100) should be 's'")
	}
}

func TestPluralES(t *testing.T) {
	if PluralES(1) != "" {
		t.Error("PluralES(1) should be empty")
	}
	if PluralES(0) != "es" {
		t.Error("PluralES(0) should be 'es'")
	}
	if PluralES(2) != "es" {
		t.Error("PluralES(2) should be 'es'")
	}
}

func TestPluralIES(t *testing.T) {
	if PluralIES(1) != "y" {
		t.Error("PluralIES(1) should be 'y'")
	}
	if PluralIES(0) != "ies" {
		t.Error("PluralIES(0) should be 'ies'")
	}
	if PluralIES(2) != "ies" {
		t.Error("PluralIES(2) should be 'ies'")
	}
}
