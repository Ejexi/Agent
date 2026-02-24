package main

import (
	"flag"
	// "log"
	// "os"

	// "github.com/SecDuckOps/agent/internal/adapters/agent"
)

func main() {
	
	// serverURL := os.Getenv("DUCKOPS_SERVER")
	// if serverURL == "" {
	// 	serverURL = "https://api.duckops.com"
	// }

	// token := os.Getenv("DUCKOPS_TOKEN")

	// a, err := agent.New(agent.Config{
	// 	ServerURL: serverURL,
	// 	Token:     token,
	// })

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// a.Run()


	var workerType string
	var mode string
	flag.StringVar(&workerType, "worker", "", "Start agent in worker mode (e.g., sast, dast, secrets)")
	flag.StringVar(&mode, "mode", "standalone", "Operation mode: standalone or cloud")
	flag.Parse()

	// 1. Initialize the application components
	k, provider := InitApp()

	// 2. Start appropriate mode
	if workerType != "" {
		RunWorkerMode(k, workerType)
	} else {
		RunInteractiveMode(k, provider, mode)
	}
}
