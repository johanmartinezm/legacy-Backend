package services

import (
	"applegacy/backend/internal/core/domain"
	"testing"
	"time"
)

// Lo que se prueba aquí no es que se guarde un dato más, sino que quede prueba
// de QUÉ texto aceptó cada persona. Un booleano acredita que hubo aceptación;
// estas columnas acreditan de qué, que es lo que exige el Decreto 1377 de 2013.

func TestSellarConsentimiento_GuardaVersionYFecha(t *testing.T) {
	cuando := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	user := &domain.User{TermsAccepted: true, DataSharingAccepted: true}

	sellarConsentimiento(user, cuando)

	if user.TermsVersion == nil || *user.TermsVersion != domain.TermsVersionVigente {
		t.Errorf("los T&C deben sellarse con la versión vigente %q, se guardó: %v",
			domain.TermsVersionVigente, user.TermsVersion)
	}
	if user.DataSharingVersion == nil || *user.DataSharingVersion != domain.PrivacyVersionVigente {
		t.Errorf("la política debe sellarse con la versión vigente %q, se guardó: %v",
			domain.PrivacyVersionVigente, user.DataSharingVersion)
	}
	if user.TermsAcceptedAt == nil || !user.TermsAcceptedAt.Equal(cuando) {
		t.Errorf("la fecha de aceptación de los T&C no coincide: %v", user.TermsAcceptedAt)
	}
	if user.DataSharingAcceptedAt == nil || !user.DataSharingAcceptedAt.Equal(cuando) {
		t.Errorf("la fecha de aceptación de la política no coincide: %v", user.DataSharingAcceptedAt)
	}
}

func TestSellarConsentimiento_NoSellaLoQueNoSeAcepto(t *testing.T) {
	// Poner fecha y versión sobre un consentimiento negado sería fabricar prueba
	// de lo contrario de lo que ocurrió. Debe quedar en NULL.
	user := &domain.User{TermsAccepted: false, DataSharingAccepted: false}

	sellarConsentimiento(user, time.Now())

	if user.TermsVersion != nil || user.TermsAcceptedAt != nil {
		t.Errorf("sin aceptación no debe sellarse nada, se guardó: %v / %v",
			user.TermsVersion, user.TermsAcceptedAt)
	}
	if user.DataSharingVersion != nil || user.DataSharingAcceptedAt != nil {
		t.Errorf("sin aceptación no debe sellarse nada, se guardó: %v / %v",
			user.DataSharingVersion, user.DataSharingAcceptedAt)
	}
}

func TestSellarConsentimiento_LosDosConsentimientosSonIndependientes(t *testing.T) {
	// Son dos casillas distintas en el registro: aceptar los términos es
	// obligatorio para usar la app, compartir datos con las unidades de negocio
	// es opcional. Sellar la segunda por arrastre de la primera registraría una
	// autorización comercial que nadie dio.
	user := &domain.User{TermsAccepted: true, DataSharingAccepted: false}

	sellarConsentimiento(user, time.Now())

	if user.TermsVersion == nil {
		t.Error("los T&C aceptados deben quedar sellados")
	}
	if user.DataSharingVersion != nil || user.DataSharingAcceptedAt != nil {
		t.Errorf("la política NO se aceptó y no debe sellarse: %v / %v",
			user.DataSharingVersion, user.DataSharingAcceptedAt)
	}
}
