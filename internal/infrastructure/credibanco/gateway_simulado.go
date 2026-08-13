package credibanco

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
)

// GatewaySimulado imita a CredibanCo sin cobrar nada.
//
// Existe porque la pasarela real lleva bloqueada desde el 2026-08-06 con
// "acceso denegado", y sin ella no había forma de probar **el resto del flujo**:
// crear la intención, salir al navegador, volver por el deep link, recibir la
// notificación y confirmar la inscripción. De los diez fallos analizados en
// agosto, ocho estaban en ese recorrido y no en la pasarela.
//
// Lo que NO valida, y conviene no olvidarlo: el comportamiento real del banco.
// Ni la redirección, ni el formato exacto de su notificación, ni —sobre todo—
// si el importe se interpreta multiplicado por 100 o en pesos enteros. Eso solo
// lo cierra una transacción real de importe mínimo.
type GatewaySimulado struct {
	mu      sync.Mutex
	ordenes map[string]*ordenSimulada
	baseURL string
}

type ordenSimulada struct {
	importe   float64
	returnURL string
	estado    domain.TransactionStatus
}

// SimuladoEsPeligroso dice si encender el modo simulado con esta URL permitiría
// aprobar pagos sin cobrar allí donde importa.
//
// La comprobación es por lista blanca —solo se permite en UAT y en local— y no
// por lista negra del dominio de producción: si mañana la pasarela real cambia
// de nombre, una lista negra dejaría de proteger sin que nadie se entere,
// mientras que ésta falla del lado seguro.
func SimuladoEsPeligroso(baseURL string) bool {
	entornosDePrueba := []string{"ecouat", "localhost", "127.0.0.1", "sandbox", "test"}
	for _, permitido := range entornosDePrueba {
		if strings.Contains(baseURL, permitido) {
			return false
		}
	}
	// Sin URL no hay a quién cobrar de verdad, así que no hay peligro.
	return strings.TrimSpace(baseURL) != ""
}

// NuevoGatewaySimulado construye la pasarela de mentira. `baseURL` es la
// dirección pública de este backend, la que abrirá el navegador del teléfono.
func NuevoGatewaySimulado(baseURL string) *GatewaySimulado {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &GatewaySimulado{
		ordenes: make(map[string]*ordenSimulada),
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

var _ ports.PaymentGateway = (*GatewaySimulado)(nil)

func (g *GatewaySimulado) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber string, returnUrl string) (string, string, error) {
	orderID := "SIM-" + uuid.New().String()

	g.mu.Lock()
	g.ordenes[orderID] = &ordenSimulada{
		importe:   amount,
		returnURL: returnUrl,
		estado:    domain.TxStatusPending,
	}
	g.mu.Unlock()

	log.Printf("[PAGO SIMULADO] orden %s creada por %.2f (referencia %s)", orderID, amount, orderNumber)

	return orderID, fmt.Sprintf("%s/api/payments/simulado/%s", g.baseURL, orderID), nil
}

func (g *GatewaySimulado) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	orden, ok := g.ordenes[orderId]
	if !ok {
		// Igual que la pasarela real ante una orden que no conoce.
		return domain.TxStatusFailed, fmt.Errorf("orden simulada desconocida: %s", orderId)
	}
	return orden.estado, nil
}

// RegistrarRutas cuelga la pasarela de mentira del router. Solo se llama cuando
// el modo simulado está encendido: si no, estas rutas no existen.
func (g *GatewaySimulado) RegistrarRutas(r chi.Router) {
	r.Get("/api/payments/simulado/{orderID}", g.mostrarPantalla)
	r.Get("/api/payments/simulado/{orderID}/resolver", g.resolver)
}

var plantillaPantalla = template.Must(template.New("simulado").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pasarela de prueba</title>
<style>
 body { margin:0; background:#0B1A2E; color:#fff; font-family:system-ui,sans-serif;
        display:flex; min-height:100vh; align-items:center; justify-content:center; }
 .caja { max-width:22rem; padding:2rem 1.5rem; text-align:center; }
 .aviso { font-size:.75rem; letter-spacing:.1em; text-transform:uppercase; color:#D9A74A; }
 h1 { font-size:1.4rem; margin:.8rem 0 .3rem; }
 .importe { font-size:2.4rem; font-weight:700; margin:1rem 0; }
 .ref { font-size:.75rem; color:#8fa3bb; word-break:break-all; margin-bottom:1.5rem; }
 a { display:block; padding:.9rem; border-radius:10px; text-decoration:none;
     font-weight:600; margin-bottom:.7rem; }
 .ok { background:#2C6A4C; color:#fff; }
 .no { background:#A32918; color:#fff; }
 .nada { background:transparent; color:#8fa3bb; border:1px solid #2a3d55; }
</style></head><body><div class="caja">
 <p class="aviso">Pasarela de prueba · no se cobra nada</p>
 <h1>Pago de prueba</h1>
 <div class="importe">$ {{.Importe}}</div>
 <p class="ref">{{.OrderID}}</p>
 <a class="ok" href="{{.Base}}/api/payments/simulado/{{.OrderID}}/resolver?resultado=aprobado">Aprobar el pago</a>
 <a class="no" href="{{.Base}}/api/payments/simulado/{{.OrderID}}/resolver?resultado=rechazado">Rechazar el pago</a>
 <a class="nada" href="{{.Base}}/api/payments/simulado/{{.OrderID}}/resolver?resultado=pendiente">Dejarlo pendiente</a>
</div></body></html>`))

func (g *GatewaySimulado) mostrarPantalla(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")

	g.mu.Lock()
	orden, ok := g.ordenes[orderID]
	g.mu.Unlock()

	if !ok {
		http.Error(w, "orden desconocida", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = plantillaPantalla.Execute(w, map[string]any{
		"OrderID": orderID,
		"Importe": fmt.Sprintf("%.2f", orden.importe),
		"Base":    g.baseURL,
	})
}

func (g *GatewaySimulado) resolver(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")

	g.mu.Lock()
	orden, ok := g.ordenes[orderID]
	if ok {
		switch r.URL.Query().Get("resultado") {
		case "aprobado":
			orden.estado = domain.TxStatusApproved
		case "rechazado":
			orden.estado = domain.TxStatusDeclined
		default:
			orden.estado = domain.TxStatusPending
		}
	}
	g.mu.Unlock()

	if !ok {
		http.Error(w, "orden desconocida", http.StatusNotFound)
		return
	}

	log.Printf("[PAGO SIMULADO] orden %s resuelta como %s", orderID, orden.estado)

	// Se devuelve al punto por el que la app espera volver, igual que haría la
	// pasarela real. Ahí es donde se comprueba que el deep link funciona.
	http.Redirect(w, r, orden.returnURL, http.StatusFound)
}
