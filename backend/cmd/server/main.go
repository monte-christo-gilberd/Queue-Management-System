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

	txRunner := database.NewSqlxTxRunner(db)
	ticketRepo := repository.NewTicketRepository(db)
	ticketService := service.NewTicketService(ticketRepo, txRunner)
	ticketHandler := handler.NewTicketHandler(ticketService)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, cfg.JWTSecret)
	userHandler := handler.NewUserHandler(userService)

	queueRepo := repository.NewQueueRepository(db)
	subscriptionChecker := repository.NewSubscriptionChecker(db)
	queueService := service.NewQueueService(queueRepo, subscriptionChecker, txRunner)
	queueHandler := handler.NewQueueHandler(queueService)

	engine := router.SetupRouter(
		router.NewAppRouter(
			ticketHandler,
			userHandler,
			queueHandler,
			middleware.RequireAuth(
				cfg.JWTSecret,
			),
		),
	)

	port := fmt.Sprintf(":%s", cfg.Port)
	engine.Run(port)
}
