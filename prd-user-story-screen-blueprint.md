# PRD User Story Screen Blueprint

**Product:** Intelligent RAG Platform  
**Purpose:** Figma-ready screen plan based on the PRD user stories, requirements, and MVP acceptance criteria.  
**Target Figma file:** Intelligent RAG Platform - Product UI Concept  

---

## Screen Set Overview

The full product flow should cover the PRD's first-time, returning-user, team, document, session, retrieval, security, and billing stories.

Recommended screens to add:

1. Sign Up
2. Log In / Reset Password
3. Workspace Creation
4. First Upload
5. Processing + Classification
6. Document Library
7. Document Detail
8. Persistent Chat Session
9. Source Citation Preview
10. Team Members + Roles
11. Workspace Settings + Security
12. Usage + Billing

---

## 1. Sign Up

### PRD Coverage

- AUTH-001: Users must be able to sign up with email and password.
- AUTH-004: Users must have a profile containing name, email, avatar, plan, and organization membership.
- User story: As a new user, I want to create an account so that I can securely manage my uploaded knowledge.

### Layout

- Two-column auth screen.
- Left panel: product value proposition and trust signals.
- Right panel: sign-up form.

### UI Elements

- Product logo: Intelligent RAG
- Heading: Create your knowledge workspace
- Name input
- Email input
- Password input
- Primary button: Create account
- Secondary action: Continue with Google
- Link: Already have an account? Log in
- Trust copy: Encrypted document processing, private retrieval, persistent sessions

### Notes

The sign-up screen should not feel like a landing page. It should be a direct entry point into the product.

---

## 2. Log In / Reset Password

### PRD Coverage

- AUTH-002: Users must be able to log in and log out securely.
- AUTH-003: Users must be able to reset passwords.

### Layout

- Compact auth card with product identity.
- Recovery state can appear as a second card or modal variant.

### UI Elements

- Email input
- Password input
- Log in button
- Forgot password link
- Reset password state with email field
- Security note: Your sessions and encrypted sources stay private

---

## 3. Workspace Creation

### PRD Coverage

- WKS-001: Users must be able to create workspaces.
- WKS-003: Users must be able to view documents and sessions inside a workspace.
- First-time flow: User creates or enters a default workspace.
- User story: As a user, I want to create multiple workspaces so that I can separate projects, clients, teams, or topics.

### Layout

- Onboarding stepper: Account -> Workspace -> Upload -> Ask.
- Central form for workspace details.
- Right preview panel showing what will be created.

### UI Elements

- Workspace name input
- Workspace type selector: Personal, Team, Organization
- Domain chips: Legal, Research, Finance, Education, Technical, General
- Privacy selector: Private, Team, Organization
- Button: Create workspace
- Preview: Default workspace includes documents, sessions, members, settings

---

## 4. First Upload

### PRD Coverage

- ING-001: Upload PDF.
- ING-002: Upload DOCX.
- ING-003: Upload XLSX and CSV.
- ING-004: URL ingestion.
- ING-007: Show document processing status.
- User story: As a user, I want to upload multiple documents at once so that I can build a complete workspace quickly.

### Layout

- Full workspace shell.
- Large upload drop zone.
- Supported source cards.
- Recent uploads rail.

### UI Elements

- Drag-and-drop area
- Upload from computer
- Add URL
- Source type chips: PDF, DOCX, XLSX, CSV, URL, Audio, Image
- Error example: Unsupported file or password-protected PDF
- Upload queue with status: Uploaded, Scanning, Processing

---

## 5. Processing + Classification

### PRD Coverage

- META-001: Detect document type/domain.
- META-002: Generate title when missing.
- META-004: Generate summary.
- META-005: Generate tags and topics.
- IDX-001: Chunk extracted content.
- IDX-003: Chunking strategy must adapt to document type and structure.
- SEC-001 and SEC-002: Encrypt original files and extracted chunks.

### Layout

- Pipeline screen after upload.
- Left: document list.
- Center: selected document pipeline.
- Right: generated metadata and selected agent strategy.

### UI Elements

- Processing steps:
  - Scan
  - Extract
  - Classify
  - Chunk
  - Encrypt
  - Embed
  - Index
- Classification result: Legal, Research, Finance, Technical, Educational
- Confidence score
- Generated title
- Generated summary
- Generated tags
- Chunking policy selected
- Encryption status

---

## 6. Document Library

### PRD Coverage

- WKS-003: View documents and sessions in workspace.
- WKS-007: Search documents within workspace.
- RAG-007: Filters by document, date, tag, owner, file type, domain.
- User story: As a user, I want generated titles and summaries so that I can understand documents at a glance.

### Layout

- App shell with sidebar.
- Document table/list in main area.
- Filter sidebar or top filter bar.

### UI Elements

- Search bar
- Filters: Domain, status, owner, file type, date, tags
- Document rows with:
  - Title
  - Domain
  - Status
  - Owner
  - Version
  - Last indexed
  - Related sessions
- Empty state: Upload your first document
- Failed state with retry action

---

## 7. Document Detail

### PRD Coverage

- Document identity: title, history, ownership, sessions.
- META-003: Extract author, date, source, language, file type, page count, size.
- META-008: Users should be able to edit generated metadata.
- SES-001: Users can create chat sessions tied to a document.

### Layout

- Header with title, status, domain, owner.
- Left/center document summary and metadata.
- Right panel with sessions and source preview.

### UI Elements

- Document title
- Domain classification
- Summary
- Metadata table
- Version history
- Owner
- Access level
- Related sessions
- Button: Start document chat
- Button: Re-index
- Button: Edit metadata

---

## 8. Persistent Chat Session

### PRD Coverage

- SES-001: Create sessions tied to a document.
- SES-002: Create sessions tied to a workspace.
- SES-003: Persist message history.
- SES-004: Store referenced documents and citations.
- SES-006: Rename sessions.
- RAG-001 through RAG-006: Retrieval and grounded answering.

### Layout

- Three-column app surface.
- Left: session list and source scope.
- Center: chat messages.
- Right: citations/source intelligence.

### UI Elements

- Session title
- Rename action
- Scope selector: Workspace, Document, Selected sources
- Message history
- AI answer with citation chips
- Composer
- Retrieval filters
- Share session action
- Archive/delete controls

---

## 9. Source Citation Preview

### PRD Coverage

- CIT-001: Answers include citations.
- CIT-002: Citations link back to original document location.
- CIT-003: Citations include title and source position.
- CIT-004: Users can open or preview cited source.
- CIT-005: Distinguish direct source evidence from AI inference.

### Layout

- Right-side drawer or full citation view.
- Source snippet highlighted.
- Retrieval metadata below.

### UI Elements

- Citation list ranked by relevance
- Source title
- Page/section/timestamp/sheet/cell range
- Snippet preview
- Relevance score
- Button: Open source
- Label: Direct evidence
- Label: AI inference
- Retrieval metadata: vector, keyword, rerank, permission filter

---

## 10. Team Members + Roles

### PRD Coverage

- WKS-004: Workspace owners can invite members.
- WKS-005: Workspace owners can assign roles.
- TEN-003: Workspace owners manage members and roles.
- Permission model: Owner, Admin, Editor, Viewer.

### Layout

- Admin/settings shell.
- Member table.
- Invite panel.

### UI Elements

- Invite by email input
- Role selector
- Member table:
  - Name
  - Email
  - Role
  - Last active
  - Access status
- Role explanation panel
- Pending invites
- Remove member action
- Permission warning: Shared sessions cannot expose restricted citations

---

## 11. Workspace Settings + Security

### PRD Coverage

- SEC-001 through SEC-008.
- TEN-001 and TEN-002: Multi-tenant isolation.
- Security stories: encrypted documents, access controls, audit logs, retention settings.

### Layout

- Settings tabs: General, Security, Retention, Audit Logs, Model Usage.
- Security overview cards.

### UI Elements

- Tenant ID
- Encryption status
- Data retention selector
- Sharing policy
- Model usage policy
- Audit log table
- Customer-managed keys placeholder
- SSO placeholder for enterprise
- Toggle: Allow session sharing
- Toggle: Require citations for grounded answers

---

## 12. Usage + Billing

### PRD Coverage

- BILL-001: Track storage usage.
- BILL-002: Track pages, tokens, embeddings, chat usage.
- BILL-003: Plans define limits.
- BILL-004: Clear feedback when approaching limits.
- BILL-005: Upgrade plans.

### Layout

- Billing dashboard.
- Usage cards and plan comparison.

### UI Elements

- Current plan
- Storage usage
- Processed pages
- Embedding usage
- Chat/retrieval usage
- Team seats
- Limit warning
- Upgrade button
- Plan cards: Free, Pro, Team, Enterprise

---

## Suggested Figma Layout

Create a new section named **PRD User Story Flow** and arrange the screens in this order:

```txt
Row 1:
Sign Up -> Log In / Reset -> Workspace Creation -> First Upload

Row 2:
Processing + Classification -> Document Library -> Document Detail -> Persistent Chat

Row 3:
Citation Preview -> Team Members + Roles -> Security Settings -> Usage + Billing
```

Use compact desktop frames, ideally `1200x780`, so the entire journey can be scanned side-by-side.

---

## User Story Mapping Summary

| Story Group | Screens |
| --- | --- |
| Account and workspace stories | Sign Up, Log In, Workspace Creation |
| Document stories | First Upload, Processing, Document Library, Document Detail |
| AI session stories | Persistent Chat, Citation Preview |
| Retrieval and citation stories | Persistent Chat, Citation Preview |
| Domain-specific stories | Processing, Document Detail, Persistent Chat |
| Security and admin stories | Team Members, Security Settings, Usage + Billing |

---

## MVP Design Completion Criteria

The Figma screen set is complete when it shows:

- New user signup
- Login and password reset
- Workspace creation
- Document upload
- Processing status
- Document classification and metadata
- Document library
- Document detail page
- Persistent chat session
- Cited answers
- Source preview
- Team invitation and roles
- Security controls
- Usage and billing limits

