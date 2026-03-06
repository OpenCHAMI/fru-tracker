# demo/collector-plan.md

# FRU-Tracker Collector Generation Plan

## Objective
Write a script that gathers hardware inventory from a target system and posts it to the OpenCHAMI fru-tracker API.

## API Endpoint
- **URL**: `http://localhost:8080/discoverysnapshots`
- **Method**: `POST`
- **Content-Type**: `application/json`

## Data Model Requirements
The API requires a single "DiscoverySnapshot" payload that contains an array of all discovered devices in the `spec.rawData` field.

### Device Rules:
1. Every device MUST have a `deviceType` (e.g., "Node", "DIMM", "CPU").
2. Every device MUST have a `properties` object containing a `redfish_uri` string. This URI is used as the primary key by the database. It does not need to be a real Redfish endpoint, but it must be a unique string formatted like a hierarchical path (e.g., `/Systems/Host1/Memory/DIMM1`).
3. If a device is a child of another device (e.g., a DIMM inside a Node), it MUST include a `parentSerialNumber` that matches the `serialNumber` of its parent device.
4. Populate the `properties` map with the specific machine data you want to gather and store (e.g., firmware versions, temperatures, custom tags).
5. Optional root-level fields include `manufacturer`, `partNumber`, and `serialNumber`.

## Target JSON Structure
Your script must generate and POST a JSON payload exactly matching this structure:

```json
{
  "apiVersion": "example.fabrica.dev/v1",
  "kind": "DiscoverySnapshot",
  "metadata": {
    "name": "snapshot-<timestamp-or-hostname>"
  },
  "spec": {
    "rawData": [
      {
        "deviceType": "Node",
        "serialNumber": "NODE-123",
        "manufacturer": "HPE",
        "properties": {
          "redfish_uri": "/Systems/NODE-123",
          "bios_version": "v1.0.2"
        }
      },
      {
        "deviceType": "DIMM",
        "serialNumber": "DIMM-456",
        "parentSerialNumber": "NODE-123",
        "properties": {
          "redfish_uri": "/Systems/NODE-123/Memory/1",
          "clock_speed": "3200MHz"
        }
      }
    ]
  }
}
```

## Context & Examples
To see a working implementation of a collector that queries a BMC via Redfish, parses the data, and posts this exact structure, reference the Go code located in the `demo/collector/` directory of this workspace.