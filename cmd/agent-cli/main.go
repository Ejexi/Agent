package main

func main() {
	// 1. Initialize the application components
	k, provider := InitApp()

	// 2. Start the interactive loop
	RunInteractiveMode(k, provider)
}
