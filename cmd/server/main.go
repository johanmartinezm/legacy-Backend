package main

import (
	"applegacy/backend/internal/adapter/storage/postgres"
	"applegacy/backend/internal/config"
	"applegacy/backend/internal/core/services"
	handler "applegacy/backend/internal/handler/http"
	"applegacy/backend/internal/infrastructure/credibanco"
	"applegacy/backend/internal/infrastructure/email"
	"applegacy/backend/internal/infrastructure/firebase"
	"applegacy/backend/internal/infrastructure/websocket"
	"applegacy/backend/internal/security"
	"context"
	"fmt"
	"log"
	netHttp "net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {
	// 1. Configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Database Connection
	dbPool, err := pgxpool.Connect(context.Background(), cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()
	fmt.Println("Connected to Database")

	// 3. Services Initialization
	cryptoService, err := security.NewCryptoService(cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to init crypto: %v", err)
	}

	userRepo := postgres.NewUserRepository(dbPool)
	adminRepo := postgres.NewAdminUserRepository(dbPool)
	tokenRepo := postgres.NewPasswordResetRepository(dbPool)
	verifyRepo := postgres.NewEmailVerificationRepository(dbPool)

	emailService, err := email.NewGmailService(
		cfg.Email.GmailCredentialsFile,
		cfg.Email.GmailImpersonateUser,
	)
	if err != nil {
		log.Fatalf("Failed to init email service: %v", err)
	}

	authService := services.NewAuthService(
		userRepo,
		adminRepo,
		tokenRepo,
		verifyRepo,
		emailService,
		cryptoService,
		cfg.Security.JWTSecret,
		cfg.WebApp.ResetPasswordURL,
		cfg.WebApp.VerifyEmailURL,
		cfg.Firebase.GoogleClientID,
	)
	userHandler := handler.NewUserHandler(authService)
	adminHandler := handler.NewAdminHandler(authService)
	likeRepo := postgres.NewLikeRepository(dbPool)
	likeService := services.NewLikeService(likeRepo)
	likeHandler := handler.NewLikeHandler(likeService)

	// Las notificaciones se cablean antes que los eventos y el contenido porque
	// esos handlers las usan para avisar de una novedad: un evento nuevo o un
	// contenido recién publicado mandan un push al topic "all".
	fcmClient, err := firebase.NewFCMClient(cfg.Firebase.CredentialsFile)
	if err != nil {
		log.Fatalf("Failed to init FCM client: %v", err)
	}
	notificationRepo := postgres.NewNotificationRepository(dbPool)
	notificationService := services.NewNotificationService(notificationRepo, fcmClient)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	eventRepo := postgres.NewEventRepository(dbPool)
	eventService := services.NewEventService(eventRepo, cryptoService)
	eventHandler := handler.NewEventHandler(eventService, notificationService)

	// El repositorio de bloqueos lo usan dos servicios: el suyo propio y el de
	// chat, que consulta el bloqueo antes de dejar invitar, escribir o leer.
	blockRepo := postgres.NewBlockRepository(dbPool)
	blockService := services.NewBlockService(blockRepo, cryptoService)
	blockHandler := handler.NewBlockHandler(blockService)

	chatRepo := postgres.NewChatRepository(dbPool)
	chatService := services.NewChatService(chatRepo, userRepo, blockRepo, cryptoService)
	chatHub := websocket.NewHub(chatService, chatRepo)
	go chatHub.Run()
	chatHandler := handler.NewChatHandler(chatService, chatHub)

	bannerRepo := postgres.NewBannerRepository(dbPool)
	bannerService := services.NewBannerService(bannerRepo)
	bannerHandler := handler.NewBannerHandler(bannerService)

	contentCatRepo := postgres.NewContentCategoryRepository(dbPool)
	customContentRepo := postgres.NewCustomContentRepository(dbPool)
	contentService := services.NewContentService(contentCatRepo, customContentRepo)
	contentHandler := handler.NewContentHandler(contentService, notificationService)

	statsRepo := postgres.NewStatsRepository(dbPool)
	statsService := services.NewStatsService(statsRepo, cryptoService)
	statsHandler := handler.NewStatsHandler(statsService)

	synergyRepo := postgres.NewSynergyRepository(dbPool)
	synergyService := services.NewSynergyService(synergyRepo, cryptoService)
	synergyHandler := handler.NewSynergyHandler(synergyService)

	boardService := services.NewBoardService(emailService, cfg.BoardContacts)
	boardHandler := handler.NewBoardHandler(boardService, authService)

	asesoriaService := services.NewAsesoriaService(emailService, cfg.AsesoriaEmail)
	asesoriaHandler := handler.NewAsesoriaHandler(asesoriaService, authService)

	groupRepo := postgres.NewGroupRepository(dbPool)
	groupService := services.NewGroupService(groupRepo)
	groupHandler := handler.NewGroupHandler(groupService)

	credibancoClient := credibanco.NewCredibancoClient(cfg)
	transactionRepo := postgres.NewTransactionRepository(dbPool)
	// eventRepo entra aqui para que un pago aprobado confirme la inscripcion del
	// evento; sin el, la inscripcion se quedaria en pending_payment para siempre.
	paymentService := services.NewPaymentService(transactionRepo, credibancoClient, eventRepo)
	paymentHandler := handler.NewPaymentHandler(paymentService)

	forumRepo := postgres.NewForumRepository(dbPool)
	forumService := services.NewForumService(forumRepo, userRepo)
	forumHandler := handler.NewForumHandler(forumService, nil)

	// Las imagenes de los foros. El handler existia con sus tests desde hacia
	// meses, pero nunca se instancio ni se enruto: adjuntar una imagen a un hilo
	// respondia 404 y la app lo mostraba como un fallo al subir.
	imageHandler := handler.NewImageHandler(cfg.Storage.UploadsDir)

	// 4. Router Setup
	r := chi.NewRouter()

	// CORS: solo los origenes que de verdad usan esta API desde un navegador.
	//
	// Antes era AllowedOrigins: {"*"} junto con AllowCredentials: true, que es
	// una combinacion que los navegadores rechazan de todos modos, asi que
	// ademas de abierta era incorrecta. Cualquier pagina de cualquier dominio
	// podia llamar a la API con el token de quien la visitara.
	//
	// La app movil NO se ve afectada: un cliente nativo no manda cabecera Origin
	// y CORS no interviene. Esto solo gobierna a los navegadores, es decir al
	// panel administrativo y a la app compilada para web.
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  origenPermitido,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/register", userHandler.Register)
	r.Post("/login", userHandler.Login)
	r.Post("/social-login", userHandler.SocialLogin)
	r.Post("/forgot-password", userHandler.ForgotPassword)
	r.Post("/reset-password", userHandler.ResetPassword)
	
	// New verification routes
	r.Post("/verify-email", userHandler.VerifyEmail)
	r.Post("/resend-verification", userHandler.ResendVerificationEmail)

	// Alias bajo /api/ de las rutas de auth que HAProxy no enruta desde la raíz.
	// En producción solo /api/... y una lista fija de paths llegan al backend, y
	// estas tres quedaron fuera de esa lista: sin el alias, nginx responde 405.
	r.Post("/api/auth/social-login", userHandler.SocialLogin)
	r.Post("/api/auth/verify-email", userHandler.VerifyEmail)
	r.Post("/api/auth/resend-verification", userHandler.ResendVerificationEmail)

	// El panel Angular ya desplegado llama a /api/verify-email (auth.service.ts:58).
	// Se mantiene como alias para no tener que redesplegar el frontend.
	r.Post("/api/verify-email", userHandler.VerifyEmail)

	// Servir una imagen de foro es publico a proposito: la app la pinta con
	// Image.network, que no manda la cabecera Authorization. El nombre lo genera
	// el servidor con un UUID, asi que no se puede adivinar, y GetImage recorta
	// el nombre con filepath.Base para que nadie salga del directorio.
	//
	// Se registran las dos formas por lo mismo que las rutas de auth de arriba:
	// en produccion HAProxy solo enruta /api/... al backend, pero los builds de
	// la app ya instalados piden /images/... directamente.
	r.Get("/api/images/{fileName}", imageHandler.GetImage)
	r.Get("/images/{fileName}", imageHandler.GetImage)

	// Webhook de CredibanCo. Va aquí, sin AuthMiddleware, porque quien llama es
	// la pasarela y no tiene un token nuestro. El handler no se cree el
	// contenido: consulta el estado a CredibanCo con nuestras credenciales, así
	// que una notificación inventada no puede aprobar ningún pago.
	//
	// GET y POST: esta familia de pasarelas notifica por GET, pero no conviene
	// depender de ello.
	r.Get("/api/payments/credibanco/callback", paymentHandler.CredibancoCallback)
	r.Post("/api/payments/credibanco/callback", paymentHandler.CredibancoCallback)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(handler.AuthMiddleware([]byte(cfg.Security.JWTSecret)))
		r.Post("/api/posts/{id}/like", likeHandler.ToggleLike)
		r.Post("/api/workshops/{id}/rating", eventHandler.SubmitWorkshopRating)

		// Subir exige sesion aunque ver no la exija: sin esto cualquiera podria
		// llenar el disco del servidor. La app ya manda el Bearer al subir
		// (forum_thread_screen.dart:88).
		r.Post("/api/images/upload", imageHandler.UploadImage)
		r.Post("/images/upload", imageHandler.UploadImage)
	})

	// Public routes (with optional auth)
	r.Group(func(r chi.Router) {
		r.Use(handler.OptionalAuthMiddleware([]byte(cfg.Security.JWTSecret)))
		r.Get("/api/posts/{id}/likes", likeHandler.GetLikeStatus)
		r.Post("/api/posts/{id}/view", likeHandler.RecordView)

		// Event routes
		r.Get("/api/events", eventHandler.ListEvents)
		r.Get("/api/events/{id}", eventHandler.GetEventDetails)
		r.Get("/api/categories", eventHandler.ListCategories)

		// Banner routes (Public)
		r.Get("/api/banners", bannerHandler.ListActive)

		// Custom Content routes (Public)
		r.Get("/api/content/categories", contentHandler.ListCategories)
		r.Get("/api/content/items", contentHandler.ListContent)
		r.Get("/api/content/items/{id}", contentHandler.GetContent)

		// Synergy routes (Public read)
		r.Get("/api/synergies", synergyHandler.ListSynergies)
		r.Get("/api/synergies/{id}", synergyHandler.GetSynergy)

		// Forums (Public read)
		r.Get("/api/forums", forumHandler.ListPublicForums)
		r.Get("/api/forums/{forumID}/posts", forumHandler.ListPosts)
	})

	// Private routes
	r.Group(func(r chi.Router) {
		r.Use(handler.AuthMiddleware([]byte(cfg.Security.JWTSecret)))
		r.Post("/api/events/{id}/register", eventHandler.Register)

		// Encuesta general del evento. Distinta de /api/workshops/{id}/rating,
		// que califica una charla suelta. El servicio exige que el usuario este
		// registrado en el evento y solo admite una respuesta por persona.
		r.Post("/api/events/{id}/survey", eventHandler.SubmitEventSurvey)
		r.Get("/api/events/{id}/survey/me", eventHandler.GetMyEventSurvey)

		// Mi credencial: los eventos en los que el usuario esta inscrito, con su
		// QR. Cuelga de /api/me y no de /api/events porque el patron
		// /api/events/{id} del grupo publico captura cualquier segmento y se
		// tragaba la ruta: devolvia "invalid input syntax for type uuid".
		r.Get("/api/me/registrations", eventHandler.GetMyRegistrations)

		// Agenda Management
		r.Get("/api/events/agenda", eventHandler.GetAgenda)
		r.Post("/api/workshops/{id}/agenda", eventHandler.AddToAgenda)
		r.Delete("/api/workshops/{id}/agenda", eventHandler.RemoveFromAgenda)

		// Payments
		r.Post("/api/payments/intent", paymentHandler.CreatePaymentIntent)
		r.Get("/api/payments/verify", paymentHandler.VerifyPayment)

		// User Profile (Authenticated User)
		r.Get("/api/me", userHandler.Me)
		r.Put("/api/me", userHandler.UpdateMe)
		r.Post("/api/me/change-password", userHandler.ChangePassword)
		// Eliminar la propia cuenta. Lo exigen App Store y Google Play a toda
		// app con registro; sin esta ruta no se puede publicar.
		r.Delete("/api/me", userHandler.DeleteMe)

		// Board contact route
		r.Post("/api/board/contact", boardHandler.Contact)

		// Asesoria request route
		r.Post("/api/asesoria/request", asesoriaHandler.Request)

		// FCM Token Registration
		r.Post("/api/me/fcm-token", notificationHandler.RegisterToken)

		// Synergy Interaction routes
		r.Post("/api/synergies", synergyHandler.CreateSynergy)
		r.Post("/api/synergies/{id}/comments", synergyHandler.CommentSynergy)
		r.Post("/api/synergies/{id}/like", synergyHandler.ToggleLike)

		// Chat routes
		r.Route("/api/chat", func(r chi.Router) {
			r.Get("/ws", chatHandler.HandleWS)
			r.Get("/members", chatHandler.ListMembers)
			r.Get("/connections", chatHandler.ListConnections)
			r.Post("/connect/{receiverID}", chatHandler.SendInvite)
			r.Post("/accept/{connectionID}", chatHandler.AcceptInvite)
			r.Get("/history/{connectionID}", chatHandler.GetHistory)
			r.Post("/message", chatHandler.SendMessage)
		})

		// Bloquear y reportar personas. Requisito de la directriz 1.2 de Apple:
		// una app con chat y foros necesita que se pueda reportar contenido Y
		// bloquear a quien abusa, desde la propia app.
		r.Get("/api/blocks", blockHandler.ListBlocked)
		r.Post("/api/blocks/{userID}", blockHandler.BlockUser)
		r.Delete("/api/blocks/{userID}", blockHandler.UnblockUser)
		r.Post("/api/users/{userID}/report", blockHandler.ReportUser)

		// Forum interaction routes
		r.Post("/api/forums", forumHandler.CreateUserForum)
		r.Post("/api/forums/{forumID}/posts", forumHandler.PublishPost)
		r.Post("/api/forums/posts/{postID}/report", forumHandler.ReportPost)

		// Regular User Management (Admin Only)
		r.Group(func(r chi.Router) {
			r.Use(handler.AdminOnly([]byte(cfg.Security.JWTSecret)))
			r.Get("/api/users", userHandler.List)
			r.Put("/api/users/{id}", userHandler.Update)
			r.Delete("/api/users/{id}", userHandler.Delete)
		})
	})

	// Admin Public Routes
	r.Post("/api/admin/login", adminHandler.AdminLogin)

	// Admin routes (protected, admin only)
	r.Group(func(r chi.Router) {
		r.Use(handler.AdminOnly([]byte(cfg.Security.JWTSecret)))
		r.Post("/api/admin/register", adminHandler.RegisterAdmin)
		r.Get("/api/admin/users", adminHandler.ListAdmins)
		r.Put("/api/admin/users/{id}", adminHandler.UpdateAdmin)
		r.Delete("/api/admin/users/{id}", adminHandler.DeleteAdmin)

		// Admin Event Management
		// Estaban bajo AuthMiddleware pese al comentario "Admin": cualquier usuario
		// con sesion podia crear, editar y borrar eventos, registrar asistencia por
		// QR y leer las calificaciones. Los usa solo el panel, que autentica con un
		// token de rol "admin" (auth_service.go:301).
		r.Post("/api/events", eventHandler.CreateEvent)
		r.Put("/api/events/{id}", eventHandler.UpdateEvent)
		r.Delete("/api/events/{id}", eventHandler.DeleteEvent)
		r.Get("/api/events/{id}/feedback", eventHandler.GetEventFeedback)
		r.Get("/api/events/{id}/registrations", eventHandler.GetEventRegistrants)
		r.Get("/api/events/{id}/survey/summary", eventHandler.GetEventSurveySummary)
		r.Post("/api/events/check-in", eventHandler.CheckIn)

		// Admin Banner Management
		r.Get("/api/admin/banners", bannerHandler.AdminListAll)
		r.Post("/api/admin/banners", bannerHandler.AdminCreate)
		r.Put("/api/admin/banners/{id}", bannerHandler.AdminUpdate)
		r.Delete("/api/admin/banners/{id}", bannerHandler.AdminDelete)

		// Admin Custom Content Management
		r.Get("/api/admin/content/categories", contentHandler.ListCategories)
		r.Post("/api/admin/content/categories", contentHandler.AdminCreateCategory)
		r.Put("/api/admin/content/categories/{id}", contentHandler.AdminUpdateCategory)
		r.Delete("/api/admin/content/categories/{id}", contentHandler.AdminDeleteCategory)

		r.Get("/api/admin/content/items", contentHandler.AdminListContent)
		r.Post("/api/admin/content/items", contentHandler.AdminCreateContent)
		r.Put("/api/admin/content/items/{id}", contentHandler.AdminUpdateContent)
		r.Delete("/api/admin/content/items/{id}", contentHandler.AdminDeleteContent)

		// Admin Stats
		r.Get("/api/admin/stats/dashboard", statsHandler.GetDashboardStats)

		// Admin Notifications
		r.Post("/api/admin/notifications/send", notificationHandler.Send)
		// Suscribe al topico "all" los dispositivos ya registrados. Los nuevos se
		// suscriben solos al registrar su token; esto arregla los anteriores.
		r.Post("/api/admin/notifications/subscribe-all", notificationHandler.SubscribeAll)
		r.Get("/api/admin/notifications/history", notificationHandler.GetHistory)

		// Admin Custom Groups
		r.Get("/api/admin/groups", groupHandler.ListGroups)
		r.Post("/api/admin/groups", groupHandler.CreateGroup)
		r.Delete("/api/admin/groups/{id}", groupHandler.DeleteGroup)
		r.Get("/api/admin/groups/{id}/members", groupHandler.GetMembers)
		r.Post("/api/admin/groups/{id}/members", groupHandler.ReplaceMembers)

		// Admin Forums
		r.Get("/api/admin/forums", forumHandler.AdminListForums)

		r.Post("/api/admin/forums", forumHandler.AdminCreateForum)
		r.Put("/api/admin/forums/{forumID}", forumHandler.AdminUpdateForum)
		// AdminDeleteForum llevaba escrito desde el modulo de foros y nunca se
		// registro, asi que el boton "Eliminar" del panel —que ya llama a esta
		// misma URL desde forum-admin.service.ts— recibia un 405. El patron
		// existia para PUT pero no para DELETE.
		r.Delete("/api/admin/forums/{forumID}", forumHandler.AdminDeleteForum)
		r.Patch("/api/admin/forums/{forumID}/lock", forumHandler.AdminLockForum)
		r.Patch("/api/admin/forums/{forumID}/unlock", forumHandler.AdminUnlockForum)
		// /flagged va ANTES que /{forumID}/posts: chi resuelve primero el
		// segmento literal, pero dejarlos juntos y en este orden lo hace evidente.
		// Esta linea estaba repetida dos veces; la copia se retiro.
		r.Get("/api/admin/forums/flagged", forumHandler.AdminListFlaggedPosts)
		r.Get("/api/admin/forums/{forumID}/posts", forumHandler.AdminGetForumTree)
		r.Delete("/api/admin/forums/posts/{postID}", forumHandler.AdminDeletePost)

		// Bandeja de reportes de personas, la que atiende las denuncias del
		// chat. Los reportes de publicaciones de foro tienen la suya aparte,
		// en /api/admin/forums/flagged.
		r.Get("/api/admin/user-reports", blockHandler.ListReports)
		r.Patch("/api/admin/user-reports/{reportID}", blockHandler.ResolveReport)
	})

	// Health check
	r.Get("/health", func(w netHttp.ResponseWriter, r *netHttp.Request) {
		w.Write([]byte("OK"))
	})

	// 5. Start Server
	fmt.Printf("Starting server on port %s...\n", cfg.Server.Port)
	if err := netHttp.ListenAndServe(":"+cfg.Server.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// origenesDeConfianza son los sitios que sirven la interfaz web en produccion.
//
// Si algun dia el panel o la app web se publican en otro dominio, hay que
// anadirlo aqui o dejaran de funcionar con un error de CORS en el navegador.
var origenesDeConfianza = []string{
	"https://legacy.intelyclick.com",
}

// origenPermitido decide si un navegador puede llamar a la API desde ese origen.
//
// Acepta los dominios de produccion y cualquier localhost, sin importar el
// puerto: `ng serve` usa el 4200 y `flutter run -d chrome` levanta uno
// aleatorio en cada arranque, asi que fijar puertos romperia el desarrollo.
func origenPermitido(r *netHttp.Request, origin string) bool {
	for _, permitido := range origenesDeConfianza {
		if strings.EqualFold(origin, permitido) {
			return true
		}
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
