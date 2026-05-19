// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/example/fru-tracker/internal/storage"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/openchami/fabrica/pkg/resource"
)

func (r *DiscoverySnapshotReconciler) reconcileDiscoverySnapshot(ctx context.Context, snapshot *v1.DiscoverySnapshot) error {
	if snapshot.Status.Phase == "Completed" {
		r.Logger.Infof("Reconciling %s: Already completed, skipping.", snapshot.GetName())
		return nil
	}

	r.Logger.Infof("Reconciling %s: Starting reconciliation", snapshot.GetName())
	snapshot.Status.Phase = "Processing"
	snapshot.Status.Message = "Reconciler has started processing the snapshot."
	snapshot.Status.Ready = false

	var payloadSpecs []v1.DeviceSpec
	if err := json.Unmarshal(snapshot.Spec.RawData, &payloadSpecs); err != nil {
		snapshot.Status.Phase = "Error"
		snapshot.Status.Message = fmt.Sprintf("Failed to parse rawData: %v", err)
		snapshot.Status.Ready = false
		if updateErr := r.UpdateStatus(ctx, snapshot); updateErr != nil {
			return fmt.Errorf("failed to persist error status: %w", updateErr)
		}
		return fmt.Errorf("failed to parse rawData: %w", err)
	}

	lookupKeys := collectLookupKeys(payloadSpecs)
	existingDevices, err := storage.LoadDevicesByIdentifiers(ctx, lookupKeys)
	if err != nil {
		snapshot.Status.Phase = "Error"
		snapshot.Status.Message = fmt.Sprintf("Failed to prefetch devices: %v", err)
		snapshot.Status.Ready = false
		if updateErr := r.UpdateStatus(ctx, snapshot); updateErr != nil {
			return fmt.Errorf("failed to persist error status: %w", updateErr)
		}
		return fmt.Errorf("failed to prefetch devices: %w", err)
	}

	bySerial := make(map[string]*v1.Device)
	byURI := make(map[string]*v1.Device)
	byUID := make(map[string]*v1.Device)
	for _, device := range existingDevices {
		indexDevice(device, bySerial, byURI, byUID)
	}

	processedDevices := make([]*v1.Device, 0, len(payloadSpecs))
	createdCount := 0
	updatedCount := 0

	for _, spec := range payloadSpecs {
		device := deviceFromSpec(spec)
		if device == nil {
			r.Logger.Warnf("Reconciling %s: Skipping invalid device spec", snapshot.GetName())
			continue
		}

		if existing := matchDevice(spec, bySerial, byURI); existing != nil {
			merged := mergeDevice(existing, spec)
			processedDevices = append(processedDevices, merged)
			indexDevice(merged, bySerial, byURI, byUID)
			updatedCount++
			continue
		}

		processedDevices = append(processedDevices, device)
		indexDevice(device, bySerial, byURI, byUID)
		createdCount++
	}

	if err := storage.SaveDevicesBulk(ctx, processedDevices); err != nil {
		snapshot.Status.Phase = "Error"
		snapshot.Status.Message = fmt.Sprintf("Failed to persist device changes: %v", err)
		snapshot.Status.Ready = false
		if updateErr := r.UpdateStatus(ctx, snapshot); updateErr != nil {
			return fmt.Errorf("failed to persist error status: %w", updateErr)
		}
		return fmt.Errorf("failed to persist device changes: %w", err)
	}

	r.Logger.Infof("Reconciling %s (Pass 2): Linking parent relationships...", snapshot.GetName())
	linksUpdated := 0
	cycleSkips := 0
	linkUpdates := make([]*v1.Device, 0, len(processedDevices))
	for _, dev := range processedDevices {
		parentKey := dev.Spec.ParentSerialNumber
		parentURI := propertyString(dev.Spec.Properties, "redfish_parent_uri")
		if parentKey == "" {
			parentKey = parentURI
		}
		if parentKey == "" {
			continue
		}

		parentDevice := bySerial[parentKey]
		if parentDevice == nil {
			parentDevice = byURI[parentKey]
		}
		if parentDevice == nil && parentURI != "" && parentURI != parentKey {
			parentDevice = bySerial[parentURI]
			if parentDevice == nil {
				parentDevice = byURI[parentURI]
			}
		}
		if parentDevice == nil {
			r.Logger.Errorf("Reconciling %s (Pass 2): Parent device %s not found for child %s", snapshot.GetName(), parentKey, deviceLabel(dev))
			continue
		}
		if dev.GetUID() == parentDevice.GetUID() {
			r.Logger.Warnf("Reconciling %s (Pass 2): Skipping self-parenting link for %s", snapshot.GetName(), deviceLabel(dev))
			continue
		}
		if dev.Spec.ParentID == parentDevice.GetUID() {
			continue
		}
		if wouldCreateCycle(dev.GetUID(), parentDevice.GetUID(), byUID) {
			cycleSkips++
			r.Logger.Warnf("Reconciling %s (Pass 2): Skipping cyclic parent link %s -> %s", snapshot.GetName(), deviceLabel(dev), deviceLabel(parentDevice))
			continue
		}

		r.Logger.Infof("Reconciling %s (Pass 2): Linking %s (UID: %s) to parent %s (UID: %s)", snapshot.GetName(), deviceLabel(dev), dev.GetUID(), deviceLabel(parentDevice), parentDevice.GetUID())
		dev.Spec.ParentID = parentDevice.GetUID()
		dev.Metadata.UpdatedAt = time.Now()
		linkUpdates = append(linkUpdates, dev)
		linksUpdated++
		indexDevice(dev, bySerial, byURI, byUID)
	}

	if err := storage.SaveDevicesBulk(ctx, linkUpdates); err != nil {
		snapshot.Status.Phase = "Error"
		snapshot.Status.Message = fmt.Sprintf("Failed to persist parent links: %v", err)
		snapshot.Status.Ready = false
		if updateErr := r.UpdateStatus(ctx, snapshot); updateErr != nil {
			return fmt.Errorf("failed to persist error status: %w", updateErr)
		}
		return fmt.Errorf("failed to persist parent links: %w", err)
	}

	snapshot.Status.Phase = "Completed"
	snapshot.Status.Message = fmt.Sprintf("Snapshot processed. %d devices created, %d updated, %d parent links established.", createdCount, updatedCount, linksUpdated)
	if cycleSkips > 0 {
		snapshot.Status.Message = fmt.Sprintf("%s %d cyclic links skipped.", snapshot.Status.Message, cycleSkips)
	}
	snapshot.Status.Ready = true

	r.Logger.Infof("Reconciling %s: Successfully reconciled", snapshot.GetName())
	return nil
}

func collectLookupKeys(specs []v1.DeviceSpec) []string {
	keys := make([]string, 0, len(specs)*4)
	for _, spec := range specs {
		keys = append(keys, spec.SerialNumber)
		keys = append(keys, propertyString(spec.Properties, "redfish_uri"))
		keys = append(keys, spec.ParentSerialNumber)
		keys = append(keys, propertyString(spec.Properties, "redfish_parent_uri"))
	}
	return keys
}

func deviceFromSpec(spec v1.DeviceSpec) *v1.Device {
	uid, err := resource.GenerateUIDForResource("Device")
	if err != nil {
		return nil
	}

	device := &v1.Device{
		APIVersion: "example.fabrica.dev/v1",
		Kind:       "Device",
		Metadata: fabrica.Metadata{
			Name: chooseDeviceName(spec),
			UID:  uid,
		},
		Spec: spec,
	}
	device.Metadata.Initialize(device.Metadata.Name, device.Metadata.UID)
	return device
}

func mergeDevice(existing *v1.Device, spec v1.DeviceSpec) *v1.Device {
	merged := *existing
	merged.Spec = existing.Spec
	if spec.DeviceType != "" {
		merged.Spec.DeviceType = spec.DeviceType
	}
	if spec.Manufacturer != "" {
		merged.Spec.Manufacturer = spec.Manufacturer
	}
	if spec.PartNumber != "" {
		merged.Spec.PartNumber = spec.PartNumber
	}
	if spec.SerialNumber != "" {
		merged.Spec.SerialNumber = spec.SerialNumber
	}
	if spec.ParentSerialNumber != "" {
		merged.Spec.ParentSerialNumber = spec.ParentSerialNumber
	}
	if len(spec.Properties) > 0 {
		merged.Spec.Properties = mergeProperties(existing.Spec.Properties, spec.Properties)
	}
	merged.Metadata.Name = chooseDeviceName(merged.Spec)
	merged.Metadata.UpdatedAt = time.Now()
	return &merged
}

func chooseDeviceName(spec v1.DeviceSpec) string {
	if spec.SerialNumber != "" {
		return spec.SerialNumber
	}
	if uri := propertyString(spec.Properties, "redfish_uri"); uri != "" {
		return uri
	}
	if spec.DeviceType != "" {
		return spec.DeviceType
	}
	return "device"
}

func propertyString(properties map[string]json.RawMessage, key string) string {
	if len(properties) == 0 {
		return ""
	}
	raw, ok := properties[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return strings.Trim(string(raw), `"`)
}

func mergeProperties(existing map[string]json.RawMessage, incoming map[string]json.RawMessage) map[string]json.RawMessage {
	merged := make(map[string]json.RawMessage, len(existing)+len(incoming))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range incoming {
		merged[key] = value
	}
	return merged
}

func indexDevice(device *v1.Device, bySerial, byURI, byUID map[string]*v1.Device) {
	if device == nil {
		return
	}
	if device.GetUID() != "" {
		byUID[device.GetUID()] = device
	}
	if device.Spec.SerialNumber != "" {
		bySerial[device.Spec.SerialNumber] = device
	}
	if uri := propertyString(device.Spec.Properties, "redfish_uri"); uri != "" {
		byURI[uri] = device
	}
}

func matchDevice(spec v1.DeviceSpec, bySerial, byURI map[string]*v1.Device) *v1.Device {
	if spec.SerialNumber != "" {
		if device := bySerial[spec.SerialNumber]; device != nil {
			return device
		}
	}
	if uri := propertyString(spec.Properties, "redfish_uri"); uri != "" {
		if device := byURI[uri]; device != nil {
			return device
		}
	}
	return nil
}

func deviceLabel(device *v1.Device) string {
	if device == nil {
		return "<nil>"
	}
	if device.Spec.SerialNumber != "" {
		return device.Spec.SerialNumber
	}
	if uri := propertyString(device.Spec.Properties, "redfish_uri"); uri != "" {
		return uri
	}
	return device.GetName()
}

func wouldCreateCycle(childUID, parentUID string, ancestry map[string]*v1.Device) bool {
	seen := map[string]struct{}{childUID: {}}
	current := parentUID
	for current != "" {
		if _, ok := seen[current]; ok {
			return true
		}
		seen[current] = struct{}{}
		parent := ancestry[current]
		if parent == nil {
			return false
		}
		current = parent.Spec.ParentID
	}
	return false
}
