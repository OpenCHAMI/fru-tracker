package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/fabrica"
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
		return nil
	}

	deviceMapBySerial, err := r.buildDeviceMapBySerial(ctx)
	if err != nil {
		return fmt.Errorf("failed to build device map by Serial: %w", err)
	}

	r.Logger.Infof("Reconciling %s: Loaded %d devices by Serial", snapshot.GetName(), len(deviceMapBySerial))
	snapshotDeviceMap := make(map[string]*v1.Device)
	processedCount := 0

	for _, spec := range payloadSpecs {
		serial := spec.SerialNumber
		if serial == "" {
			r.Logger.Errorf("Reconciling %s: Skipping device, missing serialNumber", snapshot.GetName())
			continue
		}

		existingDevice, found := deviceMapBySerial[serial]
		if !found {
			r.Logger.Infof("Reconciling %s (Pass 1): Creating new device: %s", snapshot.GetName(), serial)
			newDevice, err := r.createNewDevice(ctx, spec, serial)
			if err != nil {
				r.Logger.Errorf("Reconciling %s (Pass 1): Failed to create device %s: %v", snapshot.GetName(), serial, err)
				continue
			}
			snapshotDeviceMap[serial] = newDevice
			deviceMapBySerial[serial] = newDevice

		} else {
			r.Logger.Infof("Reconciling %s (Pass 1): Updating existing device: %s (UID: %s)", snapshot.GetName(), serial, existingDevice.GetUID())

			spec.ParentID = existingDevice.Spec.ParentID
			existingDevice.Spec = spec
			existingDevice.Metadata.UpdatedAt = time.Now()

			if err := r.Client.Update(ctx, existingDevice); err != nil {
				r.Logger.Errorf("Reconciling %s (Pass 1): Failed to update device %s: %v", snapshot.GetName(), serial, err)
				continue
			}
			snapshotDeviceMap[serial] = existingDevice
		}
		processedCount++
	}

	r.Logger.Infof("Reconciling %s (Pass 2): Linking parent relationships...", snapshot.GetName())
	linksUpdated := 0
	for _, dev := range snapshotDeviceMap {
		parentSerial := dev.Spec.ParentSerialNumber
		if parentSerial == "" {
			continue
		}
		parentDevice, found := deviceMapBySerial[parentSerial]
		if !found {
			r.Logger.Errorf("Reconciling %s (Pass 2): Parent device with serial %s not found for child %s", snapshot.GetName(), parentSerial, dev.Spec.SerialNumber)
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

func (r *DiscoverySnapshotReconciler) createNewDevice(ctx context.Context, spec v1.DeviceSpec, serialNumber string) (*v1.Device, error) {
	uid, err := resource.GenerateUIDForResource("Device")
	if err != nil {
		return nil, fmt.Errorf("failed to generate UID for device: %w", err)
	}

	newDevice := &v1.Device{
		APIVersion: "example.fabrica.dev/v1",
		Kind:       "Device",
		Metadata: fabrica.Metadata{
			Name: serialNumber,
			UID:  uid,
		},
		Spec: spec,
	}
	newDevice.Metadata.Initialize(newDevice.Metadata.Name, newDevice.Metadata.UID)

	if err := r.Client.Create(ctx, newDevice); err != nil {
		return nil, fmt.Errorf("failed to create device %s: %w", serialNumber, err)
	}
	return newDevice, nil
}

func (r *DiscoverySnapshotReconciler) buildDeviceMapBySerial(ctx context.Context) (map[string]*v1.Device, error) {
	resourceList, err := r.Client.List(ctx, "Device")
	if err != nil {
		return nil, err
	}
	deviceMap := make(map[string]*v1.Device)
	for _, item := range resourceList {
		dev, ok := item.(*v1.Device)
		if !ok {
			r.Logger.Errorf("Reconciling: Found non-device item in storage, skipping.")
			continue
		}
		if dev.Spec.SerialNumber != "" {
			deviceMap[dev.Spec.SerialNumber] = dev
		}
	}
	return deviceMap, nil
}
