// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/openchami/fabrica/pkg/resource"
)

// DiscoverySnapshot represents a DiscoverySnapshot resource
type DiscoverySnapshot struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   fabrica.Metadata        `json:"metadata"`
	Spec       DiscoverySnapshotSpec   `json:"spec" validate:"required"`
	Status     DiscoverySnapshotStatus `json:"status,omitempty"`
}

// DiscoverySnapshotSpec defines the desired state of DiscoverySnapshot
type DiscoverySnapshotSpec struct {
	// RawData holds the complete, raw JSON payload from a discovery tool (e.g., the collector).
	// The reconciler will parse this.
	RawData json.RawMessage `json:"rawData" validate:"required"`
}

// DiscoverySnapshotStatus defines the observed state of DiscoverySnapshot
type DiscoverySnapshotStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Ready   bool   `json:"ready"`
}

// Validate implements custom validation logic for DiscoverySnapshot
func (r *DiscoverySnapshot) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource
func (r *DiscoverySnapshot) GetKind() string {
	return "DiscoverySnapshot"
}

// GetName returns the name of the resource
func (r *DiscoverySnapshot) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *DiscoverySnapshot) GetUID() string {
	return r.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("DiscoverySnapshot", "dis")
}
