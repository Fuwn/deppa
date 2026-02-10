package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var knownDependencyDirectories = map[string]bool{
	"node_modules": true,
	"target":       true,
	".next":        true,
	".nuxt":        true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".gradle":      true,
	"Pods":         true,
	"zig-cache":    true,
	"zig-out":      true,
	"_build":       true,
	".dart_tool":   true,
}
var ignoredDirectories = map[string]bool{
	".git": true,
	".hg":  true,
	".svn": true,
}

type DependencyDirectory struct {
	Path          string
	RelativePath  string
	SizeBytes     int64
	DirectoryType string
}

func scanForDependencies(rootPath string) ([]DependencyDirectory, error) {
	var foundDirectories []DependencyDirectory

	walkError := filepath.WalkDir(rootPath, func(currentPath string, entry fs.DirEntry, accessError error) error {
		if accessError != nil {
			return filepath.SkipDir
		}

		if !entry.IsDir() {
			return nil
		}

		if ignoredDirectories[entry.Name()] {
			return filepath.SkipDir
		}

		if knownDependencyDirectories[entry.Name()] {
			relativePath, _ := filepath.Rel(rootPath, currentPath)
			foundDirectories = append(foundDirectories, DependencyDirectory{
				Path:          currentPath,
				RelativePath:  relativePath,
				DirectoryType: entry.Name(),
			})

			return filepath.SkipDir
		}

		return nil
	})

	if walkError != nil {
		return nil, walkError
	}

	var waitGroup sync.WaitGroup

	semaphore := make(chan struct{}, runtime.NumCPU())

	for directoryIndex := range foundDirectories {
		waitGroup.Add(1)

		go func(targetIndex int) {
			defer waitGroup.Done()

			semaphore <- struct{}{}

			defer func() { <-semaphore }()

			foundDirectories[targetIndex].SizeBytes = calculateDirectorySize(foundDirectories[targetIndex].Path)
		}(directoryIndex)
	}

	waitGroup.Wait()

	return foundDirectories, nil
}

func calculateDirectorySize(directoryPath string) int64 {
	var totalSize int64

	_ = filepath.WalkDir(directoryPath, func(_ string, entry fs.DirEntry, walkError error) error {
		if walkError != nil || entry.IsDir() {
			return nil
		}

		fileInformation, informationError := entry.Info()

		if informationError == nil {
			totalSize += fileInformation.Size()
		}

		return nil
	})

	return totalSize
}

func deleteDependencies(dependencies []DependencyDirectory, selected map[int]bool) (int64, int) {
	var freedBytes int64
	var deletedCount int

	for dependencyIndex, dependency := range dependencies {
		if !selected[dependencyIndex] {
			continue
		}

		if removeError := os.RemoveAll(dependency.Path); removeError == nil {
			freedBytes += dependency.SizeBytes

			deletedCount++
		}
	}

	return freedBytes, deletedCount
}
