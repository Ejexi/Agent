package main

import "flag"

func main() {
	var workerType string
	flag.StringVar(&workerType, "worker", "", "Start agent in worker mode (e.g., sast, dast, secrets)")
	flag.Parse()

	// 1. Initialize the application components
	k, provider := InitApp()

	// 2. Start appropriate mode
	if workerType != "" {
		RunWorkerMode(k, workerType)
	} else {
		RunInteractiveMode(k, provider)
	}
}
