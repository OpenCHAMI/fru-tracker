package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/example/fru-tracker/internal/storage"
)

func newExportCommand() *cobra.Command {
	var (
		format  string
		output  string
		kinds   []string
		perType bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export resources to files",
		Long: `Export all resources from storage to human-readable JSON or YAML files.

This is useful for:
  - Creating backups of your resources
  - Migrating data between environments
  - Version controlling resource definitions
  - Inspecting resource state

Examples:
  # Export all resources to YAML
  fru_tracker export --format yaml --output ./backup

  # Export specific resource types
  fru_tracker export --kinds Device --output ./device-backup
  fru_tracker export --kinds DiscoverySnapshot --output ./discoverysnapshot-backup
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd.Context(), format, output, kinds, perType)
		},
	}

	cmd.Flags().StringVar(&format, "format", "yaml", "Output format: json, yaml")
	cmd.Flags().StringVar(&output, "output", "./backup", "Output directory for exported files")
	cmd.Flags().StringSliceVar(&kinds, "kinds", nil, "Filter by resource kinds")
	cmd.Flags().BoolVar(&perType, "per-type", true, "Organize output into subdirectories by resource type")

	return cmd
}

func runExport(ctx context.Context, format, output string, kinds []string, perType bool) error {
	fmt.Printf("🚀 Exporting resources...\n")
	fmt.Printf("   Format: %s\n", format)
	fmt.Printf("   Output: %s\n", output)

	// Validate format
	if format != "json" && format != "yaml" {
		return fmt.Errorf("unsupported format: %s (use 'json' or 'yaml')", format)
	}

	// Create output directory
	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine resource kinds to export
	var resourceKinds []string
	if len(kinds) > 0 {
		resourceKinds = kinds
	} else {
		// Export all known resource types
		resourceKinds = []string{"Device", "DiscoverySnapshot"}
	}

	totalExported := 0

	// Export each resource kind
	for _, kind := range resourceKinds {
		count, err := exportResourceKind(ctx, kind, output, format, perType)
		if err != nil {
			return fmt.Errorf("failed to export %s: %w", kind, err)
		}
		totalExported += count
	}

	fmt.Printf("✅ Export complete. Exported %d resources.\n", totalExported)
	return nil
}

func exportResourceKind(ctx context.Context, kind, output, format string, perType bool) (int, error) {
	// Query all resources of this kind using storage query builder
	var resources []interface{}
	var err error

	switch kind {
	case "Device":
		items, e := storage.Querydevices(ctx).All(ctx)
		if e != nil {
			return 0, fmt.Errorf("failed to query devices: %w", e)
		}
		for _, item := range items {
			resources = append(resources, item)
		}
	case "DiscoverySnapshot":
		items, e := storage.Querydiscoverysnapshots(ctx).All(ctx)
		if e != nil {
			return 0, fmt.Errorf("failed to query discoverysnapshots: %w", e)
		}
		for _, item := range items {
			resources = append(resources, item)
		}
	default:
		return 0, fmt.Errorf("unknown resource kind: %s", kind)
	}

	if len(resources) == 0 {
		return 0, nil
	}

	// Create resource type directory
	var resourceDir string
	if perType {
		resourceDir = filepath.Join(output, strings.ToLower(kind)+"s")
		if err := os.MkdirAll(resourceDir, 0755); err != nil {
			return 0, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Write each resource
	count := 0
	for i, resource := range resources {
		// Extract name from metadata (requires type assertion based on resource)
		name := fmt.Sprintf("%s-%d", strings.ToLower(kind), i)

		// Serialize resource
		var data []byte
		if format == "json" {
			data, err = json.MarshalIndent(resource, "", "  ")
		} else {
			data, err = yaml.Marshal(resource)
		}
		if err != nil {
			return count, fmt.Errorf("failed to marshal resource: %w", err)
		}

		var filename string
		if perType {
			filename = filepath.Join(resourceDir, fmt.Sprintf("%s.%s", name, format))
		} else {
			filename = filepath.Join(output, fmt.Sprintf("%s-%s.%s", strings.ToLower(kind), name, format))
		}

		// Write file
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return count, fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Printf("  ✓ %s\n", filepath.Base(filename))
		count++
	}

	return count, nil
}
