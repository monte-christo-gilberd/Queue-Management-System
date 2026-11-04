package main

import (
	"fmt"
	"log"
	"qms/internal/config"
	"qms/internal/database"
	"qms/internal/handler"
	"qms/internal/mailclient"
	"qms/internal/middleware"
	"qms/internal/qrclient"
	"qms/internal/repository"
	"qms/internal/router"
	"qms/internal/service"
	"qms/internal/sse"
	noHTMLValidator "qms/internal/validator"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Connect(
		database.Config{DSN: cfg.DatabaseURL},
	)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	hub := sse.NewHub()
	go hub.Run()

	txRunner := database.
		NewSqlxTxRunner(db)
	userRepo := repository.
		NewUserRepository(db)
	userService := service.
		NewUserService(userRepo, cfg.JWTSecret).
		WithAvatars(repository.NewAvatarRepository(db))
	userHandler := handler.
		NewUserHandler(userService)

	queueRepo := repository.
		NewQueueRepository(db)
	subscriptionChecker := repository.
		NewSubscriptionChecker(db)
	ticketRepo := repository.
		NewTicketRepository(db)
	queueService := service.
		NewQueueService(
			queueRepo,
			subscriptionChecker,
			txRunner,
			hub,
		).
		WithTicketHolderChecker(ticketRepo)

	qrService := service.
		NewQRService(
			queueRepo,
			qrclient.New(
				cfg.QRServiceURL,
				cfg.QRServiceToken,
				cfg.QRHTTPTimeout,
			),
			cfg.PublicAppURL,
			hub,
		).
		WithDynamicTTL(120 * time.Second)

	queueHandler := handler.
		NewQueueHandlerWithQR(
			queueService,
			qrService,
		)

	ticketService := service.
		NewTicketService(ticketRepo, txRunner, hub).
		WithQueueGuard(queueService).
		WithAccountFinder(userRepo)
	ticketHandler := handler.
		NewTicketHandlerWithQR(
			ticketService,
			qrService,
		)

	sseHandler := handler.
		NewSSEHandler(hub, queueService)

	// Reset password: pencabutan sesi butuh pembacaan password_changed_at, jadi middleware memakai repo ini.
	resetRepo := repository.NewPasswordResetRepository(db)

	appRouter := router.NewAppRouter(
		ticketHandler,
		userHandler,
		queueHandler,
		middleware.RequireAuthWithRevocation(
			cfg.JWTSecret,
			resetRepo,
		),
		sseHandler,
		cfg.AllowedOrigins,
	)
	noteService := service.
		NewNoteService(repository.NewNoteRepository(db)).
		WithQueueGuard(queueService)

	appRouter.WithNotes(handler.NewNoteHandler(noteService))
	counterService := service.
		NewCounterService(repository.NewCounterRepository(db), txRunner, hub).
		WithQueueGuard(queueService).
		WithTicketReleaser(ticketService)
	appRouter.WithCounters(handler.NewCounterHandler(counterService))
	appRouter.WithHistory(handler.NewHistoryHandler(
		service.NewHistoryService(repository.NewHistoryRepository(db)),
	))

	if cfg.MailServiceURL != "" {
		appRouter.WithPasswordReset(handler.NewPasswordHandler(
			service.NewPasswordResetService(
				resetRepo,
				mailclient.New(cfg.MailServiceURL, cfg.MailServiceToken, 0),
				cfg.PublicAppURL,
			),
		))
	} else {
		log.Println("MAIL_SERVICE_URL kosong: lupa/reset password tidak dipasang")
	}

	if cfg.VAPIDPublicKey != "" {
		appRouter.WithPush(handler.NewPushHandler(
			repository.NewPushRepository(db),
			cfg.VAPIDPublicKey,
		))
	}
	engine := router.SetupRouter(appRouter)

	if v, ok := binding.
		Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation(
			"no_html_tags",
			noHTMLValidator.NoHTMLTags,
		)
	}

	// Tanpa daftar, header X-Forwarded-For dari klien tidak dipercaya (IP klien = alamat koneksi).
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}

	port := fmt.Sprintf(":%s", cfg.Port)
	engine.Run(port)
}
