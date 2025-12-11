# inventory-v3

## Getting Started
This is an inventory API for OpenCHAMI, generated with Fabrica, based on an event-driven reconciliation model.

Unlike a simple CRUD API, this service is designed to be populated by a collector.
1.  A Redfish collector (`cmd/collector`) discovers hardware and `POST`s a complete `DiscoverySnapshot` resource to the API.
2.  This `POST` creates the snapshot and automatically publishes a `resource.created` event via the generated handlers.
3.  A server-side `DiscoverySnapshotReconciler` catches this event and begins processing the snapshot's `rawData` payload.
4.  The reconciler performs a "get-or-create" for each `Device` in the payload, using the **Redfish URI** as the unique key (to handle components without serial numbers).
5.  A two-pass system ensures that after all devices are created, parent/child relationships are linked by resolving the `parentSerialNumber` (from the collector) to the `parentID` (the parent's UUID in the database).

### Device Data Model
All hardware data is stored in the `spec` field, representing the observed state from the last snapshot.

#### Core `spec` fields
* **deviceType (String):** The type of hardware (e.g., "Node", "CPU", "DIMM").
* **manufacturer (String):** The manufacturer name.
* **partNumber (String):** The part number.
* **serialNumber (String):** The serial number (used for parent linking).
* **parentSerialNumber (String):** The serial number of the parent device (set by the collector).
* **parentID (String):** The UUID of the parent device (set by the reconciler).
* **properties (Map):** An arbitrary key-value map for additional data.

#### Core `status` fields
* **phase (String):** The reconciliation status (e.g., "Processing", "Completed").
* **message (String):** A human-readable message from the reconciler.
* **ready (Boolean):** Indicates if the resource is fully reconciled.

<details><summary>Properties information</summary>

(This section is preserved from your template as it describes the desired data conventions.)

##### The `properties` Field for Custom Attributes
To resolve the open question regarding custom attributes, a `properties` field will be in the Device model. This field allows storing arbitrary key-value data that is not covered by the core model fields.

The `properties` field is a map where keys are strings and values can be any valid JSON type (string, number, boolean, null, array, or object). To ensure consistency and usability, the following constraints and guidelines apply.

##### Constraints on Keys
* all keys must be in **lowercase snake_case**.
* keys may only contain **lowercase alphanumeric characters** (a-z, 0-9), **underscores** (`_`), and **dots** (`.`).
* the dot character (`.`) is used exclusively as a **namespace separator** to group related attributes (e.g., `bios.release_date`).

</details>

### Metadata
* **apiVersion (String):** The API group version (e.g., "inventory/v1").
* **kind (String):** The resource type (e.g., "Device").
* **schemaVersion (String):** The version of this resource's schema.
* **createdAt (Timestamp):** Timestamp of when the device was created.
* **updatedAt (Timestamp):** Timestamp of the last update.

---

## Usage

### Running the API Server
The server runs the API endpoints and the background reconciliation controller.

```bash
# Install dependencies
go mod tidy

# Run the server (using the 'serve' command for cobra)
go run ./cmd/server serve
```

The server will start on `http://localhost:8080`.

### Running the Redfish Collector
This repository includes a command-line tool to discover hardware from a BMC via Redfish and post it to the API.

**Note:** The collector currently uses hardcoded credentials in `pkg/collector/collector.go` (`DefaultUsername` and `DefaultPassword`). These must be updated to match your target BMC.

```bash
# Install dependencies
go mod tidy

# Run the collector, pointing it at a target BMC
go run ./cmd/collector/main.go --ip <BMC_IP_ADDRESS>
```

---

## End-to-End Verification
This section shows the successful end-to-end test run. The collector discovers hardware, posts a `DiscoverySnapshot`, and the server-side reconciler processes the data to create and link the `Device` resources.

### Step 1: Collector Output
The collector successfully found 7 devices and posted them to the API.

```bash
$ go run ./cmd/collector/main.go --ip 172.24.0.2
Starting inventory collection for BMC IP: 172.24.0.2
Starting Redfish discovery...
Redfish Discovery Complete: Found 7 total devices.
Creating new DiscoverySnapshot resource...
Successfully created snapshot with UID: dis-1e7b1b56
The server reconciler will now process this snapshot.
Inventory collection and posting completed successfully.
```

### Step 2: Server Reconciliation Log
The server logs show the generated handler receiving the post, the event bus dispatching the event, and the `DiscoverySnapshotReconciler` executing the two-pass logic using the `redfish_uri` as the primary key.

```bash
$ go run ./cmd/server serve
...
[INFO] Reconciliation controller started with 5 workers
[INFO] Server starting on 0.0.0.0:8080
...
[DEBUG] Processing reconciliation for DiscoverySnapshot/dis-1e7b1b56 (reason: Event: inventory-v3.resource.discoverysnapshot.created)
[INFO] Reconciling snapshot-172.24.0.2-1763412474: Starting reconciliation
[INFO] Reconciling snapshot-172.24.0.2-1763412474: Loaded 0 existing devices into map
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 1): Creating new device: QSBP82909274
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 1): Creating new device: 3128C51A
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 1): Creating new device: 10CD71D4
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 1): Creating new device: 3128C442
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 1): Creating new device: 10CD71BE
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 2): Linking parent relationships...
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 2): Linking 3128C51A (UID: dev-bb91bd85) to parent QSBP82909274 (UID: dev-505df620)
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 2): Linking 10CD71D4 (UID: dev-bfbea2ad) to parent QSBP82909274 (UID: dev-505df620)
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 2): Linking 3128C442 (UID: dev-9df32a66) to parent QSBP82909274 (UID: dev-505df620)
[INFO] Reconciling snapshot-172.24.0.2-1763412474 (Pass 2): Linking 10CD71BE (UID: dev-b648b548) to parent QSBP82909274 (UID: dev-505df620)
[INFO] Reconciling snapshot-172.24.0.2-1763412474: Successfully reconciled
[DEBUG] Reconciliation successful for DiscoverySnapshot/dis-1e7b1b56
```

### Step 3: Final Data in API (Result)
A `GET /devices` call confirms that the devices were created and linked. Note the `spec` field for the DIMM, which now contains the resolved `parentID` pointing to the Node's UUID.

```json
[
  {
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {
      "name": "QSBP82909274",
      "uid": "dev-505df620",
      ...
    },
    "spec": {
      "deviceType": "Node",
      "serialNumber": "QSBP82909274",
      "parentSerialNumber": "",
      "properties": {
        "redfish_uri": "/Systems/QSBP82909274"
      }
    }
  },
  {
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {
      "name": "3128C51A",
      "uid": "dev-bb91bd85",
      ...
    },
    "spec": {
      "deviceType": "DIMM",
      "manufacturer": "Hynix",
      "serialNumber": "3128C51A",
      "parentID": "dev-505df620",
      "parentSerialNumber": "QSBP82909274",
      "properties": {
        "redfish_uri": "/Systems/QSBP82909274/Memory/Memory1"
      }
    }
  }
]
```

### Data Analysis (Parent/Child Linking)
The following table shows the successful resolution of a child DIMM to its parent Node, as performed by the two-pass reconciler.

| Device | `spec.serialNumber` | `spec.parentSerialNumber` | `spec.parentID` (Resolved by Reconciler) |
| :--- | :--- | :--- | :--- |
| **Node** | `QSBP82909274` | (empty) | (empty) |
| **DIMM** | `3128C51A` | `QSBP82909274` | **`dev-505df620`** |
| **DIMM** | `10CD71D4` | `QSBP82909274` | **`dev-505df620`** |