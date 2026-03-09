# FRU-Tracker Collector Extension Plan

## Objective
Extend the existing Go Redfish collector (`demo/collector`) to gather additional data from a BMC and format it for the OpenCHAMI fru-tracker API. You will be asked to either discover entirely new hardware components or extract additional attributes for existing components.

## API Data Model Rules
The API ingests a single `DiscoverySnapshot` containing a `rawData` array of `DeviceSpec` objects. 

1. **`DeviceType`**: Every component must have a distinct string (e.g., "Node", "CPU", "DIMM", "Drive").
2. **`Properties.redfish_uri`**: Every device MUST include its Redfish `@odata.id` mapped to `properties["redfish_uri"]`. This acts is used by the service to determinte parent-child relations.
3. **`ParentSerialNumber`**: If the component is a child (e.g., a Drive inside the chassis), it MUST include a `parentSerialNumber` matching the parent Node's `serialNumber` to ensure the server-side reconciler links them correctly.
4. **Custom Attributes**: Any data field that is not `manufacturer`, `partNumber`, or `serialNumber` MUST be serialized to JSON bytes and stored inside the `properties` map (type `map[string]json.RawMessage`).

## Scenario 1: Adding New Hardware Components
When instructed to add a new hardware component type (e.g., Drives, Power Supplies), modify the Go code as follows:

### 1. Update `models.go`
Create the necessary JSON struct to unmarshal the target Redfish endpoint. Embed the `CommonRedfishProperties` struct.

```go
type RedfishDrive struct {
	CommonRedfishProperties
	CapacityBytes int64  `json:"CapacityBytes"`
	Protocol      string `json:"Protocol"`
}
```

### 2. Update `SystemInventory`
Add a slice for the new component type to the `SystemInventory` struct.

```go
type SystemInventory struct {
	NodeSpec *v1.DeviceSpec
	CPUs     []*v1.DeviceSpec
	DIMMs    []*v1.DeviceSpec
	Drives   []*v1.DeviceSpec // New addition
}
```

### 3. Update Discovery Logic in `collector.go`
In `getSystemInventory`, navigate to the appropriate Redfish collection URI (e.g., `/Systems/{id}/Storage`). Use the `getCollectionDevices` helper, passing the Node's serial number as the `parentSerial` argument. Then, in `discoverDevices`, append the populated slice from `SystemInventory` to the master `specs` array.

## Scenario 2: Adding New Attributes to Components
When instructed to gather additional data fields for an existing component (e.g., getting the `CapacityBytes` for a DIMM), modify the Go code as follows:

### 1. Update the Struct in `models.go`
Add the target JSON tag to the relevant struct.

```go
type RedfishMemory struct {
	CommonRedfishProperties
	CapacityMiB int `json:"CapacityMiB"` // New field
}
```

### 2. Update the Mapping Logic in `collector.go`
The existing `mapCommonProperties` helper only maps standard fields. To inject custom attributes into the `properties` map, intercept the mapped `DeviceSpec` before it is appended, extract the custom fields from your populated struct, marshal them to `json.RawMessage`, and insert them into the `Properties` map.

```go
// Example modification inside the iteration loop of getCollectionDevices:
spec := mapCommonProperties(rfProps, deviceType, memberURI, parentURI, parentSerial)

// Type assert back to specific component to get custom fields
if mem, ok := component.(*RedfishMemory); ok {
    capBytes, _ := json.Marshal(mem.CapacityMiB)
    spec.Properties["capacity_mib"] = capBytes
}

specs = append(specs, spec)
```
