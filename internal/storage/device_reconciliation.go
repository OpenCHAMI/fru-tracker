// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/example/fru-tracker/internal/storage/ent"
	entresource "github.com/example/fru-tracker/internal/storage/ent/resource"
	"github.com/openchami/fabrica/pkg/resource"
)

// LoadDevicesByIdentifiers loads Device resources whose names match any of the provided identifiers.
// The reconciler uses this to prefetch only the records that matter for a snapshot.
func LoadDevicesByIdentifiers(ctx context.Context, identifiers []string) ([]*v1.Device, error) {
	if err := ensureBackendReady(); err != nil {
		return nil, err
	}

	lookup := uniqueStrings(identifiers)
	if len(lookup) == 0 {
		return nil, nil
	}

	entResources, err := entClient.Resource.Query().
		Where(
			entresource.KindEQ("Device"),
			entresource.NameIn(lookup...),
		).
		WithLabels().
		WithAnnotations().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load scoped Device resources: %w", err)
	}

	devices := make([]*v1.Device, 0, len(entResources))
	for _, entResource := range entResources {
		fabricaResource, err := FromEntResource(ctx, entResource)
		if err != nil {
			continue
		}
		devices = append(devices, fabricaResource.(*v1.Device))
	}

	return devices, nil
}

// SaveDevicesBulk upserts a set of Device resources in a single transaction.
func SaveDevicesBulk(ctx context.Context, devices []*v1.Device) error {
	if err := ensureBackendReady(); err != nil {
		return err
	}

	return WithTx(ctx, func(tx *ent.Tx) error {
		for _, device := range devices {
			if device == nil {
				continue
			}

			if device.Metadata.UID == "" {
				uid, err := resource.GenerateUIDForResource("Device")
				if err != nil {
					return fmt.Errorf("failed to generate UID for Device: %w", err)
				}
				device.Metadata.UID = uid
			}
			if device.APIVersion == "" {
				device.APIVersion = "example.fabrica.dev/v1"
			}
			if device.Kind == "" {
				device.Kind = "Device"
			}
			if device.Metadata.Name == "" {
				device.Metadata.Name = device.Metadata.UID
			}

			spec, err := json.Marshal(device.Spec)
			if err != nil {
				return fmt.Errorf("failed to marshal Device spec: %w", err)
			}
			status, err := json.Marshal(device.Status)
			if err != nil {
				return fmt.Errorf("failed to marshal Device status: %w", err)
			}

			existing, err := tx.Resource.Query().
				Where(entresource.UIDEQ(device.Metadata.UID), entresource.KindEQ("Device")).
				Only(ctx)
			if err != nil && !ent.IsNotFound(err) {
				return fmt.Errorf("failed to look up Device %s: %w", device.Metadata.UID, err)
			}

			now := time.Now()
			if ent.IsNotFound(err) {
				builder := tx.Resource.Create().
					SetUID(device.Metadata.UID).
					SetName(device.Metadata.Name).
					SetAPIVersion(device.APIVersion).
					SetKind("Device").
					SetResourceType("Device").
					SetSpec(spec).
					SetStatus(status).
					SetCreatedAt(now).
					SetUpdatedAt(now)

				if !device.Metadata.CreatedAt.IsZero() {
					builder = builder.SetCreatedAt(device.Metadata.CreatedAt)
				}
				if !device.Metadata.UpdatedAt.IsZero() {
					builder = builder.SetUpdatedAt(device.Metadata.UpdatedAt)
				}

				if _, err := builder.Save(ctx); err != nil {
					return fmt.Errorf("failed to create Device %s: %w", device.Metadata.UID, err)
				}
				continue
			}

			builder := tx.Resource.UpdateOneID(existing.ID).
				SetName(device.Metadata.Name).
				SetAPIVersion(device.APIVersion).
				SetKind("Device").
				SetResourceType("Device").
				SetSpec(spec).
				SetStatus(status).
				SetUpdatedAt(now)

			if _, err := builder.Save(ctx); err != nil {
				return fmt.Errorf("failed to update Device %s: %w", device.Metadata.UID, err)
			}
		}
		return nil
	})
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}