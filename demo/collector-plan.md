# FRU-Tracker Collector Extension Plan

## Objective
Extend the existing Go Redfish collector (`demo/collector`) to discover new hardware component types from a BMC and format them for the OpenCHAMI fru-tracker API.

## API Data Model Rules
The API ingests a single `DiscoverySnapshot` containing a `rawData` array of `DeviceSpec` objects. When adding new components to the collector, you must adhere to these rules:

1. **`DeviceType`**: Assign a distinct string (e.g., "Drive", "PowerSupply", "Fan").
2. **`Properties.redfish_uri`**: Every device MUST include its Redfish `@odata.id` mapped to `properties["redfish_uri"]`. This is the database primary key.
3. **`ParentSerialNumber`**: If the new component is a child of the Node (e.g., a Drive inside the chassis), it MUST include a `parentSerialNumber` that matches the Node's `serialNumber`. This ensures the server-side reconciler links them correctly.

## Implementation Steps for Copilot

When instructed to add a new hardware component (like a Drive or Power Supply), perform the following modifications to the Go code:

### 1. Update `models.go`
Create the necessary JSON structs to unmarshal the target Redfish endpoint. Embed the `CommonRedfishProperties` struct to capture standard fields like Manufacturer, PartNumber, and SerialNumber.

```go
// Example for a new component
type RedfishDrive struct {
	CommonRedfishProperties
	CapacityBytes int64 `json:"CapacityBytes"`
	Protocol      string `json:"Protocol"`
}
```

### 2. Update `SystemInventory` struct
Add a slice for the new component type to the `SystemInventory` struct in `models.go`.

```go
type SystemInventory struct {
	NodeSpec *v1.DeviceSpec
	CPUs     []*v1.DeviceSpec
	DIMMs    []*v1.DeviceSpec
    Drives   []*v1.DeviceSpec // New addition
}
```

### 3. Update `collector.go` Discovery Logic
In `getSystemInventory`, navigate to the appropriate Redfish collection URI (e.g., `/Systems/{id}/Storage` or `/Chassis/{id}/Power`). Use the existing `getCollectionDevices` helper function to iterate through the members, passing the Node's serial number as the `parentSerial` argument.

### 4. Append to Snapshot
In `discoverDevices`, append the newly populated slice from `SystemInventory` to the master `specs` array so it is included in the final API POST payload.