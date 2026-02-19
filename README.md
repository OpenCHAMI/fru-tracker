# fru-tracker

## Getting Started
This is an inventory API for OpenCHAMI, generated with Fabrica, based on an event-driven reconciliation model.

Unlike a simple CRUD API, this service is designed to be populated by a collector.
1.  A Redfish collector (`cmd/collector`) discovers hardware and `POST`s a complete `DiscoverySnapshot` resource to the API.
2.  This `POST` creates the snapshot and automatically publishes a `fru-tracker.resource.discoverysnapshot.created` event via the generated handlers.
3.  A server-side `DiscoverySnapshotReconciler` catches this event and begins processing the snapshot's `rawData` payload.
4.  The reconciler performs a "get-or-create" for each `Device` in the payload, using the **Redfish URI** as the unique key (to handle components without serial numbers).
5.  A two-pass system ensures that after all devices are created, parent/child relationships are linked by resolving the `parentSerialNumber` (from the collector) to the `parentID` (the parent's UUID in the database).

## What Can You Do To Work With This Today?

You can run the service locally and simulate a hardware discovery event to see the event-driven reconciliation in action. This requires no actual hardware or background knowledge of the system.

### 1. Start the Server
Open a terminal and start the API server:

```bash
go mod tidy
go run ./cmd/server serve
```

*What is happening:* The server initializes the local file database, starts the internal event bus, and spins up the background reconciliation workers. It is now listening for requests on `http://localhost:8080`.

### 2. Simulate a Hardware Discovery
Open a **second** terminal. Create a file named `upload_request.json` containing a mock payload with a Node and a DIMM:

```bash
cat << 'EOF' > upload_request.json
{
  "apiVersion": "example.fabrica.dev/v1",
  "kind": "DiscoverySnapshot",
  "metadata": {
    "name": "manual-snapshot-01"
  },
  "spec": {
    "rawData": [
      {
        "deviceType": "Node",
        "serialNumber": "NODE12345",
        "manufacturer": "Intel",
        "properties": {
          "redfish_uri": "/Systems/NODE12345"
        }
      },
      {
        "deviceType": "DIMM",
        "partNumber": "16GB-DDR4",
        "serialNumber": "DIMM67890",
        "parentSerialNumber": "NODE12345",
        "properties": {
          "redfish_uri": "/Systems/NODE12345/Memory/1"
        }
      }
    ]
  }
}
EOF
```

Post this payload to the server:

```bash
curl -X POST http://localhost:8080/discoverysnapshots \
  -H "Content-Type: application/json" \
  -d @upload_request.json
```

*What is happening:* You are acting as the collector. The server accepts the snapshot and publishes a `created` event. The reconciler catches this event and processes the payload in the background, creating the two devices and linking the DIMM to the Node.

### 3. Verify the Results
Retrieve the parsed devices from the API to see the results of the reconciliation:

```bash
curl -s http://localhost:8080/devices
```

*What is happening:* The output will show the two distinct `Device` resources. If you look at the `spec` for the DIMM, you will see that the `parentID` field has been automatically populated with the specific UUID of the Node, proving that the two-pass reconciler successfully executed.

### Intended Use Cases

The primary use case for `fru-tracker` is tracking hardware state changes over time using an event-driven architecture. 

Instead of requiring clients to manually compute diffs between raw hardware snapshots, the system provides a workflow for detecting hardware modifications (e.g., a DIMM replacement or CPU swap):

1. **Initial Collection:** A collector pushes an initial `DiscoverySnapshot` containing the baseline hardware state. The reconciler parses this payload and populates the database with individual `Device` resources.
2. **Hardware Modification:** A physical or configuration change occurs on the target machine.
3. **Subsequent Collection:** The collector pushes a new `DiscoverySnapshot` reflecting the current state.
4. **Event-Driven Delta Tracking:** During the reconciliation process, the system identifies differences between the newly observed state and the existing database state. For any modified component, the reconciler updates the corresponding `Device` record and automatically emits a `fru-tracker.resource.device.updated` event over the message bus. 
5. **Downstream Consumption:** External services or scripts can subscribe to this event stream to log the delta, trigger inventory alerts, or update external dashboards in real-time without needing to parse the raw snapshots.

#### Collector Integration

The `fru-tracker` service is designed to be passive and agnostic to the specific hardware management protocols used in a data center. It expects users to deploy their own collectors tailored to their environment, collecting only information useful to each site. 

To integrate a custom collector, the collector simply needs to gather the hardware state, format it as a JSON array of device specifications, and `POST` it to the `/discoverysnapshots` endpoint. 

A reference implementation of a Redfish-based collector is provided in `cmd/collector` to demonstrate this interaction and serve as a starting point for development. Also, see below for a sample payload.

### Current Capabilities

The current implementation has been validated with an end-to-end workflow using the provided Redfish collector and the event-driven reconciliation controller.

* **Redfish Discovery Collector (`cmd/collector`):** Capable of authenticating with a BMC, walking the Redfish `/Systems` tree, and extracting hardware data for Nodes, Processors (CPUs), and Memory (DIMMs). It packages this data into a `DiscoverySnapshot` payload and posts it to the API.
* **Event-Driven Triggering:** The server publishes a `fru-tracker.resource.discoverysnapshot.created` event upon receiving a snapshot, which reliably triggers the background reconciler.
* **Two-Pass Reconciliation:** 
    * **Pass 1 (Ingestion):** The reconciler parses the raw JSON payload and performs a get-or-create operation for each device, utilizing the `redfish_uri` from the properties map as a unique primary key.
    * **Pass 2 (Relationship Linking):** The reconciler evaluates the `parentSerialNumber` provided by the collector, identifies the corresponding parent device in the database, and updates the child device's `parentID` with the appropriate UUID.
* **Storage Backend:** Validated using the local file storage backend for persisting resources.

### Future Work

While the core event-driven ingestion pipeline is functional, several enhancements are planned to make `fru-tracker` production-ready:

* **Production Storage Backend:** Migrate testing and deployment documentation from the local `file` storage backend to a robust relational database (e.g., SMD using Fabrica's `ent` backend option).
* **Hardware Removal Handling:** Enhance the `DiscoverySnapshotReconciler` to detect missing components. If a previously tracked child device is absent from a new snapshot, the reconciler should update the existing `Device` record to mark it as removed, offline, or inactive.
* **Event Delta Consumer:** Build a reference implementation of an event subscriber. This service will listen to the message bus for `fru-tracker.resource.device.updated` and `deleted` events to generate human-readable changelogs and trigger alerts.
* **Collector Enhancements:** * Expand the reference Redfish collector to support additional component types (e.g., Drives, PowerSupplies, NetworkAdapters).
    * Implement secure credential management for the collector (replacing hardcoded BMC credentials).
    * Develop examples of non-Redfish collectors (e.g., an OS-level script using `dmidecode` or `lshw`).

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
* **apiVersion (String):** The API group version (e.g., "example.fabrica.dev/v1").
* **kind (String):** The resource type (e.g., "Device").
* **createdAt (Timestamp):** Timestamp of when the device was created.
* **updatedAt (Timestamp):** Timestamp of the last update.

---

## Usage

### Running the API Server
The server runs the API endpoints and the background reconciliation controller.

``` bash
# Install dependencies
go mod tidy

# Run the server (using the 'serve' command for cobra)
go run ./cmd/server serve
```

The server will start on `http://localhost:8080`.

### Running the Redfish Collector
This repository includes a command-line tool to discover hardware from a BMC via Redfish and post it to the API.

**Note:** The collector currently uses hardcoded credentials in `pkg/collector/collector.go` (`DefaultUsername` and `DefaultPassword`). These must be updated to match your target BMC.

``` bash
# Install dependencies
go mod tidy

# Run the collector, pointing it at a target BMC
go run ./cmd/collector/main.go --ip <BMC_IP_ADDRESS>
```

---

## End-to-End Verification
This section shows the successful end-to-end test run. The collector discovers hardware, posts a `DiscoverySnapshot`, and the server-side reconciler processes the data to create and link the `Device` resources.

### Step 1: Collector Output
The collector successfully found 7 devices (1 Node, 2 CPUs, 4 DIMMs) and posted them to the API.

``` bash
$ go run ./cmd/collector/main.go --ip 172.24.0.2
Starting inventory collection for BMC IP: 172.24.0.2
Starting Redfish discovery...
Redfish Discovery Complete: Found 7 total devices.
Creating new DiscoverySnapshot resource...
Successfully created snapshot with UID: discoverysnapshot-639ab206
The server reconciler will now process this snapshot.
Inventory collection and posting completed successfully.
```

### Step 2: Server Reconciliation Log
The server logs show the generated handler receiving the post, the event bus dispatching the event, and the `DiscoverySnapshotReconciler` executing the two-pass logic.

``` bash
$ go run ./cmd/server serve
...
[INFO] Reconciliation controller started with 5 workers
[INFO] Server starting on 0.0.0.0:8080
...
[DEBUG] Processing reconciliation for DiscoverySnapshot/discoverysnapshot-639ab206 (reason: Event: fru-tracker.resource.discoverysnapshot.created)
[DEBUG] Reconciling DiscoverySnapshot DiscoverySnapshot/discoverysnapshot-639ab206
[INFO] Reconciling snapshot-172.24.0.2-1770836443: Starting reconciliation
[INFO] Reconciling snapshot-172.24.0.2-1770836443: Loaded 2 devices by URI and 2 by Serial
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Processors/CPU1
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Processors/CPU2
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Memory/Memory1
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Memory/Memory2
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Memory/Memory3
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 1): Creating new device: /Systems/QSBP82909274/Memory/Memory4
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking parent relationships...
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Processors/CPU1 (UID: device-244b078d) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Processors/CPU2 (UID: device-e4973199) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Memory/Memory1 (UID: device-27b7425d) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Memory/Memory2 (UID: device-506327c2) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Memory/Memory3 (UID: device-693d311f) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443 (Pass 2): Linking /Systems/QSBP82909274/Memory/Memory4 (UID: device-3f4acf01) to parent /Systems/QSBP82909274 (UID: device-6dad4952)
[INFO] Reconciling snapshot-172.24.0.2-1770836443: Successfully reconciled
[DEBUG] Reconciliation successful for DiscoverySnapshot/discoverysnapshot-639ab206
```

### Step 3: Final Data in API (Result)
A `GET /devices` call confirms that the devices were created and linked. Note the `spec` field for the child components, which now contains the resolved `parentID` pointing to the Node's UUID (`device-6dad4952`).

``` json
[
  {
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "Device",
    "metadata": {
      "name": "/Systems/QSBP82909274",
      "uid": "device-6dad4952",
      ...
    },
    "spec": {
      "deviceType": "Node",
      "serialNumber": "QSBP82909274",
      "properties": {
        "redfish_uri": "/Systems/QSBP82909274"
      }
    }
  },
  {
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "Device",
    "metadata": {
      "name": "/Systems/QSBP82909274/Processors/CPU1",
      "uid": "device-244b078d",
      ...
    },
    "spec": {
      "deviceType": "CPU",
      "manufacturer": "Intel",
      "serialNumber": "CPU1-Serial",
      "parentID": "device-6dad4952",
      "parentSerialNumber": "QSBP82909274",
      "properties": {
        "redfish_uri": "/Systems/QSBP82909274/Processors/CPU1"
      }
    }
  }
]
```

### Data Analysis (Parent/Child Linking)
The following table shows the successful resolution of child components to their parent Node, as performed by the two-pass reconciler.

| Device | `spec.serialNumber` | `spec.parentSerialNumber` | `spec.parentID` (Resolved by Reconciler) |
| :--- | :--- | :--- | :--- |
| **Node** | `QSBP82909274` | (empty) | (empty) |
| **CPU 1** | `CPU1-Serial` | `QSBP82909274` | **`device-6dad4952`** |
| **DIMM 1** | `DIMM1-Serial` | `QSBP82909274` | **`device-6dad4952`** |

## Using Your Own Collector (Bulk Upload)

The Inventory Service is designed to be passive; it does not require direct connectivity to your management network or BMCs. If you don't want to use the provided collector, you can use an external collector to push data to the API.

While there is a provided Go collector, you can write your own collector. The API provides a single bulk endpoint via the `DiscoverySnapshot` resource.

### 1. Format Your Inventory Data
Prepare your inventory data as a JSON array of device specifications. Each object should include at least a `deviceType` and a unique identifier (either `serialNumber` or a `redfish_uri` in `properties`).

**inventory_payload.json**:
``` json
[
  {
    "deviceType": "Node",
    "serialNumber": "QSBP82909274",
    "manufacturer": "Intel",
    "properties": { "redfish_uri": "/Systems/QSBP82909274" }
  },
  {
    "deviceType": "DIMM",
    "partNumber": "16GB-DDR4",
    "serialNumber": "3128C51A",
    "parentSerialNumber": "QSBP82909274",
    "properties": { "redfish_uri": "/Systems/QSBP82909274/Memory/1" }
  }
]
```

### 2. Wrap it in a Snapshot
To upload this data, wrap the JSON array into the `rawData` field of a `DiscoverySnapshot` resource.

**upload_request.json**:
``` json
{
  "apiVersion": "example.fabrica.dev/v1",
  "kind": "DiscoverySnapshot",
  "metadata": {
    "name": "manual-upload-001"
  },
  "spec": {
    "rawData": [ ... paste your inventory_payload.json array here ... ]
  }
}
```

### 3. POST to the API
Submit the snapshot to the API. The server will immediately accept the payload (201 Created) and process the creating and linking of devices in the background.

```bash
curl -X POST http://localhost:8080/discoverysnapshots \
  -H "Content-Type: application/json" \
  -d @upload_request.json
```

This approach allows you to run the collection logic on a distinct network segment, while keeping this service isolated.