// Package youtube lee los videos publicados en un canal con la API de datos de
// YouTube v3.
//
// Solo lee material público, así que basta una clave de API: no hace falta
// OAuth, que únicamente se necesita para datos privados o para publicar.
package youtube

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseURL = "https://www.googleapis.com/youtube/v3"

	// maxPorPagina es el tope que admite playlistItems.list. Pedir menos no
	// abarata la llamada —cuesta 1 unidad igual— y obliga a más páginas.
	maxPorPagina = 50

	// tiempoDeEspera acota cada llamada. Sin él, una petición colgada a Google
	// bloquearía la respuesta de nuestra propia API.
	tiempoDeEspera = 10 * time.Second
)

// Client habla con la API de datos de YouTube.
type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: tiempoDeEspera},
	}
}

// VideosDelCanal devuelve las últimas subidas de un canal.
//
// Son dos llamadas de **1 unidad cada una**, más una por página extra:
// `channels.list` da la lista de subidas del canal y `playlistItems.list` la
// recorre. La alternativa evidente, `search.list`, **cuesta 100 unidades** y con
// una cuota diaria de 10.000 la agotaría con un puñado de usuarios.
func (c *Client) VideosDelCanal(ctx context.Context, handle string, max int) ([]domain.VideoDeCanal, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("youtube: falta la clave de api")
	}
	if max <= 0 {
		max = maxPorPagina
	}

	playlistID, err := c.listaDeSubidas(ctx, handle)
	if err != nil {
		return nil, err
	}

	var videos []domain.VideoDeCanal
	pageToken := ""

	for len(videos) < max {
		restantes := max - len(videos)
		porPagina := maxPorPagina
		if restantes < porPagina {
			porPagina = restantes
		}

		pagina, siguiente, err := c.paginaDeLista(ctx, playlistID, pageToken, porPagina)
		if err != nil {
			return nil, err
		}
		videos = append(videos, pagina...)

		if siguiente == "" || len(pagina) == 0 {
			break
		}
		pageToken = siguiente
	}

	return videos, nil
}

// listaDeSubidas resuelve el handle del canal a su playlist de subidas.
//
// Se usa `forHandle` y no el identificador `UC…` porque el handle es lo que
// aparece en la URL del canal y lo que la gente tiene a mano; la API lo acepta
// desde 2023. La página pública del canal no expone el identificador —se pinta
// por JavaScript—, así que sacarlo de ahí no era opción.
func (c *Client) listaDeSubidas(ctx context.Context, handle string) (string, error) {
	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}

	params := url.Values{}
	params.Set("part", "contentDetails")
	params.Set("forHandle", handle)
	params.Set("key", c.apiKey)

	var respuesta struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := c.pedir(ctx, "channels", params, &respuesta); err != nil {
		return "", err
	}
	if len(respuesta.Items) == 0 {
		return "", fmt.Errorf("youtube: no existe el canal %s", handle)
	}

	uploads := respuesta.Items[0].ContentDetails.RelatedPlaylists.Uploads
	if uploads == "" {
		return "", fmt.Errorf("youtube: el canal %s no tiene lista de subidas", handle)
	}
	return uploads, nil
}

func (c *Client) paginaDeLista(ctx context.Context, playlistID, pageToken string, max int) ([]domain.VideoDeCanal, string, error) {
	params := url.Values{}
	params.Set("part", "snippet")
	params.Set("playlistId", playlistID)
	params.Set("maxResults", fmt.Sprintf("%d", max))
	params.Set("key", c.apiKey)
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	var respuesta struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			Snippet struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				PublishedAt string `json:"publishedAt"`
				// VideoOwnerChannelTitle es el canal dueño del video. En una
				// lista de subidas coincide con ChannelTitle, pero se prefiere
				// este porque el otro es el dueño de la *lista*.
				VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
				ChannelTitle           string `json:"channelTitle"`
				ResourceID             struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
				Thumbnails map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := c.pedir(ctx, "playlistItems", params, &respuesta); err != nil {
		return nil, "", err
	}

	videos := make([]domain.VideoDeCanal, 0, len(respuesta.Items))
	for _, item := range respuesta.Items {
		id := item.Snippet.ResourceID.VideoID
		if id == "" {
			// Un video borrado o privado sigue apareciendo en la lista sin id
			// utilizable. Enlazarlo llevaría a una página de error.
			continue
		}

		canal := item.Snippet.VideoOwnerChannelTitle
		if canal == "" {
			canal = item.Snippet.ChannelTitle
		}

		publicado, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)

		videos = append(videos, domain.VideoDeCanal{
			ID:           id,
			Title:        item.Snippet.Title,
			Description:  item.Snippet.Description,
			VideoURL:     "https://www.youtube.com/watch?v=" + id,
			ThumbnailURL: mejorMiniatura(item.Snippet.Thumbnails),
			Channel:      canal,
			PublishedAt:  publicado,
		})
	}

	return videos, respuesta.NextPageToken, nil
}

// mejorMiniatura escoge la mayor disponible. No todos los videos traen todas las
// resoluciones, y una miniatura pequeña estirada en una tarjeta se ve mal.
func mejorMiniatura(miniaturas map[string]struct {
	URL string `json:"url"`
}) string {
	for _, calidad := range []string{"maxres", "standard", "high", "medium", "default"} {
		if m, ok := miniaturas[calidad]; ok && m.URL != "" {
			return m.URL
		}
	}
	return ""
}

func (c *Client) pedir(ctx context.Context, recurso string, params url.Values, destino any) error {
	direccion := fmt.Sprintf("%s/%s?%s", baseURL, recurso, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("youtube: %s: %w", recurso, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// El cuerpo del error trae el motivo (clave inválida, cuota agotada,
		// restricción de IP), y sin él estos fallos son indistinguibles. No
		// lleva la clave: va en la URL, que no se registra.
		var detalle struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&detalle)
		return fmt.Errorf("youtube: %s: HTTP %d: %s", recurso, resp.StatusCode, detalle.Error.Message)
	}

	return json.NewDecoder(resp.Body).Decode(destino)
}
