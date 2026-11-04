// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openchami/fabrica/pkg/fabrica"
	v1 "github.com/openchami/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/openchami/fru-tracker/internal/storage"
	"github.com/openchami/fru-tracker/internal/storage/ent/enttest"
	fabricaclient "github.com/openchami/fru-tracker/pkg/client"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var registerTestResourcePrefixes = sync.OnceValue(registerResourcePrefixes)

func TestDiscoverySnapshotIntegration(t *testing.T) {
	// 1. Setup in-memory Ent storage for the test
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	storage.SetEntClient(client)

	// 2. Register resource prefixes (required for CreateDiscoverySnapshot)
	err := registerTestResourcePrefixes()
	require.NoError(t, err)

	// 3. Setup router and routes
	r := chi.NewRouter()
	RegisterGeneratedRoutes(r)

	// 4. Prepare the payload with at least one Node, one CPU, and one DIMM
	// Looking at apis/example.fabrica.dev/v1/device_types.go and reconcilers
	// DiscoverySnapshotSpec.RawData is expected to be a list of DeviceSpec

	nodeSerial := "node-001"
	cpuSerial := "cpu-001"
	dimmSerial := "dimm-001"

	payload := []v1.DeviceSpec{
		{
			DeviceType:   "Node",
			Manufacturer: "Dell",
			SerialNumber: nodeSerial,
			Properties: map[string]json.RawMessage{
				"model": json.RawMessage(`"PowerEdge R640"`),
			},
		},
		{
			DeviceType:         "CPU",
			Manufacturer:       "Intel",
			SerialNumber:       cpuSerial,
			ParentSerialNumber: nodeSerial,
			Properties: map[string]json.RawMessage{
				"model": json.RawMessage(`"Xeon Gold 6130"`),
			},
		},
		{
			DeviceType:         "DIMM",
			Manufacturer:       "Samsung",
			SerialNumber:       dimmSerial,
			ParentSerialNumber: nodeSerial,
			Properties: map[string]json.RawMessage{
				"model": json.RawMessage(`"32GB DDR4 2666MHz"`),
			},
		},
	}

	rawData, err := json.Marshal(payload)
	require.NoError(t, err)

	createReq := CreateDiscoverySnapshotRequest{
		Metadata: fabrica.Metadata{
			Name: "test-snapshot",
		},
		Spec: v1.DiscoverySnapshotSpec{
			RawData: rawData,
		},
	}

	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	// 5. Simulate POST request
	req := httptest.NewRequest(http.MethodPost, "/discoverysnapshots", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 6. Verify response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response v1.DiscoverySnapshot
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "test-snapshot", response.Metadata.Name)
	assert.NotEmpty(t, response.Metadata.UID)

	// 7. Verify data is correctly persisted in storage layer
	persisted, err := storage.LoadDiscoverySnapshot(context.Background(), response.Metadata.UID)
	require.NoError(t, err)
	assert.Equal(t, response.Metadata.UID, persisted.Metadata.UID)

	// Compare unmarshaled versions to avoid issues with JSON formatting/ordering
	var expectedSpecs, actualSpecs []v1.DeviceSpec
	err = json.Unmarshal(rawData, &expectedSpecs)
	require.NoError(t, err)
	err = json.Unmarshal(persisted.Spec.RawData, &actualSpecs)
	require.NoError(t, err)
	assert.Equal(t, expectedSpecs, actualSpecs)
}

func TestGeneratedClientAndBackupIntegration(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:client-backup?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	storage.SetEntClient(client)
	require.NoError(t, registerTestResourcePrefixes())

	router := chi.NewRouter()
	RegisterGeneratedRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	sdk, err := fabricaclient.NewClient(server.URL, server.Client(), zerolog.Nop())
	require.NoError(t, err)

	device, err := sdk.CreateDevice(ctx, fabricaclient.CreateDeviceRequest{
		Metadata: fabrica.Metadata{
			Name:   "backup-node",
			Labels: map[string]string{"site": "test"},
		},
		Spec: v1.DeviceSpec{DeviceType: "Node", SerialNumber: "backup-node-001"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, device.Metadata.UID)

	device.Spec.Manufacturer = "Updated manufacturer"
	updated, err := sdk.UpdateDeviceSimple(ctx, device.Metadata.UID, device.Spec)
	require.NoError(t, err)
	assert.Equal(t, device.Spec, updated.Spec)
	assert.Equal(t, device.Metadata.Labels, updated.Metadata.Labels)

	rawData, err := json.Marshal([]v1.DeviceSpec{updated.Spec})
	require.NoError(t, err)
	snapshot, err := sdk.CreateDiscoverySnapshotSimple(ctx, "backup-snapshot", v1.DiscoverySnapshotSpec{RawData: rawData})
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Metadata.UID)

	backupDir := t.TempDir()
	require.NoError(t, runExport(ctx, "json", backupDir, nil, true))
	require.FileExists(t, filepath.Join(backupDir, "devices", "device-0.json"))
	require.FileExists(t, filepath.Join(backupDir, "discoverysnapshots", "discoverysnapshot-0.json"))
	importDir := t.TempDir()
	for name, resource := range map[string]interface{}{"device.json": updated, "snapshot.json": snapshot} {
		data, err := json.Marshal(resource)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(importDir, name), data, 0600))
	}
	_, err = client.Label.Delete().Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, sdk.DeleteDevice(ctx, device.Metadata.UID))
	require.NoError(t, sdk.DeleteDiscoverySnapshot(ctx, snapshot.Metadata.UID))
	devices, err := sdk.GetDevices(ctx)
	require.NoError(t, err)
	require.Empty(t, devices)

	require.NoError(t, runImport(ctx, importDir, "upsert", false, false))
	restored, err := sdk.GetDevice(ctx, device.Metadata.UID)
	require.NoError(t, err)
	assert.Equal(t, updated.Spec, restored.Spec)
	assert.Equal(t, updated.Metadata.Labels, restored.Metadata.Labels)
	restoredSnapshot, err := sdk.GetDiscoverySnapshot(ctx, snapshot.Metadata.UID)
	require.NoError(t, err)
	assert.JSONEq(t, string(rawData), string(restoredSnapshot.Spec.RawData))
}
