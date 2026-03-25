package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
	"github.com/example/fru-tracker/internal/storage"
	"github.com/example/fru-tracker/internal/storage/ent/enttest"
	"github.com/go-chi/chi/v5"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverySnapshotIntegration(t *testing.T) {
	// 1. Setup in-memory Ent storage for the test
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	storage.SetEntClient(client)

	// 2. Register resource prefixes (required for CreateDiscoverySnapshot)
	err := registerResourcePrefixes()
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
