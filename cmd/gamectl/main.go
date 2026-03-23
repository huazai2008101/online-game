package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	version = "1.0.0"
)

var commands = map[string]func([]string) error{
	"start":   cmdStart,
	"stop":    cmdStop,
	"status":  cmdStatus,
	"logs":    cmdLogs,
	"restart": cmdRestart,
	"build":   cmdBuild,
	"test":    cmdTest,
	"clean":   cmdClean,
	"health":  cmdHealth,
	"version": cmdVersion,
	"help":    cmdHelp,
}

func main() {
	if len(os.Args) < 2 {
		cmdHelp([]string{})
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	handler, ok := commands[cmd]
	if !ok {
		fmt.Printf("Unknown command: %s\n", cmd)
		cmdHelp([]string{})
		os.Exit(1)
	}

	if err := handler(args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdStart(args []string) error {
	service := "all"
	if len(args) > 0 {
		service = args[0]
	}

	fmt.Printf("Starting %s...\n", service)

	if service == "all" {
		return execCommand("./deploy/start.sh", args...)
	}

	// Start specific service
	return execCommand("go", "run", fmt.Sprintf("./cmd/%s/main.go", service))
}

func cmdStop(args []string) error {
	fmt.Println("Stopping services...")
	return execCommand("./deploy/stop.sh", args...)
}

func cmdRestart(args []string) error {
	fmt.Println("Restarting services...")
	if err := cmdStop(args); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	return cmdStart(args)
}

func cmdStatus(args []string) error {
	fmt.Println("Service Status:")
	fmt.Println(strings.Repeat("-", 50))

	// Check Docker services
	dockerRunning := isDockerRunning()
	if dockerRunning {
		fmt.Println("Docker: Running")
		printServiceStatus()
	} else {
		fmt.Println("Docker: Not running")
	}

	// Check API Gateway
	if isGatewayRunning() {
		fmt.Println("API Gateway: Running (http://localhost:8080)")
	} else {
		fmt.Println("API Gateway: Stopped")
	}

	return nil
}

func cmdLogs(args []string) error {
	service := "all"
	if len(args) > 0 {
		service = args[0]
	}

	return execCommand("./deploy/logs.sh", service)
}

func cmdBuild(args []string) error {
	fmt.Println("Building services...")
	fmt.Println(strings.Repeat("-", 40))

	services := []string{
		"user-service",
		"game-service",
		"payment-service",
		"player-service",
		"activity-service",
		"guild-service",
		"item-service",
		"notification-service",
		"organization-service",
		"permission-service",
		"id-service",
		"file-service",
		"api-gateway",
	}

	for _, svc := range services {
		fmt.Printf("Building %s...", svc)
		cmd := exec.Command("go", "build", "-o", fmt.Sprintf("bin/%s", svc), fmt.Sprintf("./cmd/%s/main.go", svc))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf(" FAILED\n")
			return err
		}
		fmt.Printf(" OK\n")
	}

	fmt.Println("\nBuild complete!")
	return nil
}

func cmdTest(args []string) error {
	verbose := false
	bench := false

	for _, arg := range args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
		}
		if arg == "-b" || arg == "--bench" {
			bench = true
		}
	}

	if bench {
		fmt.Println("Running benchmarks...")
		return execCommand("go", "test", "./tests/...", "-bench=.", "-benchtime=1s")
	}

	fmt.Println("Running tests...")
	testArgs := []string{"test", "./..."}
	if verbose {
		testArgs = append(testArgs, "-v")
	}
	return execCommand("go", testArgs...)
}

func cmdClean(args []string) error {
	fmt.Println("Cleaning...")

	dirs := []string{"bin", "tmp", "data"}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	fmt.Println("Clean complete!")
	return nil
}

func cmdHealth(args []string) error {
	service := "http://localhost:8080"
	if len(args) > 0 {
		service = args[0]
	}

	fmt.Printf("Checking health: %s\n", service)

	resp, err := http.Get(service + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		fmt.Println(string(body))
		return nil
	}

	// Pretty print health status
	status := health["status"]
	fmt.Printf("Status: %v\n", status)

	if checks, ok := health["checks"].(map[string]interface{}); ok {
		fmt.Println("\nChecks:")
		for name, check := range checks {
			if c, ok := check.(map[string]interface{}); ok {
				fmt.Printf("  %s: %v\n", name, c["status"])
			}
		}
	}

	return nil
}

func cmdVersion(args []string) error {
	fmt.Printf("Game Platform CLI v%s\n", version)
	fmt.Println("A high-performance online game platform")
	return nil
}

func cmdHelp(args []string) error {
	fmt.Println("Game Platform CLI - Management Tool")
	fmt.Println("")
	fmt.Println("Usage: gamectl <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start [service]    Start all services or specific service")
	fmt.Println("  stop               Stop all services")
	fmt.Println("  restart            Restart all services")
	fmt.Println("  status             Show service status")
	fmt.Println("  logs [service]     View service logs")
	fmt.Println("  build              Build all services")
	fmt.Println("  test [-v|-b]       Run tests (-v: verbose, -b: benchmark)")
	fmt.Println("  clean              Clean build artifacts")
	fmt.Println("  health [url]       Check service health")
	fmt.Println("  version            Show version info")
	fmt.Println("  help               Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  gamectl start              # Start all services")
	fmt.Println("  gamectl start user-service # Start user service")
	fmt.Println("  gamectl logs game-service  # View game service logs")
	fmt.Println("  gamectl health              # Check API gateway health")
	fmt.Println("")
	fmt.Println("Services:")
	services := []string{
		"user-service", "game-service", "payment-service",
		"player-service", "activity-service", "guild-service",
		"item-service", "notification-service", "organization-service",
		"permission-service", "id-service", "file-service",
		"api-gateway",
	}
	for _, svc := range services {
		fmt.Printf("  - %s\n", svc)
	}
	return nil
}

// Helper functions

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isDockerRunning() bool {
	cmd := exec.Command("docker", "ps")
	return cmd.Run() == nil
}

func isGatewayRunning() bool {
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

func printServiceStatus() {
	// Try to get service status from docker-compose
	cmd := exec.Command("docker-compose", "-f", "deploy/docker-compose.yml", "-p", "game-platform", "ps")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
