# Internal Provider — Code-level Index & Developer Guide

Generated: 2026-01-27
Purpose: Quick reference for developers and Copilot agents to understand the internal Terraform provider implementation (`internal/provider/`). **Read this file before making changes to provider code.**

---

## Overview ✨
This folder contains the implementation of the Terraform provider `nubes` (resources and data-sources) and the HTTP client used to interact with the Taffy/Deck API.

Primary responsibilities:
- Define provider schema and configuration (`provider.go`).
- Implement resources (`*_resource.go`) and data-sources (`*_data_source.go`).
- Encapsulate HTTP interactions in `client_impl.go` (NubesClient) and helper types.
- Implement polling/wait logic and operation submission patterns used across resources.

---

## Files (short mapping)
- `provider.go` — Provider factory, Metadata, Schema, and lists of Resources/DataSources.
- `client_impl.go` — `NubesClient` implementation: request helpers, polling (`WaitForInstanceReady`, `WaitForOperation`), instance state retrieval, and operation submission.
- `edge_resource.go` / `edge_data_source.go` — Edge Gateway resource and data-source implementations.
- `vm_resource.go` — VM resource implementation (create/read/update/delete/import and operation waiting, `waitForVMOperationAndInstanceStatus`).
- `vapp_resource.go` / `vapp_data_source.go` — vApp resource and data-source.
- `vdc_resource.go` / `vdc_data_source.go` — vDC resource and data-source.
- `organization_resource.go` — Organization resource implementation.
- `postgres_resource.go` / `pgadmin_resource.go` — Postgres and PgAdmin resources.
- `s3bucket_resource.go` — S3 bucket resource.
- `quick_start_resource.go` — QuickStart resource (helper for full-stack installs).
- `tubulus_resource.go` / `tubulus_ai.go` — Tubulus resource (AI integrations). Contains `askGemini` usage.

---

## Key Patterns & Conventions 🔧
1. Resource lifecycle methods follow Terraform SDK conventions: `Metadata`, `Schema`, `Configure`, `Create`, `Read`, `Update`, `Delete`, `ImportState`.
2. Creation pattern:
   - Submit a create operation (via `NubesClient` helpers) -> wait for operation state -> wait for instance readiness (if applicable) -> apply post-creation steps (VIP, DNS, FW).
3. Operation polling:
   - Each resource implements `waitForOperationAndInstanceStatus` style helpers (e.g., VM uses specialized `waitForVMOperationAndInstanceStatus`) that poll Deck API for operation completion using `GetInstanceOperation` / `GetInstanceState`.
4. Parameter submission:
   - Before `Create`, resources collect a model (e.g., `VMResourceModel`) and call `submitVMOperationParams` / `submitOperationParams` which uses `instanceOperationCfsParams` mapping to Deck's `svcOperationCfsParamId`.
5. Error handling:
   - If operation stage `dtFinish` is `null` or the stage is stuck on `[PROCESS]`, functions return a timeout/error and propagate to Terraform user.

---

## Important Types & Functions (by file)

### provider.go
- `New(version string) func() provider.Provider` — Provider factory used by Terraform to instantiate provider.
- `NubesProvider` (type) — implements provider hooks: `Metadata`, `Schema`, `Configure`.
- `Resources()` and `DataSources()` — lists of registered resources and data-sources.

### client_impl.go
- `type NubesClient struct` — HTTP client wrapper; holds base URL, token provider, logger.
- `GetOperationId(ctx, serviceId, opName)` — Resolve operation numeric ID by name.
- `CreateInstance(ctx, displayName, serviceId, svcOperationId, params)` — Creates instance and returns UIDs.
- `WaitForInstanceReady`, `WaitForOperation` — Polling helpers.
- `GetInstanceStateDetails`, `GetInstanceOperation` — low-level getters for state & operation details.
- `Post`, `postInstance` helpers — handle posting and optionally returning Location/ID from `Location` header.

### vm_resource.go
- `NewVMResource()` — resource constructor.
- `VMResource` and `VMResourceModel` — model for user-specified parameters and mapping.
- `Create` — builds request model, submits params, runs operation (watching for `Firewall` stage and others), and treats partial success carefully.
- `waitForVMOperationAndInstanceStatus` — VM-specific wait logic (parses stages and handles FW timeouts).

### edge_resource.go & edge_data_source.go
- `NewEdgeResource()`, `NewEdgeDataSource()` — constructors.
- `EdgeResourceModel`, `readInstance`, `submitOperationParams`, `waitForOperationAndInstanceStatus` — same patterns applied for Edge resource.

### tubulus_resource.go & tubulus_ai.go
- Integrates AI flows (Gemini) with resource flow.
- `askGemini` — helper which calls AI integration for instruction parsing.

---

## How to build the provider locally (developer workflow) ⚙️
1. Build the provider binary:

```bash
# At repo root
go build -o terraform-provider-nubes ./
```

2. Make it available to Terraform for local testing:

```bash
# Option A: plugin dir
mkdir -p ~/.terraform.d/plugins/local/terraform-provider-nubes
cp terraform-provider-nubes ~/.terraform.d/plugins/local/terraform-provider-nubes/
# Option B: use plugin-dir during init
terraform init -plugin-dir=./ (not recommended if you have mixed plugins)
```

3. Run an example config:

```bash
cd examples/quick_start
terraform init
terraform apply -var="token=<YOUR_TOKEN>" -auto-approve
```

4. Useful commands during development:
- `go vet`, `golangci-lint run` (if configured), `go test ./...`.
- Use `tools/har/*` scripts to replay HAR-based scenarios when testing resource behavior.

> Note: Examples might perform destructive operations against live environment — prefer dev account and `-auto-approve` only when you expect the run.

---

## Acceptance / Integration Tests
- Use `tests/` scenarios to validate create/modify/delete flows.
- Manual acceptance: run an example on a dev account, inspect `instances` and `operations` via Deck API.
- Test idempotency: apply the same config twice and ensure `plan` shows no changes.

---

## Conventions for Agents and Contributors
- ALWAYS read `/home/naeel/terra/REPO_CONTENTS.md` and this file before making changes to provider code.
- When adding a resource:
  - Add a new `*_resource.go` and a `*_data_source.go` if discovery is required.
  - Implement `Create` / `Read` / `Update` / `Delete` / import if supported.
  - Add tests in `tests/` and examples in `examples/`.

---

## Next steps I can take ✅
- Generate a function-level index (per-file exported functions and short signatures) as a machine-readable YAML/JSON for other agents.
- Add `internal/provider/DEVELOPMENT.md` with a checklist and `make` targets for build and acceptance test steps.

If you want the function-level JSON index and a `DEVELOPMENT.md`, say "code-level" and I'll create them and commit to the repo.