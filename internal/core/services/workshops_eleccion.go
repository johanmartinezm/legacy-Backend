package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Elección de workshops al inscribirse (§10 del plan de material de talleres,
// decidido el 2026-11-05).
//
// La regla de negocio vive aquí y no en la app: la app pinta la misma hoja,
// pero quien decide si una elección vale es el servidor.
//
//   - En cada franja de cada día se elige exactamente un workshop.
//   - Una franja son los workshops del mismo día, en hora de Colombia, cuyos
//     horarios se cruzan, aunque sea en parte.
//   - Los workshops no tienen cupo: la franja solo «se llena» para quien ya
//     eligió en ella.
//   - Se puede elegir o cambiar hasta el día anterior al evento. Desde las 00:00
//     del primer día, nadie, y la app tampoco puede inscribir a nadie.
//   - Calificar un workshop exige haberlo elegido, salvo en las inscripciones
//     que no tienen ninguno elegido (las de antes de esta función, las del panel
//     y las de la carga masiva).

var (
	// ErrInscripcionesCerradas: desde las 00:00 del primer día del evento nadie
	// se inscribe desde la app. El panel y la carga masiva sí pueden.
	ErrInscripcionesCerradas = errors.New("las inscripciones a este evento ya cerraron")
	// ErrEleccionCerrada: desde las 00:00 del primer día ya no se eligen ni se
	// cambian workshops, se hayan elegido o no.
	ErrEleccionCerrada = errors.New("la elección de workshops de este evento ya cerró")
	// ErrSinInscripcion: para elegir workshops hay que estar inscrito al evento.
	ErrSinInscripcion = errors.New("primero tienes que inscribirte al evento")
	// ErrWorkshopNoElegido: la inscripción eligió workshops y este no es uno de
	// ellos, así que no se puede calificar.
	ErrWorkshopNoElegido = errors.New("solo puedes calificar los workshops que elegiste")
)

// ErrorEleccion es una elección que no cumple las reglas. Motivo va tal cual a
// la persona, en español, así que dice qué falta o qué sobra.
type ErrorEleccion struct {
	Motivo string
}

func (e *ErrorEleccion) Error() string { return e.Motivo }

// franja agrupa los workshops de un día que se cruzan en horario.
type franja struct {
	inicio, fin time.Time
	workshops   []domain.Workshop
}

// diaEnColombia es el día del calendario de un instante, en Colombia.
func diaEnColombia(t time.Time) string {
	return t.In(colombia).Format("2006-01-02")
}

// calcularFranjas agrupa los workshops en franjas. Ordena por inicio y va
// alargando la franja mientras el siguiente empiece antes de que termine la
// actual: uno de 9:00 a 10:00 y otro de 9:30 a 11:00 caen juntos, y uno que
// empieza justo a las 10:00, cuando acaba el anterior, ya no.
//
// Una franja nunca cruza de un día a otro, aunque un workshop acabe pasada la
// medianoche: el corte es el día en que empieza cada uno.
func calcularFranjas(workshops []domain.Workshop) []franja {
	ordenados := append([]domain.Workshop(nil), workshops...)
	sort.SliceStable(ordenados, func(i, j int) bool {
		return ordenados[i].StartDateTime.Before(ordenados[j].StartDateTime)
	})

	var franjas []franja
	for _, w := range ordenados {
		if n := len(franjas); n > 0 {
			actual := &franjas[n-1]
			mismoDia := diaEnColombia(actual.inicio) == diaEnColombia(w.StartDateTime)
			if mismoDia && w.StartDateTime.Before(actual.fin) {
				actual.workshops = append(actual.workshops, w)
				if w.EndDateTime.After(actual.fin) {
					actual.fin = w.EndDateTime
				}
				continue
			}
		}
		franjas = append(franjas, franja{
			inicio:    w.StartDateTime,
			fin:       w.EndDateTime,
			workshops: []domain.Workshop{w},
		})
	}
	return franjas
}

// describirFranja la nombra para un mensaje de error: «el 16/09 de 09:00 a 11:00».
func describirFranja(f franja) string {
	return fmt.Sprintf("el %s de %s a %s",
		f.inicio.In(colombia).Format("02/01"),
		f.inicio.In(colombia).Format("15:04"),
		f.fin.In(colombia).Format("15:04"))
}

// eleccionAbierta dice si todavía se pueden elegir workshops del evento: hasta
// el día anterior al primer día. start_date es una columna `date` y llega como
// medianoche UTC, así que se compara como día, igual que eventoTerminado.
func eleccionAbierta(evento *domain.Event, ahora time.Time) bool {
	return hoyEnColombia(ahora) < evento.StartDate.UTC().Format("2006-01-02")
}

// idsUnicos quita repetidos y vacíos sin cambiar el orden.
func idsUnicos(ids []string) []string {
	vistos := make(map[string]bool, len(ids))
	var unicos []string
	for _, id := range ids {
		if id == "" || vistos[id] {
			continue
		}
		vistos[id] = true
		unicos = append(unicos, id)
	}
	return unicos
}

// validarPertenencia comprueba que cada id sea un workshop de ese evento. Rige
// para todos los caminos, también el panel: sin esto se guardaban ids de otro
// evento, o inventados, y la puerta los mostraba.
func validarPertenencia(delEvento []domain.Workshop, elegidos []string) error {
	existe := make(map[string]bool, len(delEvento))
	for _, w := range delEvento {
		existe[w.ID] = true
	}
	for _, id := range elegidos {
		if !existe[id] {
			return &ErrorEleccion{Motivo: "uno de los workshops elegidos no es de este evento"}
		}
	}
	return nil
}

// validarEleccion aplica la regla completa de la app: los ids son del evento y
// hay exactamente uno en cada franja.
func validarEleccion(delEvento []domain.Workshop, elegidos []string) error {
	if err := validarPertenencia(delEvento, elegidos); err != nil {
		return err
	}
	marcado := make(map[string]bool, len(elegidos))
	for _, id := range elegidos {
		marcado[id] = true
	}

	for _, f := range calcularFranjas(delEvento) {
		var enFranja []string
		for _, w := range f.workshops {
			if marcado[w.ID] {
				enFranja = append(enFranja, w.Name)
			}
		}
		switch {
		case len(enFranja) == 0:
			return &ErrorEleccion{Motivo: "falta elegir un workshop " + describirFranja(f)}
		case len(enFranja) > 1:
			return &ErrorEleccion{Motivo: fmt.Sprintf(
				"«%s» y «%s» se cruzan %s: elige solo uno",
				enFranja[0], enFranja[1], describirFranja(f))}
		}
	}
	return nil
}

// prepararWorkshopsDeInscripcion decide qué workshops se guardan con una
// inscripción nueva. Desde la app aplica la regla completa; desde el panel solo
// comprueba que sean del evento, porque el administrador está exento de elegir
// (§10.6, punto 4).
func (s *EventService) prepararWorkshopsDeInscripcion(ctx context.Context, reg *domain.Registration) ([]string, error) {
	var elegidos []string
	for _, w := range reg.Workshops {
		elegidos = append(elegidos, w.ID)
	}
	elegidos = idsUnicos(elegidos)

	if !reg.DesdeApp && len(elegidos) == 0 {
		return nil, nil
	}

	delEvento, err := s.repo.GetWorkshopsByEventID(ctx, reg.EventID)
	if err != nil {
		return nil, err
	}
	if reg.DesdeApp {
		return elegidos, validarEleccion(delEvento, elegidos)
	}
	return elegidos, validarPertenencia(delEvento, elegidos)
}

// ElegirMisWorkshops reemplaza los workshops elegidos de la inscripción del
// usuario a ese evento. Se puede hasta el día anterior al evento, con las
// mismas reglas que al inscribirse; desde el primer día, nadie.
//
// Reemplaza la lista entera y no suma: es lo que la app manda desde la hoja, y
// así cambiar un workshop por otro de la misma franja es una sola llamada.
func (s *EventService) ElegirMisWorkshops(ctx context.Context, userID, eventID string, ids []string) error {
	reg, err := s.repo.GetRegistrationByUserAndEvent(ctx, userID, eventID)
	if err != nil {
		return err
	}
	if reg == nil {
		return ErrSinInscripcion
	}

	evento, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		return err
	}
	if !eleccionAbierta(evento, s.ahora()) {
		return ErrEleccionCerrada
	}

	delEvento, err := s.repo.GetWorkshopsByEventID(ctx, eventID)
	if err != nil {
		return err
	}
	elegidos := idsUnicos(ids)
	if err := validarEleccion(delEvento, elegidos); err != nil {
		return err
	}
	return s.repo.ReplaceRegistrationWorkshops(ctx, reg.ID, elegidos)
}

// comprobarWorkshopElegido deja calificar solo lo que se eligió. Una inscripción
// sin ningún workshop elegido califica como antes (§10.6, punto 3): así no se
// quedan sin poder hacerlo los inscritos del Summit de septiembre, los del panel
// ni los de la carga masiva.
func (s *EventService) comprobarWorkshopElegido(ctx context.Context, userID string, w *domain.Workshop) error {
	elegidos, err := s.repo.GetChosenWorkshopIDs(ctx, userID, w.EventID)
	if err != nil {
		return err
	}
	if len(elegidos) == 0 {
		return nil
	}
	for _, id := range elegidos {
		if id == w.ID {
			return nil
		}
	}
	return ErrWorkshopNoElegido
}
