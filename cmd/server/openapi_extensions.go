// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
//
// This file contains the user-editable OpenAPI extension hook.
//
// ✅ This file is safe to edit: it will NOT be overwritten by regeneration.
//
// Add any routes that are not Fabrica-generated (legacy APIs, custom endpoints,
// WireGuard, cloud-init, etc.) to registerCustomOpenAPIPaths so they appear in
// the served OpenAPI spec and Swagger UI at /openapi.json and /docs.
//
// Example:
//
//	func registerCustomOpenAPIPaths(spec *openapi3.T) {
//	    metaDataOp := openapi3.NewOperation()
//	    metaDataOp.OperationID = "getMetaData"
//	    metaDataOp.Summary = "Cloud-init meta-data endpoint"
//	    metaDataOp.Tags = []string{"cloud-init"}
//	    metaDataOp.Responses = openapi3.NewResponses()
//	    metaDataOp.Responses.Set("200", &openapi3.ResponseRef{
//	        Value: openapi3.NewResponse().WithDescription("YAML metadata for the requesting node"),
//	    })
//	    spec.Paths.Set("/meta-data", &openapi3.PathItem{Get: metaDataOp})
//	}
package main

import "github.com/getkin/kin-openapi/openapi3"

// registerCustomOpenAPIPaths is called by GenerateOpenAPISpec after all
// Fabrica-generated resource paths have been registered.
// Add your custom / non-generated route definitions here.
func registerCustomOpenAPIPaths(spec *openapi3.T) {
	// Add custom route definitions here.
	// Example (uncomment and extend as needed):
	//
	// op := openapi3.NewOperation()
	// op.OperationID = "myCustomEndpoint"
	// op.Summary   = "My custom endpoint"
	// op.Tags      = []string{"Custom"}
	// op.Responses = openapi3.NewResponses()
	// op.Responses.Set("200", &openapi3.ResponseRef{
	//     Value: openapi3.NewResponse().WithDescription("OK"),
	// })
	// spec.Paths.Set("/my-endpoint", &openapi3.PathItem{Get: op})
	_ = spec
}
