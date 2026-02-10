package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
)

func main() {
	rootPath, pathError := resolveRootPath()

	if pathError != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", pathError)
		os.Exit(1)
	}

	finalModel, runError := tea.NewProgram(
		initialModel(rootPath),
		tea.WithAltScreen(),
	).Run()

	if runError != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", runError)
		os.Exit(1)
	}

	if model, isApplicationModel := finalModel.(applicationModel); isApplicationModel {
		if model.scanError != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", model.scanError)
			os.Exit(1)
		}

		if model.deletedCount > 0 {
			fmt.Printf("deleted %d directories, freed %s\n", model.deletedCount, formatBytes(model.freedBytes))
		}
	}
}

func resolveRootPath() (string, error) {
	if len(os.Args) > 1 {
		return filepath.Abs(os.Args[1])
	}

	return os.Getwd()
}
