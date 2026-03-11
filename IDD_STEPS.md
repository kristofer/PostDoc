# Building PostDoc with Issue-Driven Development

This document walks through how PostDoc was built from an empty repository to a working beta, using **Issue-Driven Development (IDD)**.

It is written for beginner programmers who know some Java and/or Python and want to see how a real project grows from a rough starting point into a full application.

---

## What is Issue-Driven Development?

Issue-Driven Development (IDD) is a lightweight way of working where **every meaningful change to the code starts with a written issue** in the project's issue tracker.

Before writing a single line of code you write a short description of the *problem* or *feature* you want to address. This has several benefits:

- It forces you to think clearly about *what* you want to build before you start building it.
- The list of closed issues becomes a living history of the project—anyone can read through the issues to understand why the code is the way it is.
- It naturally creates small, focused pieces of work instead of one giant change that is impossible to review.
- In a team setting, issues make it easy to split up work: one person takes issue #4, another takes issue #6.

IDD does not require any special tool. GitHub Issues (used here) works perfectly; so does a plain text file, a Trello board, or a sticky note on a whiteboard.

---

## The Starting Point

Every project begins somewhere. PostDoc started as the simplest thing that could possibly work:

- A Go HTTP server that accepts a `multipart/form-data` POST containing a PDF file.
- The file is saved to disk with its original name.
- A plain URL like `http://localhost:8080/report.pdf` is returned so the caller can share it.

There was no login, no database, no admin page—just a single route that saved a file and returned a URL.

This "version zero" was enough to prove the core idea: *upload a PDF, get a shareable link*.

If you are a Java programmer, think of it as a single `HttpServlet` with a `doPost` method that calls `request.getPart("file")` and writes the bytes to disk, then prints the URL. In Python it would be a one-function Flask app using `request.files["file"].save(path)`.

---

## Issue #2 – Need Upload Security

**Problem:** Anyone who knew the server address could upload files. There was no way to prevent abuse, and there was no concept of "who uploaded what."

**Solution:** Add a login system so only authorized admins can reach the upload form.

### What was built

- An `admins` table in a new SQLite database stores usernames and bcrypt-hashed passwords.
- A `POST /login` route validates credentials and issues a **JWT** (JSON Web Token) stored in an HTTP-only cookie.
- An **auth middleware** function wraps every protected route: it reads the cookie, verifies the JWT, and either lets the request through or redirects to `/login`.
- An admin management page (`/admin`) lets logged-in admins add new admin accounts and delete old ones.
- A default `admin` / `foobar` account is created on first run if no admins exist.

### Key concepts for beginners

**Hashing passwords** – A password is never stored as plain text. Instead, `bcrypt` turns `"foobar"` into a long random-looking string like `$2a$10$...`. When a user logs in, `bcrypt` can check whether the password *matches* the stored hash without the hash ever being reversible. Java has `BCrypt` in Spring Security; Python has `passlib` or `bcrypt`.

**JWT** – After a successful login, the server creates a small signed token containing the username and an expiry time. The token is sent to the browser as a cookie. On every subsequent request the browser sends the cookie back automatically, and the server verifies the signature to confirm the user is who they say they are. This is *stateless* authentication: the server does not need to remember any session data.

**Middleware** – Rather than copy-pasting the "is this user logged in?" check into every handler, it is written once as a wrapper function. In Go:

```go
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !isAuthenticated(r) {
            http.Redirect(w, r, "/login", http.StatusFound)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Every protected route is then registered as `auth.Middleware(myHandler)`.

---

## Issue #4 – Need a Way to Change the Password

**Problem:** The default password `foobar` is not secret, and there was no way for an admin to change their own password without editing the database directly.

**Solution:** Add a "Change password" form to the admin panel.

### What was built

- A `POST /admin/change-password` route accepts the current password, the new password, and a confirmation.
- It verifies the current password with `bcrypt.CompareHashAndPassword`.
- It hashes the new password and updates the row in the `admins` table.
- If anything is wrong (wrong current password, new passwords don't match) the form is re-shown with an error message.

### Key concepts for beginners

This issue is a good example of a small, focused feature that makes the app meaningfully more useful. Notice that:

- No new tables or routes were needed beyond one new POST handler.
- The existing bcrypt and database infrastructure from Issue #2 was reused.
- Error handling is important: always check that the "new password" and "confirm password" fields match *and* that the user knows their old password before saving.

---

## Issue #6 – Build a Docker Compose File

**Problem:** Running `go run .` on a laptop works for development, but deploying to a server means the SQLite database and uploaded files might get wiped every time the server is restarted.

**Solution:** Package the app as a Docker image and provide a Docker Compose file for production.

### What was built

- A **`Dockerfile`** that uses a two-stage build:
  1. A "builder" stage compiles the Go binary inside a Go container.
  2. A "runtime" stage copies only the compiled binary and templates into a small Debian image, keeping the final image lean.
- A **`docker-compose.yml`** that:
  - Mounts a persistent volume at `/data` so the SQLite database and uploaded files survive container restarts.
  - Sets the `base-url` to the production domain via an environment variable.
  - Adds a health check that polls `/login` so Docker knows whether the container is healthy.
- SQLite is configured with **WAL (Write-Ahead Logging)** mode and other performance flags so it can handle concurrent reads without locking.

### Key concepts for beginners

**Docker multi-stage builds** reduce the final image size dramatically. The first stage needs the full Go toolchain (~600 MB); the second stage only needs the compiled binary and HTML templates (a few megabytes).

**Persistent volumes** are how you keep data across container restarts. Without a volume, every `docker compose up` starts with an empty database and an empty uploads folder—all previously uploaded files would vanish.

**WAL mode** in SQLite allows readers and writers to work at the same time without blocking each other. This is important for a web app where multiple users might request pages simultaneously.

---

## Issue #8 – Change Downloads

**Problem:** When a file named `report.pdf` was uploaded, the slug generated was just `report` (without the extension). When a visitor visited the short link their browser had no idea the file was a PDF and often showed garbled text or asked what app to open it with.

**Solution:** Preserve the `.pdf` extension in the slug and set the correct MIME type when serving the file.

### What was built

- The slug generation now keeps the file extension: `report.pdf` → slug `report.pdf`.
- When serving a document, the response header `Content-Type: application/pdf` is set explicitly, so browsers know how to handle the file.
- The `Content-Disposition` header is also set to `inline`, suggesting the browser should display the PDF rather than trigger a download dialog.

### Key concepts for beginners

**MIME types** (also called media types) tell the browser what kind of data it is receiving. `text/html` means a web page; `application/pdf` means a PDF. Without the correct MIME type a browser may refuse to display the file or display it incorrectly.

This issue is a good example of a bug that only shows up at runtime—you cannot see it by reading the code, you have to actually try it in a browser.

---

## Issue #10 – Need a Document Tracking Page for Admins

**Problem:** There was no way to see which documents had been uploaded, who uploaded them, how large they were, or what their short URLs were—except by looking directly at the database.

**Solution:** Build an admin "Documents" page that lists all uploaded files with management controls.

### What was built

- A new `GET /documents` route renders a table of all documents: filename, uploader, upload date, file size, download count, and short URL.
- A checkbox on each row allows bulk selection, and a **Delete Selected** button removes the selected files from disk and from the database. A JavaScript confirmation dialog prevents accidental deletions.
- When there are more than 30 documents, the list is **paginated**: only 30 rows are shown at a time, with "Prev" and "Next" links.

### Key concepts for beginners

**Pagination** is important for performance and usability. If you loaded every document at once into a table, the page would become slow once thousands of documents exist. Instead, the database query uses `LIMIT 30 OFFSET (page-1)*30` to fetch only the rows for the current page.

**Bulk operations** (checking many boxes and pressing Delete once) are much friendlier than deleting items one at a time. The form sends a list of selected IDs as a repeated form field; the handler loops over them and deletes each one.

---

## Issue #12 – Documents Download Needs Email Tracking

**Problem:** Admins wanted to know *who* was downloading documents—not just that a download happened, but the email address of the person.

**Solution:** Add an optional "Track downloads" mode: when enabled, visitors must enter a valid email address before the PDF is served.

### What was built

- The upload form gains a **Track downloads** checkbox. Whether tracking is enabled is stored alongside the document metadata in the database.
- When a tracked document's short link is visited (`GET /{slug}`), instead of serving the PDF immediately, the server renders an **email prompt** form.
- When the form is submitted (`POST /{slug}`), the server validates the email address, records a new row in the `download_events` table (email + timestamp), and then redirects back to `GET /{slug}` which now serves the PDF. This is the classic **Post-Redirect-Get** pattern.
- The Documents page shows a 📋 link next to the download count for tracked documents, leading to a Download Events page that lists every email and timestamp.

### Key concepts for beginners

**Post-Redirect-Get (PRG)** is a web pattern that prevents the "are you sure you want to resubmit the form?" browser dialog. After a successful POST the server sends a `302 Redirect` response; the browser then makes a GET request to the new URL. If the user refreshes the page, only the GET is repeated—not the form submission.

**Data modelling** – This feature required a new table `download_events` with a foreign key pointing to the `documents` table. A foreign key means "every row in `download_events` must reference a row that actually exists in `documents`." This prevents orphaned tracking records from accumulating for documents that no longer exist.

---

## Issue #14 – Tracking the Number of Downloads is Off by One

**Problem:** After the email tracking feature was added, a bug was noticed: the download counter was wrong. After 2 downloads the count showed 3; after 3 it showed 5.

**Problem analysis:** The counter was being incremented in two places—once when the email was recorded and once when the PDF was served. Because the Post-Redirect-Get pattern makes *two* requests (the POST that records the email, then the GET that serves the PDF), the counter was being incremented twice per actual download.

**Solution:** Remove the duplicate increment so the count is only updated once per download event.

### Key concepts for beginners

This issue is a classic **off-by-one bug** caused by an interaction between two features (counting and redirect). The fix was a one-line deletion—but finding *where* to delete it required reading the code carefully and understanding the sequence of HTTP requests.

Good debugging practice: before changing anything, reproduce the bug reliably (download a document 2 times, observe the count is 3, not 2), then add logging or use a debugger to trace the execution path and find where the count is modified.

---

## Issue #16 – Minor: Add CSV Download

**Problem:** Admins sometimes needed to share the list of documents or the download events with colleagues who do not have admin accounts—for example, to analyze downloads in a spreadsheet.

**Solution:** Add "Download CSV" buttons to both the Documents page and the Download Events page.

### What was built

- `GET /documents/csv` – Returns the full document list as a CSV file.
- `GET /documents/{id}/downloads/csv` – Returns the download events for one document as a CSV file.
- Both routes set the `Content-Type: text/csv` and `Content-Disposition: attachment; filename=...` headers so the browser downloads the file rather than displaying it.

### Key concepts for beginners

**CSV export** is one of the most common "small but useful" features in admin tools. The Go standard library's `encoding/csv` package makes it straightforward: create a `csv.Writer`, write the header row, then loop over the data rows.

Choosing the right `Content-Disposition` value matters:
- `inline` → the browser tries to display the file.
- `attachment; filename="export.csv"` → the browser downloads the file with the given name.

---

## Summary – The Feature Arc

| Issue | Type | What changed |
|---|---|---|
| (start) | Feature | Basic PDF upload, in-memory, no security |
| [#2](https://github.com/kristofer/PostDoc/issues/2) | Feature | Login, JWT auth, admin management |
| [#4](https://github.com/kristofer/PostDoc/issues/4) | Feature | Change-password form |
| [#6](https://github.com/kristofer/PostDoc/issues/6) | Infrastructure | Dockerfile, Docker Compose, SQLite WAL |
| [#8](https://github.com/kristofer/PostDoc/issues/8) | Bug fix | Preserve `.pdf` extension; set correct MIME type |
| [#10](https://github.com/kristofer/PostDoc/issues/10) | Feature | Admin Documents page, pagination, bulk delete |
| [#12](https://github.com/kristofer/PostDoc/issues/12) | Feature | Email tracking, email prompt, download events log |
| [#14](https://github.com/kristofer/PostDoc/issues/14) | Bug fix | Download counter off-by-one |
| [#16](https://github.com/kristofer/PostDoc/issues/16) | Feature | CSV export for documents and download events |

### What this arc shows

1. **Start minimal** – The first version did one thing only. There was no login, no database, no fancy UI.
2. **Add security early** – As soon as the app needed to go online, authentication was the first thing added (Issue #2).
3. **Make operations safe** – Issue #4 added the ability to change passwords before any real deployment.
4. **Operationalize** – Issue #6 made the app deployable without losing data.
5. **Fix rough edges** – Issue #8 fixed a user-visible bug (wrong MIME type) that was only discovered by trying the app in a real browser.
6. **Add admin tooling** – Issue #10 gave admins visibility into what was happening.
7. **Add the headline feature** – Issue #12 implemented download tracking, the feature that makes PostDoc useful for lead generation or audience analysis.
8. **Fix bugs introduced by new features** – Issue #14 shows how a new feature can accidentally break an existing one, and how to find and fix it.
9. **Small quality-of-life improvements** – Issue #16 added CSV export, a small change with high practical value.

This is a typical arc for a small web app. If you are building your own project, you can follow the same pattern: start with the simplest thing that works, add security, make it deployable, then iteratively add features—one issue at a time.

---

## Further Reading

- **[OVERVIEW.md](OVERVIEW.md)** – Screenshots and detailed descriptions of every page in the app.
- **[README.md](README.md)** – Quick-start instructions and tech-stack summary.
