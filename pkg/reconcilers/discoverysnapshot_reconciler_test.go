// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package reconcilers

import (
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/example/fru-tracker/internal/storage"
	"github.com/example/fru-tracker/internal/storage/ent/enttest"
	"github.com/openchami/fabrica/pkg/events"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func TestDiscoverySnapshotReconciler(t *testing.T) {
	resource.RegisterResourcePrefix("Device", "device")
	resource.RegisterResourcePrefix("DiscoverySnapshot", "discoverysnapshot")

	tests := []struct {
		name              string
		seedDevices       []*v1.Device
		payload            []v1.DeviceSpec
		expectedCount      int
		expectedSerials    map[string]string
		expectedParents    map[string]string
		expectedPhase      string
		expectedReady      bool
		expectedMessageHas []string
	}{
		{
			name: "uri merge promotes serial number",
			seedDevices: []*v1.Device{
				newDevice(t, "", "/redfish/v1/Chassis/1", "Chassis", map[string]json.RawMessage{
					"redfish_uri": rawJSONString(t, "/redfish/v1/Chassis/1"),
				}),
			},
			payload: []v1.DeviceSpec{
				{
					DeviceType:   "Chassis",
					SerialNumber: "CHASSIS-1",
					Properties: map[string]json.RawMessage{
						"redfish_uri": rawJSONString(t, "/redfish/v1/Chassis/1"),
					},
				},
			},
			expectedCount:   1,
			expectedSerials: map[string]string{"CHASSIS-1": "/redfish/v1/Chassis/1"},
			expectedParents: map[string]string{},
			expectedPhase:   "Completed",
			expectedReady:   true,
			expectedMessageHas: []string{
				"0 devices created",
				"1 updated",
				"0 parent links established",
			},
		},
		{
			name: "parent uri fallback skips cycle",
			payload: []v1.DeviceSpec{
				{
					DeviceType:   "Node",
					SerialNumber: "NODE-A",
					Properties: map[string]json.RawMessage{
						"redfish_uri": rawJSONString(t, "/redfish/v1/Systems/A"),
					},
					ParentSerialNumber: "NODE-B",
				},
				{
					DeviceType:   "Node",
					SerialNumber: "NODE-B",
					Properties: map[string]json.RawMessage{
						"redfish_uri": rawJSONString(t, "/redfish/v1/Systems/B"),
					},
					ParentSerialNumber: "NODE-A",
				},
				{
					DeviceType:   "DIMM",
					SerialNumber: "DIMM-1",
					Properties: map[string]json.RawMessage{
						"redfish_uri":        rawJSONString(t, "/redfish/v1/Systems/A/Memory/1"),
						"redfish_parent_uri": rawJSONString(t, "/redfish/v1/Systems/A"),
					},
				},
			},
			expectedCount:   3,
			expectedSerials: map[string]string{"NODE-A": "/redfish/v1/Systems/A", "NODE-B": "/redfish/v1/Systems/B", "DIMM-1": "/redfish/v1/Systems/A/Memory/1"},
			expectedParents: map[string]string{"DIMM-1": "NODE-A", "NODE-A": "NODE-B"},
			expectedPhase:   "Completed",
			expectedReady:   true,
			expectedMessageHas: []string{
				"3 devices created",
				"2 parent links established",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := enttest.Open(t, "sqlite3", "file:reconcile?mode=memory&cache=shared&_fk=1")
			t.Cleanup(func() {
				require.NoError(t, client.Close())
			})
			storage.SetEntClient(client)

			bus := events.NewInMemoryEventBus(10, 10)
			bus.Start()
			t.Cleanup(func() {
				bus.Close()
			})

			reconciler := NewDefaultDiscoverySnapshotReconciler(storage.NewStorageClient(), bus)
			seedDevices := make([]*v1.Device, 0, len(tt.seedDevices))
			for _, seed := range tt.seedDevices {
				seedDevices = append(seedDevices, seed)
			}
			if len(seedDevices) > 0 {
				require.NoError(t, storage.SaveDevicesBulk(ctx, seedDevices))
			}

			rawData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			snapshot := &v1.DiscoverySnapshot{
				APIVersion: "example.fabrica.dev/v1",
				Kind:       "DiscoverySnapshot",
				Metadata: fabrica.Metadata{
					Name: "snapshot-1",
					UID:  "snapshot-1",
				},
				Spec: v1.DiscoverySnapshotSpec{RawData: rawData},
			}
			snapshot.Metadata.Initialize(snapshot.Metadata.Name, snapshot.Metadata.UID)

			err = reconciler.reconcileDiscoverySnapshot(ctx, snapshot)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPhase, snapshot.Status.Phase)
			assert.Equal(t, tt.expectedReady, snapshot.Status.Ready)
			for _, fragment := range tt.expectedMessageHas {
				assert.Contains(t, snapshot.Status.Message, fragment)
			}

			devices, err := storage.LoadAllDevices(ctx)
			require.NoError(t, err)
			assert.Len(t, devices, tt.expectedCount)

			bySerial := make(map[string]*v1.Device)
			for _, device := range devices {
				if device.Spec.SerialNumber != "" {
					bySerial[device.Spec.SerialNumber] = device
				}
			}

			for serial, uri := range tt.expectedSerials {
				device, ok := bySerial[serial]
				require.True(t, ok, "expected device %s", serial)
				assert.Equal(t, uri, propertyString(device.Spec.Properties, "redfish_uri"))
			}

			for childSerial, parentSerial := range tt.expectedParents {
				child, ok := bySerial[childSerial]
				require.True(t, ok, "expected child %s", childSerial)
				parent, ok := bySerial[parentSerial]
				require.True(t, ok, "expected parent %s", parentSerial)
				assert.Equal(t, parent.GetUID(), child.Spec.ParentID)
			}

			if tt.name == "uri merge promotes serial number" {
				assert.Equal(t, "CHASSIS-1", devices[0].Spec.SerialNumber)
				assert.Equal(t, "CHASSIS-1", devices[0].Metadata.Name)
			}
		})
	}
}

func newDevice(t *testing.T, serialNumber, name, deviceType string, properties map[string]json.RawMessage) *v1.Device {
	t.Helper()
	device := &v1.Device{
		APIVersion: "example.fabrica.dev/v1",
		Kind:       "Device",
		Metadata: fabrica.Metadata{Name: name},
		Spec: v1.DeviceSpec{
			DeviceType:   deviceType,
			SerialNumber: serialNumber,
			Properties:   properties,
		},
	}
	device.Metadata.Initialize(device.Metadata.Name, device.Metadata.UID)
	return device
}

func rawJSONString(t *testing.T, value string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}