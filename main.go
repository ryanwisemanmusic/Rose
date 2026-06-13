package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ryanwi/rose/config"
	"github.com/ryanwi/rose/memory"
	"github.com/ryanwi/rose/sandbox"
	"github.com/ryanwi/rose/tui"
)

func main() {
	cfg := config.Default()
	if err := cfg.Load(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	store, err := memory.NewStore(cfg.HistoryPath())
	if err != nil {
		log.Fatalf("Failed to open history store: %v", err)
	}
	defer store.Close()

	executor, err := sandbox.NewExecutor(cfg.SandboxTimeout)
	if err != nil {
		log.Fatalf("Failed to create sandbox executor: %v", err)
	}
	defer executor.Cleanup()

	p := tea.NewProgram(
		tui.InitialModel(cfg, store, executor),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
