package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"
)

// IdentidadApple es quien inicia sesión según su token de Apple.
type IdentidadApple struct {
	// Sujeto es el claim `sub`, el único identificador estable: el correo
	// puede faltar o ser una dirección de retransmisión privada.
	Sujeto string
	Correo string
}

// ValidadorDeApple comprueba un token de Sign in with Apple. Es una interfaz
// para poder ejercitar el inicio de sesión sin llamar a Apple.
type ValidadorDeApple interface {
	Validar(ctx context.Context, idToken string) (*IdentidadApple, error)
}

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmailBlindIndex(ctx context.Context, blindIndex string) (*domain.User, error)
	// FindBySocialID busca por la identidad del proveedor ("google" o "apple").
	// Es la única forma fiable con Apple, que solo manda el correo la primera vez.
	FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error)
	// LinkSocialID deja constancia de con qué cuenta social entra alguien.
	LinkSocialID(ctx context.Context, userID, provider, socialID string) error
	FindAll(ctx context.Context) ([]*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
	// AnonymizeUser vacía los datos personales conservando la fila. Es lo que
	// hay detrás de "eliminar mi cuenta": borrarla de verdad arrastraría en
	// cascada los chats, transacciones y encuestas de otras personas.
	AnonymizeUser(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, userID, newHash string) error
	UpdatePasswordByEmail(ctx context.Context, email, newHash string) error
	// MarkEmailAsVerified recibe el id, no el blind index: quien lo llama viene
	// de validar un token, que ya identifica a la persona por id.
	MarkEmailAsVerified(ctx context.Context, userID string) error
}

type PasswordResetRepository interface {
	StoreToken(ctx context.Context, email, token string) error
	GetToken(ctx context.Context, email string) (string, error)
	DeleteToken(ctx context.Context, email string) error
}

type CryptoService interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(encodedCiphertext string) (string, error)
	BlindIndex(input string) string
}

type LikeRepository interface {
	ToggleLike(ctx context.Context, userID, postID string) (bool, error)
	GetLikeStatus(ctx context.Context, userID, postID string) (*domain.LikeStatus, error)
	RecordView(ctx context.Context, userID, postID, title string) error
}

// AdminUserRepository manages admin accounts separate from regular users.
type AdminUserRepository interface {
	Create(ctx context.Context, admin *domain.AdminUser) error
	FindByEmail(ctx context.Context, email string) (*domain.AdminUser, error)
	FindByID(ctx context.Context, id string) (*domain.AdminUser, error)
	List(ctx context.Context) ([]*domain.AdminUser, error)
	Update(ctx context.Context, admin *domain.AdminUser) error
	Delete(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, newHash string) error
}

type AuthService interface {
	Register(ctx context.Context, user *domain.User, password string) error
	Login(ctx context.Context, email, password string) (string, error) // Returns JWT
	SocialLogin(ctx context.Context, provider, idToken string) (string, *domain.User, error)
	RegisterAdmin(ctx context.Context, admin *domain.AdminUser, password string) error
	AdminLogin(ctx context.Context, email, password string) (string, error)
	ListAdmins(ctx context.Context) ([]*domain.AdminUser, error)
	UpdateAdmin(ctx context.Context, admin *domain.AdminUser) error
	DeleteAdmin(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	DeleteUser(ctx context.Context, id string) error
	GetProfile(ctx context.Context, id string) (*domain.User, error)
	// DeleteMyAccount es "eliminar mi cuenta" desde la app: anonimiza al usuario
	// que lo pide. Distinto de DeleteUser, que es la baja administrativa.
	DeleteMyAccount(ctx context.Context, userID string) error
	ChangePassword(ctx context.Context, id string, oldPassword, newPassword string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, email, token, newPassword string) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerificationEmail(ctx context.Context, email string) error
}

type LikeService interface {
	ToggleLike(ctx context.Context, userID, postID string) (*domain.LikeStatus, error)
	GetLikeStatus(ctx context.Context, userID, postID string) (*domain.LikeStatus, error)
	RecordView(ctx context.Context, userID, postID, title string) error
}

type EmailService interface {
	SendResetPasswordEmail(to, resetURL string) error
	SendBoardContactEmail(to, senderName, senderEmail, messageText string) error
	SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error
	SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error
	SendWelcomeEmail(to, userName string) error
	SendVerificationEmail(to, link string) error
	// SendEventRegistrationEmail confirma una inscripción. Recibe una estructura
	// y no siete parámetros sueltos porque el contenido cambia con la modalidad
	// del evento: el virtual lleva enlace de acceso y el presencial remite a la
	// credencial de la app.
	SendEventRegistrationEmail(datos domain.CorreoInscripcion) error
}

// EmailVerificationRepository identifica a la persona por su id, que es lo que
// core.email_verification_tokens guarda de verdad (user_id, con clave foránea a
// core.users). Hasta el 2026-08-10 la interfaz hablaba de emailBlindIndex y las
// consultas iban contra una columna que no existe: todo registro con correo y
// contraseña moría con SQLSTATE 42703.
type EmailVerificationRepository interface {
	StoreToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	// ValidateToken devuelve el id de la persona dueña del token.
	ValidateToken(ctx context.Context, token string) (string, error)
	DeleteToken(ctx context.Context, token string) error
}

type ChatRepository interface {
	CreateConnection(ctx context.Context, requesterID, receiverID string) error
	UpdateConnectionStatus(ctx context.Context, connectionID string, status domain.ConnectionStatus) error
	GetConnection(ctx context.Context, connectionID string) (*domain.ChatConnection, error)
	FindConnectionBetweenUsers(ctx context.Context, user1, user2 string) (*domain.ChatConnection, error)
	ListConnections(ctx context.Context, userID string) ([]*domain.ChatConnection, error)
	SaveMessage(ctx context.Context, msg *domain.Message) error
	GetMessages(ctx context.Context, connectionID string, limit, offset int) ([]*domain.Message, error)
	MarkAsRead(ctx context.Context, connectionID, userID string) error
	// ListMembers recibe quién mira: el directorio se filtra por bloqueos, así
	// que no hay una única lista de miembros igual para todos.
	ListMembers(ctx context.Context, viewerID string) ([]*domain.User, error)
}

type ChatService interface {
	SendInvite(ctx context.Context, requesterID, receiverID string) error
	AcceptInvite(ctx context.Context, connectionID, userID string) error
	RejectInvite(ctx context.Context, connectionID, userID string) error
	ListMyConnections(ctx context.Context, userID string) ([]*domain.ChatConnection, error)
	GetChatHistory(ctx context.Context, connectionID, userID string, limit, offset int) ([]*domain.Message, error)
	SendMessage(ctx context.Context, senderID, connectionID, content string) (*domain.Message, error)
	ListMembers(ctx context.Context, viewerID string) ([]*domain.User, error)
}

type BannerRepository interface {
	Create(ctx context.Context, banner *domain.Banner) error
	Update(ctx context.Context, banner *domain.Banner) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.Banner, error)
	ListActive(ctx context.Context, category string) ([]*domain.Banner, error)
	ListAll(ctx context.Context) ([]*domain.Banner, error)
}

type BannerService interface {
	CreateBanner(ctx context.Context, banner *domain.Banner) error
	UpdateBanner(ctx context.Context, banner *domain.Banner) error
	DeleteBanner(ctx context.Context, id string) error
	GetActiveBanners(ctx context.Context, category string) ([]*domain.Banner, error)
	ListAllBanners(ctx context.Context) ([]*domain.Banner, error)
}

type ContentCategoryRepository interface {
	Create(ctx context.Context, cat *domain.ContentCategory) error
	ListAll(ctx context.Context) ([]*domain.ContentCategory, error)
	Update(ctx context.Context, cat *domain.ContentCategory) error
	Delete(ctx context.Context, id string) error
}

type CustomContentRepository interface {
	Create(ctx context.Context, c *domain.CustomContent) error
	Update(ctx context.Context, c *domain.CustomContent) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, categorySlug string, onlyPublished bool) ([]*domain.CustomContent, error)
	GetByID(ctx context.Context, id string) (*domain.CustomContent, error)
}

type ContentService interface {
	ListCategories(ctx context.Context) ([]*domain.ContentCategory, error)
	CreateCategory(ctx context.Context, cat *domain.ContentCategory) error
	UpdateCategory(ctx context.Context, cat *domain.ContentCategory) error
	DeleteCategory(ctx context.Context, id string) error

	ListContent(ctx context.Context, categorySlug string, onlyPublished bool) ([]*domain.CustomContent, error)
	GetContentByID(ctx context.Context, id string) (*domain.CustomContent, error)
	CreateContent(ctx context.Context, c *domain.CustomContent) error
	UpdateContent(ctx context.Context, c *domain.CustomContent) error
	DeleteContent(ctx context.Context, id string) error
}

type StatsRepository interface {
	GetTopArticles(ctx context.Context, limit int) ([]domain.ArticleStat, error)
	GetTopUsers(ctx context.Context, limit int) ([]domain.UserStat, error)
	GetViewsByPeriod(ctx context.Context, period string) ([]domain.PeriodStats, error)
}

type StatsService interface {
	GetDashboardStats(ctx context.Context) (*domain.DashboardStats, error)
}

type GroupRepository interface {
	Create(ctx context.Context, group *domain.CustomGroup) error
	List(ctx context.Context) ([]*domain.CustomGroup, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.CustomGroup, error)
	AddMembers(ctx context.Context, groupID string, userIDs []string) error
	RemoveMember(ctx context.Context, groupID string, userID string) error
	GetMembers(ctx context.Context, groupID string) ([]string, error)
	ReplaceMembers(ctx context.Context, groupID string, userIDs []string) error
}

type GroupService interface {
	CreateGroup(ctx context.Context, name, description string) (*domain.CustomGroup, error)
	ListGroups(ctx context.Context) ([]*domain.CustomGroup, error)
	DeleteGroup(ctx context.Context, id string) error
	GetGroupByID(ctx context.Context, id string) (*domain.CustomGroup, error)
	GetMembers(ctx context.Context, groupID string) ([]string, error)
	ReplaceMembers(ctx context.Context, groupID string, userIDs []string) error
}

// CanalDeVideos consulta los videos publicados en un canal externo.
//
// La interfaz habla de "canal" y no de YouTube para que el servicio no dependa
// del proveedor: lo que necesita es una lista de videos, y de dónde salen es
// asunto del adaptador.
type CanalDeVideos interface {
	// VideosDelCanal devuelve las últimas subidas de un canal, de la más
	// reciente a la más antigua. `handle` es el de la URL, con arroba.
	VideosDelCanal(ctx context.Context, handle string, max int) ([]domain.VideoDeCanal, error)
}

// VideoService sirve los videos de los canales a la app.
type VideoService interface {
	ListarVideos(ctx context.Context) ([]domain.VideoDeCanal, error)
}
