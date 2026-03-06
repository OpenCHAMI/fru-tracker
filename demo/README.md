# fru-tracker Extensible Collector Tutorial

This tutorial demonstrates how to run the fru-tracker API locally and use GitHub Copilot to extend the provided Redfish Go collector to gather additional hardware components.

### 1. Start the API Server

Run the fru-tracker container with the SQLite database backend:

```bash
mkdir -p data && docker run -p 8080:8080 -v $(pwd)/data:/data ghcr.io/openchami/fru-tracker:0.2.1 serve --database-url="file:/data/fru-tracker.db?cache=shared&_fk=1"
```

### 2. Extend the Collector

The provided collector (`demo/collector`) automatically discovers Nodes, CPUs, and DIMMs via Redfish. Open GitHub Copilot Chat (or your preferred AI coding assistant) in your IDE to extend its capabilities using the `collector-plan.md` instructions.

Example prompt:
> "@workspace Read the `demo/collector-plan.md` file and the code in `demo/collector`. I want to extend the collector to also discover Physical Drives. Add the necessary Redfish structs and update the mapping logic to extract the drives from the Redfish Storage collection and append them to the inventory."

### 3. Run the Collector

Execute the modified Go script against your target BMC:

```bash
go run ./demo/collector --ip <BMC_IP_ADDRESS>
```

### 4. Verify the Data

Retrieve the populated devices from the API to verify the reconciliation process successfully linked the new hardware:

```bash
curl -s http://localhost:8080/devices
```
