package email

import (
	"strings"
	"testing"
)

func TestCuerpoCorreoAsesoriaEscapaLoDelUsuario(t *testing.T) {
	cuerpo := cuerpoCorreoAsesoria(
		`Ana <b>`,
		"ana@x.co",
		"OTRO",
		"Necesidad: Otro\n\nDetalle del caso:\n<a href=\"https://malo.example\">pague aquí</a> & más",
	)

	if strings.Contains(cuerpo, "<a href") || strings.Contains(cuerpo, "<b>") {
		t.Fatalf("el HTML del usuario llegó sin escapar:\n%s", cuerpo)
	}
	for _, esperado := range []string{
		"&lt;a href=&#34;https://malo.example&#34;&gt;pague aquí&lt;/a&gt; &amp; más",
		"Ana &lt;b&gt;",
		"Detalle del caso:",
		"OTRO",
	} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %q en el cuerpo", esperado)
		}
	}
}
