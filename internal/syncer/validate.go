package syncer

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateContent validates all content in a directory using the same parsers
// as the sync pipeline. Returns a list of human-readable errors.
func ValidateContent(dir string) []string {
	var errors []string

	// Check phoebus.yaml exists
	_, err := parsePhoebus(dir)
	if err != nil {
		errors = append(errors, fmt.Sprintf("phoebus.yaml: %s", err))
		return errors // can't continue without root config
	}

	// Find and validate modules
	moduleDirs, err := findOrderedDirs(dir)
	if err != nil {
		errors = append(errors, fmt.Sprintf("module directories: %s", err))
		return errors
	}

	if len(moduleDirs) == 0 {
		errors = append(errors, "no module directories found (expected numbered directories like 01-intro/)")
	}

	for _, moduleDir := range moduleDirs {
		moduleName := filepath.Base(moduleDir)

		// Validate module index.md
		_, err := parseModuleIndex(moduleDir)
		if err != nil {
			errors = append(errors, fmt.Sprintf("module %s/index.md: %s", moduleName, err))
			continue
		}

		// Validate steps
		stepFiles, err := findOrderedSteps(moduleDir)
		if err != nil {
			errors = append(errors, fmt.Sprintf("module %s steps: %s", moduleName, err))
			continue
		}

		if len(stepFiles) == 0 {
			errors = append(errors, fmt.Sprintf("module %s: no step files found", moduleName))
		}

		for _, stepPath := range stepFiles {
			stepName := filepath.Base(stepPath)
			if stepName == "instructions.md" {
				stepName = filepath.Base(filepath.Dir(stepPath)) + "/instructions.md"
			}

			stepMeta, _, _, err := parseStep(stepPath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("step %s/%s: %s", moduleName, stepName, err))
				continue
			}

			// Validate step type is known
			validTypes := map[string]bool{
				"lesson":            true,
				"quiz":              true,
				"terminal-exercise": true,
				"code-exercise":     true,
			}
			if !validTypes[stepMeta.Type] {
				errors = append(errors, fmt.Sprintf("step %s/%s: unknown type %q (expected lesson, quiz, terminal-exercise, or code-exercise)", moduleName, stepName, stepMeta.Type))
			}

			// Validate code exercise has codebase dir
			if stepMeta.Type == "code-exercise" {
				codebaseDir := filepath.Join(filepath.Dir(stepPath), "codebase")
				if fi, err := os.Stat(codebaseDir); err != nil || !fi.IsDir() {
					errors = append(errors, fmt.Sprintf("step %s/%s: code-exercise requires a codebase/ directory", moduleName, stepName))
				}
			}
		}
	}

	return errors
}
