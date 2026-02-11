// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

// DiscoverySnapshot represents a discoverysnapshot resource
type DiscoverySnapshot struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       DiscoverySnapshotSpec   `json:"spec" validate:"required"`
	Status     DiscoverySnapshotStatus `json:"status,omitempty"`
}

// DiscoverySnapshotSpec defines the desired state of DiscoverySnapshot
type DiscoverySnapshotSpec struct {
	Description string `json:"description,omitempty" validate:"max=200"`
	// Add your spec fields here
}

// DiscoverySnapshotStatus defines the observed state of DiscoverySnapshot
type DiscoverySnapshotStatus struct {
	Phase      string `json:"phase,omitempty"`
	Message    string `json:"message,omitempty"`
	Ready      bool   `json:"ready"`
		// Add your status fields here
}

// Validate implements custom validation logic for DiscoverySnapshot
func (r *DiscoverySnapshot) Validate(ctx context.Context) error {
	// Add custom validation logic here
	// Example:
	// if r.Spec.Description == "forbidden" {
	//     return errors.New("description 'forbidden' is not allowed")
	// }

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

// IsHub marks this as the hub/storage version
func (r *DiscoverySnapshot) IsHub() {}
