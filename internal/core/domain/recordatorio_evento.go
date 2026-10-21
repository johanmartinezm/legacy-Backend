package domain

// EventoPorRecordar es un evento que empieza hoy y aún no ha tenido su aviso de
// la mañana. Lleva solo lo que hace falta para escribir la notificación.
type EventoPorRecordar struct {
	ID        string
	Title     string
	Location  *string
	IsVirtual bool
}
