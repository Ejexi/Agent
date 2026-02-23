package main

import "flag"

func main() {
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
