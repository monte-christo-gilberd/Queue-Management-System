package main

import (
	"fmt"
	"log"
	"qms/internal/config"
	"qms/internal/database"
	"qms/internal/handler"
	"qms/internal/middleware"
	"qms/internal/repository"
	"qms/internal/router"
	"qms/internal/service"
	"qms/internal/sse"
	noHTMLValidator "qms/internal/validator"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
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
		NewUserService(userRepo, cfg.JWTSecret)
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
	queueHandler := handler.
		NewQueueHandler(queueService)

	ticketService := service.
		NewTicketService(ticketRepo, txRunner, hub).
		WithQueueGuard(queueService)
	ticketHandler := handler.
		NewTicketHandler(ticketService)

	sseHandler := handler.
		NewSSEHandler(hub, queueService)

	engine := router.
		SetupRouter(
			router.NewAppRouter(
				ticketHandler,
				userHandler,
				queueHandler,
				middleware.RequireAuth(
					cfg.JWTSecret,
				),
				sseHandler,
				cfg.AllowedOrigins,
			),
		)

	if v, ok := binding.
		Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation(
			"no_html_tags",
			noHTMLValidator.NoHTMLTags,
		)
	}

	port := fmt.Sprintf(":%s", cfg.Port)
	engine.Run(port)
}
