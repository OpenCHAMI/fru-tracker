// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"github.com/openchami/fabrica/pkg/resource"
)

// Device represents a Device resource
type Device struct {
	resource.Resource
	Spec   DeviceSpec   `json:"spec" validate:"required"`
	Status DeviceStatus `json:"status,omitempty"`
}

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	DeviceType   string `json:"deviceType" validate:"required"`
	Manufacturer string `json:"manufacturer,omitempty"`
	PartNumber   string `json:"partNumber,omitempty"`
	SerialNumber string `json:"serialNumber" validate:"required"`

	// ParentID holds the UID of the parent device.
	// This will be populated by the reconciler.
	ParentID string `json:"parentID,omitempty"`

	// ParentSerialNumber holds the serial number of the parent.
	// The collector will set this, and the reconciler will resolve it to a ParentID.
	ParentSerialNumber string `json:"parentSerialNumber,omitempty"`

	// Properties is an arbitrary key-value map for non-standard attributes.
	Properties map[string]json.RawMessage `json:"properties,omitempty"`
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	Phase             string   `json:"phase,omitempty"`
	Message           string   `json:"message,omitempty"`
	Ready             bool     `json:"ready"`
	
	// ChildrenDeviceIds is a read-only list of devices contained within this one.
	ChildrenDeviceIds []string `json:"childrenDeviceIds,omitempty"`
}

// Validate implements custom validation logic for Device
func (r *Device) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource
func (r *Device) GetKind() string {
	return "Device"
}

// GetName returns the name of the resource
func (r *Device) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *Device) GetUID() string {
	return r.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("Device", "dev")
}
