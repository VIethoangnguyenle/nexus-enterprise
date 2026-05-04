# Tasks: Approval Redesign v2

## Backend — Dynamic Form Fields

- [x] Add `FormField` type to domain/models.go
- [x] Add `FormFields` to Template model
- [x] Add `FormData` to Request model
- [x] Update domain/template.go — CreateTemplateInput + UpdateTemplateInput to include FormFields
- [x] Update domain/execution.go — CreateRequestInput to include FormDataJSON
- [x] Update store/store.go — InsertTemplate/GetTemplate/ListTemplates to handle form_fields JSONB
- [x] Update store/store.go — InsertRequest to store form_data JSONB
- [x] Update REST handler — CreateTemplate/UpdateTemplate to accept form_fields
- [x] Update REST handler — CreateRequest to accept form_data
- [x] Create migration 010 — add form_fields/form_data columns
- [x] Update tenant schema provisioner (007) — add form_fields/form_data columns
- [x] Build verification: `go build ./cmd/` ✅

## Frontend — API & Hooks

- [x] Update api/approval.ts — add template CRUD types + API calls
- [x] Update api/approval.ts — add createRequest API with form_data
- [x] Update hooks/useApproval.ts — add useTemplates, useCreateTemplate, useUpdateTemplate
- [x] Update hooks/useApproval.ts — add useCreateRequest

## Frontend — New Components

- [x] Create components/approval/DynamicFormRenderer.tsx
- [x] Create components/approval/FormDataDisplay.tsx (in DynamicFormRenderer.tsx)
- [x] Create components/approval/FormFieldBuilder.tsx
- [x] Create components/approval/StepBuilder.tsx
- [x] Create components/approval/CreateRequestModal.tsx
- [x] Create components/approval/CreateTemplateModal.tsx
- [x] Create components/approval/TemplateDetailPanel.tsx

## Frontend — Route Rewrite

- [x] Rewrite approval.tsx — add Templates tab, New Request button
- [x] Replace ApprovalTable with DataTable columns config
- [x] Wire CreateRequestModal + CreateTemplateModal
- [x] Wire TemplateDetailPanel via PeekPanel
- [x] Responsive verification at 375px, 768px, 1280px ✅
- [x] Fix: Tab bar scrollable on narrow screens

## Build & Verify

- [x] Frontend build: `npx vite build` ✅
- [x] Backend build: `go build ./cmd/` ✅
- [x] Enhance ApprovalDetailPanel with FormDataDisplay ✅
