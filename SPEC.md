# Service Specification: fru-tracker

## 1. System Overview

**Objective:** Provides a REST API service for hardware discovery, inventory tracking, and hierarchical linking of Field Replaceable Units (FRUs) via an event-driven reconciliation model.
**Primary Domain:** Hardware lifecycle and inventory management.
**Boundaries:** This service SHOULD NOT replace the internal UUID as the primary system-of-record key, nor should it implement a full "Location" service (it must remain strictly focused on the FRU adjacency hierarchy).

## 2. Infrastructure & Scaffold Configuration

This service relies on the Fabrica framework.

* **Project Name:** fru-tracker
* **API Group:** hardware.openchami.org
* **Storage Type:** ent
* **Database Driver:** sqlite (for local testing/agent validation)
* **Required Features:** validation, events (memory bus), conditional (sha256 etag), and generation (handlers, storage, client, openapi, events, middleware, reconciliation).

## 3. Resource Requirements (Agent-Designed Schema)

### Resource: Device

* **Description:** Represents an individual piece of hardware in the system, ranging from racks and chassis to nodes, CPUs, and DIMMs.
* **Data to Capture (Spec):** * `DeviceType` (string, required)
* `Manufacturer` (string, optional)
* `PartNumber` (string, optional)
* `SerialNumber` (string, optional) - *Agent Note: Remove the `validate:"required"` tag from this field to support the new URI fallback logic.*
* `ParentID` (string, optional) - Populated by the reconciler.
* `ParentSerialNumber` (string, optional)
* `Properties` (map[string]json.RawMessage, optional) - Arbitrary key-value map. Must be leveraged to hold `redfish_uri` and `redfish_parent_uri`.


* **State to Track (Status):** Must remain an empty struct (clean state). Explicitly ensure no legacy fields like `ChildrenDeviceIds` are present in the struct.

### Resource: DiscoverySnapshot

* **Description:** A payload submitted by a hardware collector containing a batch of discovered device specifications.
* **Data to Capture (Spec):** * `RawData` (byte array or json.RawMessage) - Contains the JSON array of `DeviceSpec` objects.
* **State to Track (Status):** * `Phase` (string) - e.g., "Processing", "Completed", "Error".
* `Message` (string) - Details regarding the outcome of the reconciliation.
* `Ready` (boolean) - Indicates if the snapshot has been successfully processed.



## 4. Custom Business Logic & Reconciliation

* **Trigger:** Creation or update of a `DiscoverySnapshot` resource.
* **Action:** A two-pass reconciliation process over the snapshot payload, optimized for scale and data integrity:
* **Pre-computation Phase:** Iterate the payload to extract all `SerialNumber`, `redfish_uri`, and `redfish_parent_uri` strings. Query the database using a scoped `IN` clause to retrieve only the relevant existing `Device` records into memory, rather than pulling the entire table.
* **Pass 1 (Get, Merge, or Create):** Iterate through the payload specs. Attempt to match with an existing device by `SerialNumber`. If absent, fall back to matching by `redfish_uri`. If a record was previously created via URI and a subsequent payload provides its physical Serial Number, the reconciler must merge the Serial Number into the existing URI-generated record rather than creating a duplicate. Accumulate all new and updated structs in memory. Execute a single bulk upsert transaction to persist changes.
* **Pass 2 (Parent Linking & Cycle Detection):** Iterate through the processed devices to resolve `ParentID`. Attempt to find the parent device using `ParentSerialNumber`. If missing, fall back to `redfish_parent_uri`. Before updating the child's `ParentID` with the parent's UUID, perform a cycle detection check (e.g., depth-first search of the ancestry chain) to ensure the link does not create an infinite loop. Accumulate the link updates in memory and execute a single bulk update transaction.


* **State Update:** Transition `Phase` to "Processing" when starting. On success, update `Phase` to "Completed", set `Ready` to true, and output a `Message` summarizing the number of devices created/updated and parent links established. On payload parsing failure, set `Phase` to "Error" with the error message.

## 5. Agent Operational Directives (Strict Rules of Engagement)

You are an autonomous software engineering agent. You must achieve the target state defined in Sections 1-4 by executing terminal commands, writing code, and resolving your own errors. You are updating an existing service, not starting from scratch.

**Workflow Loop & Savepoints:**

1. **Clean (Update Directive):** Safely remove outdated Fabrica-generated files without disrupting established release code, non-generated manual business logic, or GitHub workflows. Execute `find . -type f -name "*_generated.go" -delete` to clear generated Go files, and remove generated OpenAPI specs. Do NOT delete `apis/*/*_types.go`, custom `pkg/reconcilers/*`, or any files in `.github/`.
* *Git Action:* `git add . && git commit -m "chore: clean legacy generated files"`


2. **Analyze & Design:** Read the business logic required in Section 4. Modify the exact Go struct fields required for the Spec and Status of the resources listed in Section 3 within the existing `apis/*/*_types.go` files (creating the new `hardware.openchami.org` API group directory and migrating the types). Ensure the `SerialNumber` required tag is removed.
3. **Generate:** Run `fabrica generate` to rebuild the scaffold based on the updated types and the existing `.fabrica.yaml` (updated with the new API group).
* *Git Action:* `git add . && git commit -m "feat: update schemas and regenerate artifacts"`


4. **Implement:** Update the custom logic in `pkg/reconcilers/discoverysnapshot_reconciler.go` to implement the URI fallback logic, scoped memory reads, cycle detection, and bulk database operations defined in Section 4.
5. **Verify (CRITICAL):** You must run `go mod tidy` and `go build ./...` after modifying any Go files. If the compiler outputs errors, you must read the error, modify the code, and re-compile autonomously.
6. **Test (Unit):** Update or write table-driven tests for the custom reconciliation logic to cover URI fallback, graph cycle detection, and record merging scenarios. Run `go test ./...`. Ensure tests pass.
* *Git Action:* `git add . && git commit -m "feat: implement and test optimized reconciliation logic"`


7. **Verify (Integration):** You must verify the server successfully binds to the port and routes HTTP requests.
* Start the server locally in the background using the exact required arguments (e.g., `go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`).
* Execute a `curl` POST request to the local endpoint to create a `DiscoverySnapshot` containing payload records testing the fallback and merge logic.
* If the response is a 404, 400, or 500, analyze the server logs, correct the payload or endpoint path, and re-test until you receive a successful 2xx HTTP status code.
* Terminate the background server process.


8. **Handoff (CRITICAL):** Create a `HANDOFF.md` file in the root directory. This file must contain:
* A brief summary of the business logic implemented, emphasizing the bulk operations, cycle detection, and associative merging mechanisms.
* The exact schema fields decided upon for the Spec and Status.
* The exact, verified `curl` command that succeeded in Step 7.
* The exact, verified server startup command used in Step 7.