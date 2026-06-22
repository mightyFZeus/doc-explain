# Doc Explain

Doc Explain is an Intelligent RAG Platform prototype: a backend service for uploading documents, storing document identity, receiving Cloudinary upload webhooks, and queueing document processing work for a RAG pipeline.

The long-term product vision is RAG-as-a-Service for individuals, teams, and businesses. Users upload documents once, organize them into knowledge spaces, and interact with them through persistent AI sessions with citations, document metadata, and domain-aware retrieval behavior.

## Current Scope

- User registration with email, password, full name, and terms acceptance.
- Document upload endpoint with file type and size checks.
- Cloudinary integration for file storage.
- Cloudinary upload webhook verification.
- Document model for storing upload and processing metadata.
- Redis connection support.
- Asynq task definition for background document processing.
- Worker skeleton for validating document processing jobs.
- Postgres/Gorm persistence layer.

## Architecture

```text
Client
  -> API server
  -> Cloudinary upload
  -> Cloudinary webhook
  -> API verifies webhook
  -> API enqueues document processing task
  -> Redis queue
  -> Worker consumes task
  -> Raggo chunking/embedding pipeline
  -> Postgres/vector storage
```

## Tech Stack

- Go
- Chi router
- Gorm
- PostgreSQL
- Redis
- Asynq
- Cloudinary
- Raggo
- Zap logger
- Validator

## Project Structure

```text
cmd/
  api.go                    HTTP router and application config
  main.go                   API entrypoint
  documentController.go     Upload and Cloudinary webhook handlers
  userController.go         User registration handler
  worker/
    main.go                 Worker entrypoint
    documentProcessor.go    Document processing task skeleton

internal/
  db/                       Postgres, Cloudinary, Redis, and Asynq setup
  dtos/                     Request DTOs
  env/                      Environment helpers
  jobs/                     Asynq task payloads
  models/                   User, document, and webhook models
  store/                    Storage layer and migrations
```

## Requirements

- Go 1.26.3
- PostgreSQL
- Redis
- Cloudinary account

## Environment Variables

```env
ENV=development
ADDR=:8080
API_URL=localhost:8080

DB_ADDR=postgres://admin:adminpassword@localhost:5433/doc-explain-db?sslmode=disable
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
DB_MAX_IDLE_TIME=5m

REDIS_URL=localhost:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0

CLOUDINARY_URL=cloudinary://<api-key>:<api-secret>@<cloud-name>
CLOUDINARY_API_SECRET=<api-secret>
```

Note: the Redis client expects `REDIS_URL` in `host:port` format, for example `localhost:6379`.

## Local Database

The repository includes a Postgres service:

```bash
docker compose up -d db
```

The default database connection is:

```text
postgres://admin:adminpassword@localhost:5433/doc-explain-db?sslmode=disable
```

## Running The API

```bash
go run ./cmd
```

Default routes:

```text
GET  /health
POST /auth/register
POST /document/upload
POST /cloudinary/webhook
```

## Running The Worker

The worker consumes `document:process` jobs from Redis using Asynq.

```bash
go run ./cmd/worker
```

The worker currently validates task payloads and reserves the processing points for:

- Fetching the uploaded document from Cloudinary.
- Extracting readable text.
- Sending content to Raggo for chunking and embeddings.
- Storing chunks and embeddings.
- Marking the document as ready.

## Document Upload Flow

```text
1. User uploads a supported file.
2. API validates file size, MIME type, and extension.
3. API uploads the file to Cloudinary.
4. Cloudinary sends an upload webhook.
5. API verifies the webhook signature and timestamp.
6. API decodes the Cloudinary payload.
7. API queues document processing work.
8. Worker processes the document asynchronously.
```

## Queue Flow

The queue uses Redis as the broker and Asynq as the Go task library.

```text
API process:
  asynq.Client -> pushes task into Redis

Worker process:
  asynq.Server -> pulls task from Redis
```

The current task type is:

```text
document:process
```

The current task payload includes:

```json
{
  "documentId": "document-id",
  "assetId": "cloudinary-asset-id",
  "publicId": "cloudinary-public-id",
  "secureUrl": "https://res.cloudinary.com/...",
  "format": "pdf",
  "bytes": 6348,
  "pages": 2
}
```

## Product Documents

- `intelligent-rag-platform-prd.md`: full product requirements document.
- `prd-user-story-screen-blueprint.md`: Figma-ready screen plan based on the PRD user stories.

## Roadmap

- Persist Cloudinary upload records to the `documents` table.
- Enqueue document processing from the Cloudinary webhook.
- Add document chunk and embedding models.
- Add Raggo processing in the worker.
- Add workspace and session models.
- Add document-level and workspace-level chat.
- Add citation-aware retrieval.
- Add team roles, permissions, and usage limits.
