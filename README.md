# Doc Explain

Doc Explain is an Intelligent RAG Platform prototype: a backend service for uploading documents, storing document identity, receiving Cloudinary upload webhooks, and queueing document processing work for a RAG pipeline.

The long-term product vision is RAG-as-a-Service for individuals, teams, and businesses. Users upload documents once, organize them into knowledge spaces, and interact with them through persistent AI sessions with citations, document metadata, and domain-aware retrieval behavior.

## Current Scope

- User registration with email, password, full name, and terms acceptance.
- Document upload endpoint with file type, size, and user ownership checks.
- Cloudinary integration for file storage.
- Cloudinary upload webhook verification.
- Document model for storing upload and processing metadata.
- Redis-backed Asynq queue for background document processing.
- Raggo-based text extraction, chunking, and embedding.
- Pgvector-backed `document_chunks` storage.
- Basic document classification and summary generation.
- Webhook idempotency guard to avoid duplicate processing.
- PostgreSQL/Gorm persistence layer.

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
- PostgreSQL with pgvector
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
    documentProcessor.go    Raggo document processing pipeline

internal/
  db/                       Postgres, Cloudinary, Redis, and Asynq setup
  documentanalysis/          Basic classification and summary helpers
  dtos/                     Request DTOs
  env/                      Environment helpers
  jobs/                     Asynq task payloads
  models/                   User, document, and webhook models
  store/                    Storage layer and migrations
```

## Requirements

- Go 1.26.3
- PostgreSQL with pgvector
- Redis
- Cloudinary account
- OpenAI-compatible embedding provider credentials

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

OPENAI_API_KEY=<embedding-api-key>
OPEN_AI_EMBEDDING_MODEL=text-embedding-3-small
MODEL_PROVIDER=openai
```

Note: the Redis client expects `REDIS_URL` in `host:port` format, for example `localhost:6379`.

## Local Database

The repository includes a pgvector-enabled Postgres service:

```bash
docker compose up -d db
```

If you already had the old plain Postgres container running, recreate it after pulling the new image:

```bash
docker compose up -d --force-recreate db
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
- Storing chunks and pgvector embeddings.
- Classifying and summarizing the document.
- Marking the document as ready or failed.

## Document Upload Flow

```text
1. User uploads a supported file with a valid `userId`.
2. API validates file size, MIME type, and extension.
3. API verifies that the user exists.
4. API uploads the file to Cloudinary using a generated document UUID.
5. API creates a `documents` row with `uploaded` status.
6. Cloudinary sends an upload webhook.
7. API verifies the webhook signature and timestamp.
8. API marks the document as `processing` if it is not already processing/completed.
9. API queues document processing work.
10. Worker extracts text, chunks, embeds, saves chunks, and marks the document as `ready`.
```

Supported uploads currently include:

```text
.pdf
.docx
.png
.jpg
.jpeg
.md
.markdown
.txt
```

For local webhook testing, expose the API with a tunnel such as ngrok or Cloudflare Tunnel, then configure Cloudinary to call:

```text
https://<your-public-tunnel>/cloudinary/webhook
```

Webhook.site is useful for inspecting Cloudinary payloads, but it does not call your local API unless you manually replay or forward requests.

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
  "originalFilename": "file.pdf",
  "bytes": 6348,
  "pages": 2
}
```

Asynq tasks use a deterministic task ID per document:

```text
document:process:<document-id>
```

The webhook also has a database-level idempotency guard, so duplicate Cloudinary deliveries do not enqueue duplicate work once a document is already `processing` or `completed`.

## PDF Notes

Cloudinary PDF delivery must be enabled for local PDF processing. If Cloudinary returns `401` with `x-cld-error: deny or ACL failure`, enable PDF/ZIP delivery in the Cloudinary security settings.

Some PDFs are valid in browsers but fail with the current Go PDF parser. Those documents are marked as `failed` instead of being retried forever. Scanned/image-only PDFs will need OCR support in a future extraction step.

## Product Documents

- `intelligent-rag-platform-prd.md`: full product requirements document.
- `prd-user-story-screen-blueprint.md`: Figma-ready screen plan based on the PRD user stories.

## Roadmap

- Add stronger PDF extraction fallback such as Poppler `pdftotext`.
- Add OCR for scanned PDFs and images.
- Add workspace and session models.
- Add document-level and workspace-level chat.
- Add citation-aware retrieval.
- Add team roles, permissions, and usage limits.
