package credibanco

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"applegacy/backend/internal/config"
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
)

type credibancoClient struct {
	httpClient *http.Client
	config     *config.Config
}

func NewCredibancoClient(cfg *config.Config) ports.PaymentGateway {
	return &credibancoClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		config:     cfg,
	}
}

type registerResponse struct {
	OrderID      string `json:"orderId"`
	FormURL      string `json:"formUrl"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

// jsonParams identifica quién llama. El banco lo ve en sus registros, y tenerlo
// ayuda cuando hay que reclamarles algo: sabe distinguir la app del WooCommerce
// del mismo comercio.
const jsonParamsApp = `{"CMS":"Legacy App","Module-Version":"1.0"}`

func (c *credibancoClient) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber string, returnUrl string) (string, string, error) {
	endpoint := c.config.CredibanCo.BaseURL + "register.do"

	// El importe viaja en centavos, sin separador decimal. Se redondea sobre
	// enteros en vez de formatear a texto y quitarle el punto: 25.55 en coma
	// flotante es 25.549999…, y por la vía del texto acababa en 2554.
	//
	// OJO: multiplicar por 100 es lo correcto según ISO 4217 para el peso
	// colombiano, pero varias integraciones locales esperan pesos enteros. La
	// primera transacción real debe ser de importe mínimo hasta confirmarlo en
	// el extracto; con un evento de 250.000 el error se cobra cien veces.
	amountCents := strconv.FormatInt(int64(math.Round(amount*100)), 10)

	data := url.Values{}
	data.Set("userName", c.config.CredibanCo.Username)
	data.Set("password", c.config.CredibanCo.Password)
	data.Set("orderNumber", orderNumber)
	data.Set("amount", amountCents)
	data.Set("returnUrl", returnUrl)
	data.Set("language", "es")
	data.Set("jsonParams", jsonParamsApp)

	// NO se envían `terminal`, `merchant` ni `currency`, y esto es deliberado.
	//
	// El plugin de WooCommerce del mismo comercio —que sí funciona contra esta
	// pasarela— no manda ninguno de los tres, y en su código la línea de
	// `currency` está comentada a propósito, no ausente por descuido.
	//
	// En la API de RBS, sobre la que corre CredibanCo, `merchant` lo envían los
	// agregadores que facturan en nombre de terceros. Un comercio normal que lo
	// incluye pide una operación para la que su usuario no tiene permiso, y la
	// respuesta a eso es `errorCode 5`, "acceso denegado" — que es exactamente
	// el error que bloqueaba los pagos. La divisa la fija el comercio en la
	// pasarela.
	//
	// `config.credibanco.terminal` y `.merchant` siguen existiendo porque
	// identifican el comercio ante el banco y hacen falta para hablar con
	// soporte, pero no viajan en esta llamada.

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	var regResp registerResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	if regResp.ErrorCode != "" && regResp.ErrorCode != "0" {
		return "", "", fmt.Errorf("credibanco error %s: %s", regResp.ErrorCode, regResp.ErrorMessage)
	}

	return regResp.OrderID, regResp.FormURL, nil
}

type statusResponse struct {
	OrderStatus  int    `json:"orderStatus"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (c *credibancoClient) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	endpoint := c.config.CredibanCo.BaseURL + "getOrderStatusExtended.do"

	data := url.Values{}
	data.Set("userName", c.config.CredibanCo.Username)
	data.Set("password", c.config.CredibanCo.Password)
	data.Set("orderId", orderId)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var statResp statusResponse
	if err := json.Unmarshal(body, &statResp); err != nil {
		// Try parsing as array if format differs, simple fallback
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if statResp.ErrorCode != "" && statResp.ErrorCode != "0" {
		return domain.TxStatusFailed, fmt.Errorf("credibanco error %s: %s", statResp.ErrorCode, statResp.ErrorMessage)
	}

	// Estados de la pasarela:
	// 0 - registrado, sin pagar
	// 1 - preautorizado (importe retenido, cobro sin confirmar)
	// 2 - pagado
	// 3 - anulado
	// 4 - devuelto
	// 5 - autorización iniciada por el emisor
	// 6 - rechazado
	switch statResp.OrderStatus {
	case 2:
		return domain.TxStatusApproved, nil
	case 6:
		return domain.TxStatusDeclined, nil
	case 1:
		// El plugin de WooCommerce da por bueno el 1, y aquí NO, a propósito.
		//
		// Preautorizado significa dinero retenido y cobro sin confirmar. Darlo
		// por pagado inscribiría a alguien de quien todavía no se ha cobrado, y
		// una inscripción no se deshace tan fácil como una retención.
		//
		// El plugin puede permitírselo porque ofrece el modo de dos pasos
		// (registerPreAuth.do); nosotros registramos con register.do, donde el
		// 1 no debería aparecer nunca. Si aparece, conviene enterarse.
		log.Printf("[PAGO] CredibanCo devolvió orderStatus=1 (preautorizado) para la orden %s: "+
			"queda pendiente y sin inscribir. Con register.do no debería ocurrir.", orderId)
		return domain.TxStatusPending, nil
	case 0:
		return domain.TxStatusPending, nil
	default:
		return domain.TxStatusFailed, nil
	}
}
