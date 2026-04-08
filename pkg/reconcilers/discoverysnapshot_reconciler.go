package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/openchami/fabrica/pkg/resource"
)

// extractStringProperty safely extracts a string value from the json.RawMessage properties map
func extractStringProperty(properties map[string]json.RawMessage, key string) string {
	if properties == nil {
		return ""
	}
	raw, exists := properties[key]
	if !exists {
		return ""
	}
	var val string
	if err := json.Unmarshal(raw, &val); err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

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
		return nil
	}

	deviceMapBySerial, deviceMapByURI, err := r.buildDeviceMaps(ctx)
	if err != nil {
		return fmt.Errorf("failed to build device maps: %w", err)
	}

	r.Logger.Infof("Reconciling %s: Loaded %d devices by Serial, %d by URI", snapshot.GetName(), len(deviceMapBySerial), len(deviceMapByURI))
	snapshotDeviceMap := make(map[string]*v1.Device)
	processedCount := 0

	for _, spec := range payloadSpecs {
		serial := spec.SerialNumber
		uri := extractStringProperty(spec.Properties, "redfish_uri")

		if serial == "" && uri == "" {
			r.Logger.Errorf("Reconciling %s: Skipping device, missing both serialNumber and redfish_uri", snapshot.GetName())
			continue
		}

		// Determine identity key for tracking within this snapshot loop
		identityKey := serial
		if identityKey == "" {
			identityKey = uri
		}

		var existingDevice *v1.Device
		var found bool

		// Check serial first, then fallback to URI
		if serial != "" {
			existingDevice, found = deviceMapBySerial[serial]
		}
		if !found && uri != "" {
			existingDevice, found = deviceMapByURI[uri]
		}

		if !found {
			r.Logger.Infof("Reconciling %s (Pass 1): Creating new device: %s", snapshot.GetName(), identityKey)
			newDevice, err := r.createNewDevice(ctx, spec, serial, uri)
			if err != nil {
				r.Logger.Errorf("Reconciling %s (Pass 1): Failed to create device %s: %v", snapshot.GetName(), identityKey, err)
				continue
			}
			snapshotDeviceMap[identityKey] = newDevice
			
			if serial != "" {
				deviceMapBySerial[serial] = newDevice
			}
			if uri != "" {
				deviceMapByURI[uri] = newDevice
			}

		} else {
			r.Logger.Infof("Reconciling %s (Pass 1): Updating existing device: %s (UID: %s)", snapshot.GetName(), identityKey, existingDevice.GetUID())

			spec.ParentID = existingDevice.Spec.ParentID
			existingDevice.Spec = spec
			existingDevice.Metadata.UpdatedAt = time.Now()

			if err := r.Client.Update(ctx, existingDevice); err != nil {
				r.Logger.Errorf("Reconciling %s (Pass 1): Failed to update device %s: %v", snapshot.GetName(), identityKey, err)
				continue
			}
			snapshotDeviceMap[identityKey] = existingDevice
			
			if serial != "" {
				deviceMapBySerial[serial] = existingDevice
			}
			if uri != "" {
				deviceMapByURI[uri] = existingDevice
			}
		}
		processedCount++
	}

	r.Logger.Infof("Reconciling %s (Pass 2): Linking parent relationships...", snapshot.GetName())
	linksUpdated := 0
	for identityKey, dev := range snapshotDeviceMap {
		parentSerial := dev.Spec.ParentSerialNumber
		parentURI := extractStringProperty(dev.Spec.Properties, "redfish_parent_uri")

		if parentSerial == "" && parentURI == "" {
			continue
		}

		var parentDevice *v1.Device
		var found bool

		if parentSerial != "" {
			parentDevice, found = deviceMapBySerial[parentSerial]
		}
		if !found && parentURI != "" {
			parentDevice, found = deviceMapByURI[parentURI]
		}

		if !found {
			r.Logger.Errorf("Reconciling %s (Pass 2): Parent device not found for child %s", snapshot.GetName(), identityKey)
			continue
		}
		if dev.Spec.ParentID == parentDevice.GetUID() {
			continue
		}
		r.Logger.Infof("Reconciling %s (Pass 2): Linking %s (UID: %s) to parent %s (UID: %s)",
			snapshot.GetName(), dev.GetName(), dev.GetUID(), parentDevice.GetName(), parentDevice.GetUID())

		dev.Spec.ParentID = parentDevice.GetUID()
		dev.Metadata.UpdatedAt = time.Now()

		if err := r.Client.Update(ctx, dev); err != nil {
			r.Logger.Errorf("Reconciling %s (Pass 2): Failed to update parent link for %s: %v", snapshot.GetName(), dev.GetName(), err)
		} else {
			linksUpdated++
		}
	}

	snapshot.Status.Phase = "Completed"
	snapshot.Status.Message = fmt.Sprintf("Snapshot processed. %d devices created/updated. %d parent links updated.", processedCount, linksUpdated)
	snapshot.Status.Ready = true

	r.Logger.Infof("Reconciling %s: Successfully reconciled", snapshot.GetName())
	return nil
}

func (r *DiscoverySnapshotReconciler) createNewDevice(ctx context.Context, spec v1.DeviceSpec, serialNumber string, uri string) (*v1.Device, error) {
	uid, err := resource.GenerateUIDForResource("Device")
	if err != nil {
		return nil, fmt.Errorf("failed to generate UID for device: %w", err)
	}

	name := serialNumber
	if name == "" {
		name = uri
	}

	newDevice := &v1.Device{
		APIVersion: "example.fabrica.dev/v1",
		Kind:       "Device",
		Metadata: fabrica.Metadata{
			Name: name,
			UID:  uid,
		},
		Spec: spec,
	}
	newDevice.Metadata.Initialize(newDevice.Metadata.Name, newDevice.Metadata.UID)

	if err := r.Client.Create(ctx, newDevice); err != nil {
		return nil, fmt.Errorf("failed to create device %s: %w", name, err)
	}
	return newDevice, nil
}

func (r *DiscoverySnapshotReconciler) buildDeviceMaps(ctx context.Context) (map[string]*v1.Device, map[string]*v1.Device, error) {
	resourceList, err := r.Client.List(ctx, "Device")
	if err != nil {
		return nil, nil, err
	}
	deviceMapBySerial := make(map[string]*v1.Device)
	deviceMapByURI := make(map[string]*v1.Device)

	for _, item := range resourceList {
		dev, ok := item.(*v1.Device)
		if !ok {
			r.Logger.Errorf("Reconciling: Found non-device item in storage, skipping.")
			continue
		}
		if dev.Spec.SerialNumber != "" {
			deviceMapBySerial[dev.Spec.SerialNumber] = dev
		}
		
		uri := extractStringProperty(dev.Spec.Properties, "redfish_uri")
		if uri != "" {
			deviceMapByURI[uri] = dev
		}
	}
	return deviceMapBySerial, deviceMapByURI, nil
}
