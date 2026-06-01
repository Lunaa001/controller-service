package main

import (
	"log"
	"os"

	"service-controller-notebookum/internal/config"
	"service-controller-notebookum/internal/web"
)

func main() {
	cfg := config.Load()
	router := web.NewRouter(cfg)

	addr := ":" + cfg.Port
	if err := router.Run(addr); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
