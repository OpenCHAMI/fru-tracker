# fru-tracker Extensible Collector Tutorial

This tutorial demonstrates how to run the fru-tracker API locally and use GitHub Copilot to extend the provided Redfish Go collector to gather additional hardware components.

### 1. Start the API Server

Run the fru-tracker container with the SQLite database backend:

Create a directory for the SQLite database file and ensure you can write to it:

```bash
mkdir -p data && chmod 777 data
```

Pull the docker image and run the server:

```bash
docker run -p 8081:8081 -v $(pwd)/data:/data ghcr.io/openchami/fru-tracker:0.2.1 serve --database-url="file:/data/fru-tracker.db?cache=shared&_fk=1" --port 8081
```

### 2. Extend the Collector

The provided collector (`demo/collector`) automatically discovers Nodes, CPUs, and DIMMs via Redfish. Open GitHub Copilot Chat (or your preferred AI coding assistant) in your IDE to extend its capabilities using the `collector-plan.md` instructions.

For example, let's look at our DIMMs:
```json
# curl -sk -H "X-Auth-Token: <token>" https://172.24.0.3/redfish/v1/Systems/<system>/Memory/Memory1 | jq
{
  "@odata.context": "/redfish/v1/$metadata#Memory.Memory",
  "@odata.id": "/redfish/v1/Systems/<system>/Memory/Memory1",
  "@odata.type": "#Memory.v1_7_1.Memory",
  "Id": "Memory1",
  "Name": "Memory 1",
  "Description": "System Memory",
  "MemoryType": "DRAM",
  "MemoryDeviceType": "DDR4",
  "BaseModuleType": "RDIMM",
  "CapacityMiB": 16384,
  "DataWidthBits": 64,
  "BusWidthBits": 72,
  "Manufacturer": "Hynix",
  "SerialNumber": "<serial-number>",
  "PartNumber": "<part-number>    ",
  "AllowedSpeedsMHz": [
    2400
  ],
  "MemoryMedia": [
    "DRAM"
  ],
  "RankCount": 2,
  "DeviceLocator": "CPU1_DIMM_A1",
  "MemoryLocation": {
    "Channel": 0,
    "MemoryController": 0,
    "Slot": 1,
    "Socket": 0
  },
  "ErrorCorrection": "MultiBitECC",
  "OperatingSpeedMhz": 2400,
  "Metrics": {
    "@odata.id": "/redfish/v1/Systems/<system>/Memory/Memory1/MemoryMetrics"
  },
  "Oem": {
    "Intel_RackScale": {
      "@odata.type": "#Intel.Oem.Memory",
      "VoltageVolt": 1.2
    }
  },
  "Status": {
    "State": "Enabled",
    "Health": "OK",
    "HealthRollup": "OK"
  },
  "@odata.etag": "bf2501e0b654e98e3d88d274a96b14fd"
}
```

How about we add the `CapacityMiB` and `OperatingSpeedMhz` into our collector?

**Option A: Add a completely new hardware component**
> "@workspace Read the `demo/collector-plan.md` file and the code in `demo/collector`. I want to extend the collector to also discover Physical Drives. Add the necessary Redfish structs and update the mapping logic to extract the drives from the Redfish Storage collection and append them to the inventory."

**Option B: Add new data fields to existing components**
> "@workspace Read the `demo/collector-plan.md` file and the code in `demo/collector`. I want to modify the collector to gather the `CapacityMiB` and `OperatingSpeedMhz` for each DIMM. Update the Redfish structs and ensure these new fields are extracted and saved as JSON bytes into the `properties` map of the `DeviceSpec`."

### 3. Run the Collector

Execute the modified Go script against your target BMC:

```bash
go run ./demo/collector --ip <BMC_IP_ADDRESS>
```

### 4. Verify the Data

Retrieve the populated devices from the API to verify the reconciliation process successfully linked the new hardware:

```bash
curl -s http://localhost:8081/devices
```
