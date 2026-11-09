package services

import (
	"applegacy/backend/internal/core/domain"
	"testing"
)

// Los eventos son en pesos colombianos desde el 2026-11-09. Antes la etiqueta
// pegaba «.000.000» a cualquier cifra: 120 se leía «$120.000.000».
func TestPopulatePriceLabel_EnPesos(t *testing.T) {
	s := &EventService{}
	casos := []struct {
		precio float64
		gratis bool
		quiero string
	}{
		{180000, false, "COP $180.000"},
		{1500, false, "COP $1.500"},
		{120, false, "COP $120"},
		{3150000, false, "COP $3.150.000"},
		{499.6, false, "COP $500"},
		{0, false, "GRATIS"},
		{180000, true, "GRATIS"},
	}
	for _, c := range casos {
		e := &domain.Event{Price: c.precio, IsFree: c.gratis}
		s.populatePriceLabel(e)
		if e.PriceLabel != c.quiero {
			t.Errorf("precio %v gratis %v: %q, se esperaba %q", c.precio, c.gratis, e.PriceLabel, c.quiero)
		}
	}
}
