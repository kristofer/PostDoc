# PostDoc

**PostDoc** is a lightweight web application for securely uploading PDF documents and sharing them via short, memorable URLs.

Admins log in to upload PDFs and optionally require an email address from anyone who downloads them. Visitors simply visit the short link—no account needed.

## What it does

- 📤 **Upload** – Authenticated admins upload PDF files through a simple drag-and-drop interface.
- 🔗 **Short links** – Every uploaded PDF gets a short, shareable URL (e.g. `https://example.com/report.pdf`).
- 📧 **Download tracking** – Optionally require visitors to submit an email address before the PDF is served, recording each download event.
- 📋 **Document management** – Admins can view all uploaded documents, see download counts, and delete files in bulk.
- 📊 **CSV export** – Document lists and download events can be exported as CSV for reporting.
- 👥 **Multi-admin** – Multiple admin accounts can be created, each with their own password.

## Quick start

```bash
go run . -addr :8080 -base-url http://localhost:8080
```

Open `http://localhost:8080/login` and sign in with the default credentials:
- **Username:** `admin`
- **Password:** `foobar`

## Running with Docker Compose (recommended for production)

```bash
docker compose up -d
```

Uploaded files and the SQLite database are persisted in a Docker volume under `/data`.

## Documentation

- **[OVERVIEW.md](OVERVIEW.md)** – A tour of every page in the app with descriptions of the interface.
- **[IDD_STEPS.md](IDD_STEPS.md)** – How this app was built step-by-step using Issue-Driven Development (IDD), aimed at beginner programmers.

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.24+ |
| Database | SQLite 3 (WAL mode) |
| Auth | JWT (HS256, HTTP-only cookies) |
| Templates | Go `html/template` |
| Deployment | Docker / Docker Compose |

## License

See [LICENSE](LICENSE).
