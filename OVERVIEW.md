# PostDoc – App Overview

This document describes what PostDoc looks like and how it is used, from first login to sharing a PDF link.

PostDoc has two distinct audiences:

- **Admins** – people who log in to upload documents and manage the site.
- **Visitors** – anyone with a short link who wants to download a PDF.

---

## The Login Page (`/login`)

When any protected page is visited without a valid session, the user is redirected to the login page.

```
┌─────────────────────────────────┐
│         ZCW PostDoc             │
│    Admin access required.       │
│                                 │
│  ┌─────────────────────────┐    │
│  │ Sign in                 │    │
│  │                         │    │
│  │ Username  [__________]  │    │
│  │ Password  [__________]  │    │
│  │                         │    │
│  │     [  Sign in  ]       │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

If the credentials are wrong, a red error banner appears above the form. Default credentials on a fresh install are `admin` / `foobar`.

---

## The Upload Page (`/`)

After logging in the admin lands on the main upload page.

```
┌──────────────────────────────────────────────────┐
│                  ZCW PostDoc                     │
│        Upload a PDF and get a short link.        │
│                                                  │
│  ⚙ Admin   📄 Documents   Sign out              │
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ Select a PDF                               │  │
│  │  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐  │  │
│  │     ↑  Click to browse or drag & drop    │  │  │
│  │  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘  │  │
│  │                                            │  │
│  │      [  Upload & Get Link  ]               │  │
│  │                                            │  │
│  │  ☐  Track downloads (require email)        │  │
│  │                                            │  │
│  │  Only PDF files. Max size: 32 MB.          │  │
│  └────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

Key elements:

| Element | Purpose |
|---|---|
| Drop zone | Drag a PDF onto it, or click to open a file picker. Only `.pdf` files are accepted. |
| **Upload & Get Link** button | Submits the form. The file is stored and a short URL is generated. |
| **Track downloads** checkbox | When checked, visitors must type a valid email address before the PDF is served. Their email and the download timestamp are recorded. |
| Navigation links | Quick access to the Admin panel, the Documents list, and logout. |

---

## The Success Page

After a successful upload, the admin sees a confirmation card with the generated short link.

```
┌────────────────────────────────────────────────────┐
│                   ZCW PostDoc                      │
│                                                    │
│  ⚙ Admin   Sign out                               │
│                                                    │
│  ┌──────────────────────────────────────────────┐  │
│  │  ✅  Document uploaded!                      │  │
│  │  report.pdf                                  │  │
│  │                                              │  │
│  │  Your shareable link:                        │  │
│  │  ┌──────────────────────────────────────┐   │  │
│  │  │ https://example.com/report.pdf [Copy]│   │  │
│  │  └──────────────────────────────────────┘   │  │
│  │                                              │  │
│  │        ← Upload another document            │  │
│  └──────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────┘
```

The **Copy** button writes the link to the clipboard. The admin can then share it by email, chat, or any other channel.

---

## The Documents Page (`/documents`)

This page lists every uploaded document. It is only accessible to logged-in admins.

```
┌────────────────────────────────────────────────────────────────┐
│                     ZCW PostDoc – Documents                    │
│                                                                │
│  ⚙ Admin   📤 Upload   Sign out          [Download CSV]       │
│                                                                │
│  ┌──┬─────────────────┬──────────┬──────────┬───────┬───────┐  │
│  │☐ │ Filename        │ Uploaded │   By     │  Size │Downloads│ │
│  ├──┼─────────────────┼──────────┼──────────┼───────┼───────┤  │
│  │☐ │ report.pdf      │ 10 Mar   │ admin    │ 1.2MB │  3 📋  │ │
│  │☐ │ slides.pdf      │ 11 Mar   │ admin    │ 4.7MB │  0     │ │
│  └──┴─────────────────┴──────────┴──────────┴───────┴───────┘  │
│                                                                │
│  [  Delete Selected  ]                    ← Prev  1  Next →   │
└────────────────────────────────────────────────────────────────┘
```

- Select documents using the checkboxes, then click **Delete Selected**. A confirmation dialog prevents accidental deletions.
- The 📋 icon next to a download count is a link to the full download-events log for that document (only visible when tracking is enabled).
- If there are more than 30 documents the list is paginated.
- **Download CSV** exports the complete document list as a spreadsheet.

---

## The Download Events Page (`/documents/{id}/downloads`)

When a document has download tracking enabled, each download is logged. Clicking the 📋 link on the Documents page opens this view.

```
┌──────────────────────────────────────────────────────────────┐
│             ZCW PostDoc – Downloads for report.pdf           │
│                                                              │
│  ← Back to Documents              [Download CSV]            │
│                                                              │
│  ┌───────────────────────────────┬──────────────────────┐   │
│  │ Email                         │ Downloaded At        │   │
│  ├───────────────────────────────┼──────────────────────┤   │
│  │ alice@example.com             │ 2026-03-11 09:14:02  │   │
│  │ bob@example.com               │ 2026-03-11 10:30:55  │   │
│  └───────────────────────────────┴──────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

- **Download CSV** exports the event log for this document.

---

## The Admin Management Page (`/admin`)

Accessible to any logged-in admin, this page manages admin accounts.

```
┌──────────────────────────────────────────────────────┐
│              ZCW PostDoc – Admin Panel               │
│                                                      │
│  Current admins                                      │
│  ┌────────────┬────────────────────────────────┐     │
│  │ Username   │ Actions                        │     │
│  ├────────────┼────────────────────────────────┤     │
│  │ admin      │ [Change password]  [Delete]    │     │
│  └────────────┴────────────────────────────────┘     │
│                                                      │
│  Add a new admin                                     │
│  Username [______________]                           │
│  Password [______________]                           │
│  [  Add admin  ]                                     │
└──────────────────────────────────────────────────────┘
```

Any existing admin can add new admins or delete others. A logged-in admin can also change their own password.

---

## Visitor View – Direct Download

When a visitor opens a short link for a document that does **not** require email tracking, the PDF is served immediately by the browser (either displayed inline or downloaded, depending on browser settings).

---

## Visitor View – Email Prompt

When the **Track downloads** option was checked during upload, visitors see this page before the PDF is served:

```
┌──────────────────────────────────────────────────────┐
│                   ZCW PostDoc                        │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  📄  report.pdf                               │  │
│  │                                               │  │
│  │  Please enter your email to download:        │  │
│  │  Email  [_______________________________]    │  │
│  │                                               │  │
│  │           [  Download  ]                      │  │
│  └────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

If the email is invalid the form is re-shown with an error message. Once a valid email is submitted the PDF is served and the event is logged.

---

## URL Structure

| URL | Who can access | Purpose |
|---|---|---|
| `/login` | Everyone | Sign in |
| `/logout` | Logged-in admins | Sign out |
| `/` | Logged-in admins | Upload a document |
| `/upload` (POST) | Logged-in admins | Handles the upload form |
| `/admin` | Logged-in admins | Manage admin accounts |
| `/documents` | Logged-in admins | List all documents |
| `/documents/csv` | Logged-in admins | Export document list as CSV |
| `/documents/{id}/downloads` | Logged-in admins | View download events for one document |
| `/documents/{id}/downloads/csv` | Logged-in admins | Export download events as CSV |
| `/{slug}` (GET/POST) | Everyone | Download (or email-gate) a PDF |

---

## Architecture in one diagram

```
Browser                  PostDoc (Go)              Storage
  │                          │                        │
  │  GET /{slug}             │                        │
  │─────────────────────────>│                        │
  │                          │  SELECT document       │
  │                          │───────────────────────>│ SQLite
  │                          │<───────────────────────│
  │  (tracked?) show email   │                        │
  │  prompt, else serve PDF  │  Read file             │
  │<─────────────────────────│───────────────────────>│ Filesystem
  │                          │<───────────────────────│
  │  PDF bytes               │                        │
  │<─────────────────────────│                        │
```

All uploaded PDFs are stored on disk. Document metadata (slug, uploader, size, tracking flag) and download events (email, timestamp) are stored in a SQLite database.
