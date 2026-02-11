package main

import (
	"fmt"

	"github.com/Ejexi/Agent/internal/core/config"
)

func main() {
	cfg, err := config.Load("configs/dev/config.yaml")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Environment: %s\n", cfg.App.Env)
	fmt.Printf("Log Level: %s\n", cfg.App.LogLevel)
	//fmt.Printf("Database Host: %s\n", cfg.Database.Host)
	fmt.Printf("LLM Model: %s\n", cfg.LLM.Model)
	fmt.Printf("LLM Temperature: %.2f\n", cfg.LLM.Temperature)
}
