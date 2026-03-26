# fru-tracker

## Getting Started
This is an inventory API for OpenCHAMI, generated with Fabrica, based on an event-driven reconciliation model.

Unlike a simple CRUD API, this service is designed to be populated by a collector.
1.  A Redfish collector (`cmd/collector`) discovers hardware and `POST`s a complete `DiscoverySnapshot` resource to the API.
2.  This `POST` creates the snapshot and automatically publishes a `fru-tracker.resource.discoverysnapshot.created` event.
3.  A server-side `DiscoverySnapshotReconciler` catches this event and begins processing the snapshot's `rawData` payload.
4.  The reconciler performs a "get-or-create" for each `Device` in the payload, using the **serialNumber** as the unique key.
5.  A two-pass system ensures that after all devices are created, parent/child relationships are linked by resolving the `parentSerialNumber` (from the collector) to the `parentID` (the parent's UUID in the database).

## What Can You Do To Work With This Today?

You can run the service locally and simulate a hardware discovery event to see the event-driven reconciliation in action.

### 1. Start the Server
Open a terminal and start the API server:

```bash
go mod tidy
mkdir -p data
go run ./cmd/server serve --database-url="file:./data/fru-tracker.db?cache=shared&_fk=1"
```

*What is happening*: The server initializes the local SQLite database, starts the internal event bus, and spins up the background reconciliation workers. By default, the server listens on http://localhost:8080. Note: The provided demo collector expects the server at http://localhost:8081.

### 2. Simulate a Hardware Discovery
Open a **second** terminal. Create a file named `upload_request.json` containing a mock payload with a Node and a DIMM. Note that `parentSerialNumber` is used to link the DIMM to the Node.

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

### 3. Verify the Results
Retrieve the parsed devices from the API to see the results of the reconciliation:

```bash
curl -s http://localhost:8080/devices
```

*What is happening:* The output will show the two distinct `Device` resources. The `parentID` field for the DIMM will be automatically populated with the UUID of the Node, resolved via the `parentSerialNumber`.

### Intended Use Cases

The primary use case for `fru-tracker` is tracking hardware state changes over time using an event-driven architecture.

1. **Initial Collection:** A collector pushes a `DiscoverySnapshot`. The reconciler populates the database with individual `Device` resources.
2. **Hardware Modification:** A physical change occurs (e.g., a DIMM replacement).
3. **Subsequent Collection:** The collector pushes a new `DiscoverySnapshot`.
4. **Event-Driven Delta Tracking:** The system identifies differences. For modified components, the reconciler updates the `Device` record and emits a `fru-tracker.resource.device.updated` event.
5. **Downstream Consumption:** External services subscribe to these events to log deltas or trigger alerts.

### Current Capabilities

* **Redfish Discovery Collector:** A reference implementation in `cmd/collector` (and `demo/collector.go`) walks the Redfish `/Systems` tree to extract data for Nodes, Processors, and Memory.
* **Event-Driven Triggering:** The server publishes a `created` event upon receiving a snapshot to trigger the reconciler.
* **Two-Pass Reconciliation:** * **Pass 1 (Ingestion):** Performs get-or-create for each device using `serialNumber` as the unique key.
    * **Pass 2 (Relationship Linking):** Identifies the parent device in the database using `parentSerialNumber` and updates the child's `parentID`.
* **Storage Backend:** Uses Ent ORM with a local SQLite database.

### Future Work

* **Hardware Removal Handling:** Enhance the reconciler to detect and mark missing components as removed or inactive.
* **Event Delta Consumer:** Build a subscriber to generate human-readable changelogs from update/delete events.
* **Collector Enhancements:** Support additional component types and secure credential management.

### Device Data Model
Hardware data is stored in the `spec` field, representing the observed state.

#### Core `spec` fields
* **deviceType (String):** The type of hardware (e.g., "Node", "CPU", "DIMM").
* **manufacturer (String):** The manufacturer name.
* **partNumber (String):** The part number.
* **serialNumber (String):** The unique serial number (Required).
* **parentSerialNumber (String):** The serial number of the parent device, used for linking.
* **parentID (String):** The UUID of the parent device (populated by the reconciler).
* **properties (Map):** An arbitrary key-value map for additional data (e.g., `redfish_uri`).

#### Core `status` fields
* **phase (String):** The reconciliation status (e.g., "Processing", "Completed").
* **message (String):** A human-readable message from the reconciler.
* **ready (Boolean):** Indicates if the resource is fully reconciled.
* **childrenDeviceIds (Array of Strings):** A read-only list of UIDs for devices contained within this one.

### Usage

### Running the Redfish Collector
The collector in `demo/collector.go` uses hardcoded credentials (`root` / `initial0`).

```bash
# Run the collector, pointing it at a target BMC
go run ./demo/main.go --ip <BMC_IP_ADDRESS>
```

### Using Your Own Collector (Bulk Upload)
The API provides a bulk endpoint via the `DiscoverySnapshot` resource. Wrap your inventory data (JSON array of device specifications) into the `rawData` field.

**upload_request.json**:
```json
{
  "apiVersion": "example.fabrica.dev/v1",
  "kind": "DiscoverySnapshot",
  "metadata": {
    "name": "manual-upload-001"
  },
  "spec": {
    "rawData": [
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
  }
}
```