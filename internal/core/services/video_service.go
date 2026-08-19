package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// VideoService reúne los videos de los canales configurados para la sección de
// contenido de la app.
//
// **Cachea en memoria y esa es su razón de ser.** La cuota diaria de la API de
// datos de YouTube es de 10.000 unidades: sin caché, cada persona que abre la
// pantalla gastaría una llamada por canal y unas pocas decenas de usuarios
// dejarían la sección muerta hasta el día siguiente. Con la caché el gasto es de
// dos llamadas por canal y hora, pase quien pase.
//
// La caché vive en el proceso, igual que el hub de chat. El backend ya no se
// puede escalar horizontalmente por ese motivo, así que esto no añade una
// limitación nueva; si algún día se escala, esta caché pasa a Redis con el hub.
type VideoService struct {
	canal   ports.CanalDeVideos
	handles []string
	max     int

	mu          sync.RWMutex
	cache       []domain.VideoDeCanal
	cacheHasta  time.Time
	duracionTTL time.Duration
}

const (
	// ttlPorDefecto: un canal de esta comunidad publica cada pocos días, así
	// que una hora de retraso no se nota y recorta el gasto de cuota a nada.
	ttlPorDefecto = time.Hour

	// maxPorCanalPorDefecto. Suficiente para llenar la sección sin pedir el
	// historial entero de un canal en cada refresco.
	maxPorCanalPorDefecto = 50
)

func NewVideoService(canal ports.CanalDeVideos, handles []string, max int) *VideoService {
	if max <= 0 {
		max = maxPorCanalPorDefecto
	}
	return &VideoService{
		canal:       canal,
		handles:     handles,
		max:         max,
		duracionTTL: ttlPorDefecto,
	}
}

// ListarVideos devuelve los videos de todos los canales, del más reciente al más
// antiguo.
//
// **Nunca devuelve error.** La sección de contenido tiene otras dos fuentes y no
// puede quedarse en blanco porque YouTube esté caído, la cuota agotada o la
// clave mal restringida: en ese caso se registra el fallo y se devuelve lo que
// haya, aunque sea nada.
func (s *VideoService) ListarVideos(ctx context.Context) ([]domain.VideoDeCanal, error) {
	if s.canal == nil || len(s.handles) == 0 {
		return []domain.VideoDeCanal{}, nil
	}

	if videos, vigente := s.deCache(); vigente {
		return videos, nil
	}

	var todos []domain.VideoDeCanal
	fallos := 0

	for _, handle := range s.handles {
		videos, err := s.canal.VideosDelCanal(ctx, handle, s.max)
		if err != nil {
			// Un canal que falla no puede dejar sin videos a los demás.
			log.Printf("[videos] %s: %v", handle, err)
			fallos++
			continue
		}
		todos = append(todos, videos...)
	}

	// Si fallaron todos, se conserva lo que hubiera en caché aunque esté
	// caducado: material viejo es mejor que una sección vacía.
	if fallos == len(s.handles) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return append([]domain.VideoDeCanal{}, s.cache...), nil
	}

	todos = deduplicar(todos)
	sort.Slice(todos, func(i, j int) bool {
		return todos[i].PublishedAt.After(todos[j].PublishedAt)
	})

	s.guardar(todos)
	return todos, nil
}

func (s *VideoService) deCache() ([]domain.VideoDeCanal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if time.Now().Before(s.cacheHasta) && s.cache != nil {
		return append([]domain.VideoDeCanal{}, s.cache...), true
	}
	return nil, false
}

func (s *VideoService) guardar(videos []domain.VideoDeCanal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = videos
	s.cacheHasta = time.Now().Add(s.duracionTTL)
}

// deduplicar evita que un video subido a los dos canales salga dos veces. El id
// es el de YouTube, así que identifica el video y no la fila.
func deduplicar(videos []domain.VideoDeCanal) []domain.VideoDeCanal {
	vistos := make(map[string]bool, len(videos))
	unicos := make([]domain.VideoDeCanal, 0, len(videos))
	for _, v := range videos {
		if vistos[v.ID] {
			continue
		}
		vistos[v.ID] = true
		unicos = append(unicos, v)
	}
	return unicos
}
