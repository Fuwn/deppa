package main

import (
	"fmt"
	"sort"
	"strings"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type applicationPhase int

const (
	phaseScanning applicationPhase = iota
	phaseBrowsing
	phaseDeleting
	phaseDone
)

type scanCompleteMessage struct {
	dependencies []DependencyDirectory
	scanError    error
}

type deletionCompleteMessage struct {
	freedBytes   int64
	deletedCount int
}

type applicationModel struct {
	phase          applicationPhase
	rootPath       string
	dependencies   []DependencyDirectory
	selected       map[int]bool
	cursorPosition int
	loadingSpinner spinner.Model
	terminalWidth  int
	terminalHeight int
	freedBytes     int64
	deletedCount   int
	scanError      error
}

var (
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	sizeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	typeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
)

func initialModel(rootPath string) applicationModel {
	loadingSpinner := spinner.New()
	loadingSpinner.Spinner = spinner.Dot
	loadingSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return applicationModel{
		phase:          phaseScanning,
		rootPath:       rootPath,
		selected:       make(map[int]bool),
		loadingSpinner: loadingSpinner,
	}
}

func (model applicationModel) Init() tea.Cmd {
	return tea.Batch(model.loadingSpinner.Tick, startScan(model.rootPath))
}

func startScan(rootPath string) tea.Cmd {
	return func() tea.Msg {
		dependencies, scanError := scanForDependencies(rootPath)

		return scanCompleteMessage{dependencies: dependencies, scanError: scanError}
	}
}

func startDeletion(dependencies []DependencyDirectory, selected map[int]bool) tea.Cmd {
	return func() tea.Msg {
		freedBytes, deletedCount := deleteDependencies(dependencies, selected)

		return deletionCompleteMessage{freedBytes: freedBytes, deletedCount: deletedCount}
	}
}

func (model applicationModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMessage := message.(type) {
	case tea.KeyMsg:
		return model.handleKeyPress(typedMessage)
	case tea.WindowSizeMsg:
		model.terminalWidth = typedMessage.Width
		model.terminalHeight = typedMessage.Height

		return model, nil
	case spinner.TickMsg:
		var spinnerCommand tea.Cmd

		model.loadingSpinner, spinnerCommand = model.loadingSpinner.Update(message)

		return model, spinnerCommand
	case scanCompleteMessage:
		if typedMessage.scanError != nil {
			model.scanError = typedMessage.scanError
			model.phase = phaseDone

			return model, tea.Quit
		}

		sort.Slice(typedMessage.dependencies, func(firstIndex, secondIndex int) bool {
			return typedMessage.dependencies[firstIndex].SizeBytes > typedMessage.dependencies[secondIndex].SizeBytes
		})

		model.dependencies = typedMessage.dependencies
		model.phase = phaseBrowsing

		if len(model.dependencies) == 0 {
			model.phase = phaseDone

			return model, tea.Quit
		}

		return model, nil
	case deletionCompleteMessage:
		model.freedBytes = typedMessage.freedBytes
		model.deletedCount = typedMessage.deletedCount
		model.phase = phaseDone

		return model, tea.Quit
	}

	return model, nil
}

func (model applicationModel) handleKeyPress(keyMessage tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMessage.String() == "ctrl+c" {
		return model, tea.Quit
	}

	if model.phase != phaseBrowsing {
		return model, nil
	}

	switch keyMessage.String() {
	case "q":
		return model, tea.Quit
	case "up", "k":
		if model.cursorPosition > 0 {
			model.cursorPosition--
		}
	case "down", "j":
		if model.cursorPosition < len(model.dependencies)-1 {
			model.cursorPosition++
		}
	case " ":
		if model.selected[model.cursorPosition] {
			delete(model.selected, model.cursorPosition)
		} else {
			model.selected[model.cursorPosition] = true
		}
	case "a":
		allCurrentlySelected := true

		for dependencyIndex := range model.dependencies {
			if !model.selected[dependencyIndex] {
				allCurrentlySelected = false

				break
			}
		}

		if allCurrentlySelected {
			model.selected = make(map[int]bool)
		} else {
			for dependencyIndex := range model.dependencies {
				model.selected[dependencyIndex] = true
			}
		}
	case "enter":
		if len(model.selected) == 0 {
			return model, nil
		}

		model.phase = phaseDeleting

		return model, startDeletion(model.dependencies, model.selected)
	}

	return model, nil
}

func (model applicationModel) selectedSize() int64 {
	var totalSize int64

	for dependencyIndex, isSelected := range model.selected {
		if isSelected {
			totalSize += model.dependencies[dependencyIndex].SizeBytes
		}
	}

	return totalSize
}

func (model applicationModel) View() string {
	switch model.phase {
	case phaseScanning:
		return fmt.Sprintf("\n  %s scanning %s\n", model.loadingSpinner.View(), model.rootPath)
	case phaseBrowsing:
		return model.viewBrowsing()
	case phaseDeleting:
		return fmt.Sprintf("\n  %s deleting selected directories\n", model.loadingSpinner.View())
	case phaseDone:
		return model.viewDone()
	}

	return ""
}

func (model applicationModel) viewBrowsing() string {
	var builder strings.Builder

	builder.WriteString(accentStyle.Render("\n  deppa"))
	builder.WriteString(dimStyle.Render(fmt.Sprintf(" — %d directories found\n\n", len(model.dependencies))))

	visibleHeight := model.terminalHeight - 6

	if visibleHeight < 1 {
		visibleHeight = 10
	}

	startIndex := 0

	if model.cursorPosition >= startIndex+visibleHeight {
		startIndex = model.cursorPosition - visibleHeight + 1
	}

	endIndex := min(startIndex+visibleHeight, len(model.dependencies))

	for displayIndex := startIndex; displayIndex < endIndex; displayIndex++ {
		dependency := model.dependencies[displayIndex]
		cursor := "  "

		if displayIndex == model.cursorPosition {
			cursor = accentStyle.Bold(true).Render("> ")
		}

		checkbox := dimStyle.Render("[ ]")

		if model.selected[displayIndex] {
			checkbox = accentStyle.Render("[x]")
		}

		formattedSize := sizeStyle.Render(fmt.Sprintf("%8s", formatBytes(dependency.SizeBytes)))
		formattedType := typeStyle.Render(fmt.Sprintf("%-14s", dependency.DirectoryType))

		builder.WriteString(fmt.Sprintf(
			"%s%s %s %s %s\n",
			cursor, checkbox, formattedSize, formattedType, dimStyle.Render(dependency.RelativePath),
		))
	}

	builder.WriteString(dimStyle.Render(fmt.Sprintf(
		"\n  space toggle • a all • enter delete %s • q quit",
		accentStyle.Render(formatBytes(model.selectedSize())),
	)))

	return builder.String()
}

func (model applicationModel) viewDone() string {
	if model.scanError != nil {
		return fmt.Sprintf("\n  error: %s\n", model.scanError)
	}

	if len(model.dependencies) == 0 {
		return "\n  no dependency directories found\n"
	}

	return fmt.Sprintf(
		"\n  deleted %d directories, freed %s\n",
		model.deletedCount, formatBytes(model.freedBytes),
	)
}

func formatBytes(bytes int64) string {
	const (
		kilobyte = 1024
		megabyte = 1024 * kilobyte
		gigabyte = 1024 * megabyte
	)

	switch {
	case bytes >= gigabyte:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gigabyte))
	case bytes >= megabyte:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(megabyte))
	case bytes >= kilobyte:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kilobyte))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
