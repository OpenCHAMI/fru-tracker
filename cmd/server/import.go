// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/openchami/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/openchami/fru-tracker/internal/storage"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newImportCommand() *cobra.Command {
	var (
		input        string
		mode         string
		dryRun       bool
		skipExisting bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import resources from files",
		Long: `Import resources from JSON or YAML files into storage.

This is useful for:
  - Restoring from backups
  - Migrating data between environments
  - Bulk loading resource definitions
  - Testing with known resource state

Import modes:
  - upsert: Create new resources or update existing (default)
  - replace: Delete all resources first, then import
  - skip: Skip resources that already exist

Examples:
  # Import from backup directory
  fru_tracker import --input ./backup

  # Dry run to preview changes
  fru_tracker import --input ./backup --dry-run

  # Replace all resources
  fru_tracker import --input ./backup --mode replace
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd.Context(), input, mode, dryRun, skipExisting)
		},
	}

	cmd.Flags().StringVar(&input, "input", "./backup", "Input directory containing resource files")
	cmd.Flags().StringVar(&mode, "mode", "upsert", "Import mode: upsert, replace, skip")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip resources that already exist (same as --mode skip)")

	return cmd
}

func runImport(ctx context.Context, input, mode string, dryRun, skipExisting bool) error {
	fmt.Printf("🚀 Importing resources...\n")
	fmt.Printf("   Input: %s\n", input)
	fmt.Printf("   Mode: %s\n", mode)
	if dryRun {
		fmt.Printf("   ⚠️  DRY RUN - No changes will be applied\n")
	}

	// Validate mode
	if skipExisting {
		mode = "skip"
	}
	if mode != "upsert" && mode != "replace" && mode != "skip" {
		return fmt.Errorf("unsupported mode: %s (use 'upsert', 'replace', or 'skip')", mode)
	}

	// Check input directory exists
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("input directory does not exist: %w", err)
	}
	files, err := preflightImport(input)
	if err != nil {
		return err
	}

	// Handle replace mode only after every input file passes preflight.
	if mode == "replace" && !dryRun {
		fmt.Printf("⚠️  Replace mode - deleting existing resources...\n")
		if err := deleteAllResources(ctx); err != nil {
			return fmt.Errorf("failed to delete existing resources: %w", err)
		}
	}

	totalImported := 0
	totalSkipped := 0
	for _, path := range files {
		imported, skipped, err := importFile(ctx, path, mode, dryRun)
		if err != nil {
			return fmt.Errorf("failed to import %s: %w", filepath.Base(path), err)
		}
		totalImported += imported
		totalSkipped += skipped
	}

	if dryRun {
		fmt.Printf("✅ Dry run complete. Would import %d resources (%d skipped).\n", totalImported, totalSkipped)
	} else {
		fmt.Printf("✅ Import complete. Imported %d resources (%d skipped).\n", totalImported, totalSkipped)
	}

	return nil
}

type importFileData struct {
	data []byte
	ext  string
	kind string
}

func readImportFile(path string) (importFileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return importFileData{}, fmt.Errorf("failed to read file: %w", err)
	}
	ext := filepath.Ext(path)
	var genericResource struct {
		APIVersion string `json:"apiVersion" yaml:"apiVersion"`
		Kind       string `json:"kind" yaml:"kind"`
	}

	if ext == ".json" {
		if err := json.Unmarshal(data, &genericResource); err != nil {
			return importFileData{}, fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &genericResource); err != nil {
			return importFileData{}, fmt.Errorf("failed to parse YAML: %w", err)
		}
	}
	return importFileData{data: data, ext: ext, kind: genericResource.Kind}, nil
}

func unmarshalImportResource(file importFileData, resource interface{}) error {
	if file.ext == ".json" {
		return json.Unmarshal(file.data, resource)
	}
	return yaml.Unmarshal(file.data, resource)
}

func validateImportFile(path string) error {
	file, err := readImportFile(path)
	if err != nil {
		return err
	}
	switch file.kind {
	case "Device":
		var res *v1.Device
		if err := unmarshalImportResource(file, &res); err != nil {
			return fmt.Errorf("failed to unmarshal Device: %w", err)
		}
		return nil
	case "DiscoverySnapshot":
		var res *v1.DiscoverySnapshot
		if err := unmarshalImportResource(file, &res); err != nil {
			return fmt.Errorf("failed to unmarshal DiscoverySnapshot: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown resource kind: %s", file.kind)
	}
}

func preflightImport(input string) ([]string, error) {
	var files []string
	err := filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		if err := validateImportFile(path); err != nil {
			return fmt.Errorf("invalid import file %s: %w", filepath.Base(path), err)
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("import preflight failed: %w", err)
	}
	return files, nil
}

func importFile(ctx context.Context, path string, mode string, dryRun bool) (imported, skipped int, err error) {
	file, err := readImportFile(path)
	if err != nil {
		return 0, 0, err
	}

	// Import based on kind
	switch file.kind {
	case "Device":
		var res *v1.Device
		if err := unmarshalImportResource(file, &res); err != nil {
			return 0, 0, fmt.Errorf("failed to unmarshal Device: %w", err)
		}

		// Check if resource exists
		existing, err := storage.GetDeviceByUID(ctx, res.Metadata.UID)
		if err == nil && existing != nil {
			// Resource exists
			if mode == "skip" {
				fmt.Printf("  ⊘ %s (exists)\n", filepath.Base(path))
				return 0, 1, nil
			}
			fmt.Printf("  ⟳ %s (updating)\n", filepath.Base(path))
		} else {
			fmt.Printf("  ✓ %s (creating)\n", filepath.Base(path))
		}

		if !dryRun {
			if err := storage.SaveDevice(ctx, res); err != nil {
				return 0, 0, fmt.Errorf("failed to save Device: %w", err)
			}
		}
		return 1, 0, nil
	case "DiscoverySnapshot":
		var res *v1.DiscoverySnapshot
		if err := unmarshalImportResource(file, &res); err != nil {
			return 0, 0, fmt.Errorf("failed to unmarshal DiscoverySnapshot: %w", err)
		}

		// Check if resource exists
		existing, err := storage.GetDiscoverySnapshotByUID(ctx, res.Metadata.UID)
		if err == nil && existing != nil {
			// Resource exists
			if mode == "skip" {
				fmt.Printf("  ⊘ %s (exists)\n", filepath.Base(path))
				return 0, 1, nil
			}
			fmt.Printf("  ⟳ %s (updating)\n", filepath.Base(path))
		} else {
			fmt.Printf("  ✓ %s (creating)\n", filepath.Base(path))
		}

		if !dryRun {
			if err := storage.SaveDiscoverySnapshot(ctx, res); err != nil {
				return 0, 0, fmt.Errorf("failed to save DiscoverySnapshot: %w", err)
			}
		}
		return 1, 0, nil
	default:
		return 0, 0, fmt.Errorf("unknown resource kind: %s", file.kind)
	}
}

func deleteAllResources(ctx context.Context) error {
	// Delete all devices
	deviceItems, err := storage.Querydevices(ctx).All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query devices: %w", err)
	}
	for _, item := range deviceItems {
		if err := storage.DeleteDevice(ctx, item.UID); err != nil {
			return fmt.Errorf("failed to delete Device: %w", err)
		}
	}
	// Delete all discoverysnapshots
	discoverysnapshotItems, err := storage.Querydiscoverysnapshots(ctx).All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query discoverysnapshots: %w", err)
	}
	for _, item := range discoverysnapshotItems {
		if err := storage.DeleteDiscoverySnapshot(ctx, item.UID); err != nil {
			return fmt.Errorf("failed to delete DiscoverySnapshot: %w", err)
		}
	}
	return nil
}
