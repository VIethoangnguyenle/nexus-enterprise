# file-preview

## Purpose
Any file stored in the drive can be opened in a viewer without leaving the product: images, PDF, video, audio and
text as they are, Office and OpenDocument files as a PDF the server makes. The right to preview is the right to
download; the documents are converted on the deployment's own machines and never sent elsewhere.

This section is the server part. The viewer (client part) is added by the frontend phase.

## Requirements

### Requirement: A preview answers how to show a file
`GET /api/drive/files/{id}/preview` SHALL answer 200 with one of:

- `{"kind":"native","url":<signed GET of the original>,"mime":<type>}` for image/*, video/*, audio/*, text/*,
  `application/pdf`, `application/json` and CSV. When the recorded type says nothing (empty or octet-stream) the file
  name's extension decides; a `.csv` recorded as a spreadsheet type is CSV text.
- `{"kind":"sheet","url":<signed GET of the original>,"mime":<type>}` for xls, xlsx and ods (by extension, or by type
  when the name has none). Spreadsheets are never converted: the client reads them and shows a table. A `.csv` is
  not a sheet but CSV text (native).
- `{"kind":"pdf","url":<signed GET of a PDF>,"source_mime":<type of the file>}` for doc, docx, ppt, pptx, odt, odp
  and rtf (by extension, or by type when the name has none).
- `{"kind":"unavailable","reason":<reason>}` for everything else (`unsupported`), and when a document cannot be
  converted (`too_large`, `conversion_failed`, `conversion_unavailable`, `conversion_busy`).

A type a browser runs script from when opened as a page (HTML, XHTML, SVG, XML) SHALL be answered `native` with
mime `text/plain`: the client shows it as source and never navigates to or embeds it.

A conversion that cannot be done SHALL be an answer (200, `unavailable`), never a 5xx: the client then offers the
download. Signed URLs are the ones a download gets and expire alike; the response never carries an object key or id
the user did not already hold.

#### Scenario: A Word file
- **WHEN** a member with read opens the preview of `Báo cáo.docx` for the first time
- **THEN** the answer is `pdf` with a URL of a PDF whose text is the document's

#### Scenario: A spreadsheet
- **WHEN** a member with read opens the preview of `số liệu.xlsx`
- **THEN** the answer is `sheet` with the original's URL, and nothing is converted

#### Scenario: A zip
- **WHEN** a member opens the preview of `bundle.zip`
- **THEN** the answer is `unavailable` with reason `unsupported`

### Requirement: Preview needs the right a download needs
The preview SHALL require `read` on the file's own node, exactly as the download URL does, and refuse in the same
words: an unknown id is 404 `file not found`, a caller without read is 403 `access denied`. A folder, or an upload
never confirmed, has no preview (404, after the right is checked). A refused caller SHALL cost no conversion and
SHALL learn nothing about the file.

#### Scenario: A stranger
- **WHEN** a user without read on a file asks for its preview
- **THEN** the answer is 403, the same as for its download, and nothing is converted

### Requirement: Documents are converted to PDF once per content
Conversion SHALL run on the internal Gotenberg (LibreOffice) service when the file is first previewed. The PDF is
stored in the file's workspace bucket under `drive/previews/<file id>/<content identity>/preview.pdf`, where the
content identity is the original object's stored-at time and size, so replaced content never serves an old
rendition. A stored rendition SHALL be reused without converting again. Simultaneous first previews of one file
SHALL share one conversion, which is not cancelled by any one viewer leaving. When a file is deleted for good, its
rendition SHALL be deleted with it.

#### Scenario: Second view
- **WHEN** the same file is previewed again
- **THEN** the same rendition is returned and Gotenberg is not called

#### Scenario: Ten people open it at once
- **WHEN** ten members request the preview of a never-converted file together
- **THEN** one conversion runs and all ten get the PDF

#### Scenario: Permanent delete
- **WHEN** a previewed file is permanently deleted
- **THEN** its PDF rendition is gone from storage too

### Requirement: Conversion is bounded and degrades to a download
The limits SHALL be: a source of at most 50 MB (`PREVIEW_MAX_SOURCE_MB`) is converted, judged by the size on record
and the size in storage, otherwise `too_large`; a PDF of at most 80 MB (`PREVIEW_MAX_PDF_MB`) is kept, otherwise
`too_large`; a conversion has 45 s (`PREVIEW_CONVERT_TIMEOUT_SECONDS`), counted from waiting for a slot to the PDF
being stored, otherwise `conversion_failed`; at most 2 conversions (`PREVIEW_MAX_CONCURRENT`) run at once in a
process, and a request that gets no slot in its time is `conversion_busy`. The PDF is written to a temporary file,
never held whole in memory, and the file is removed afterwards.

A refusal or failure by Gotenberg is `conversion_failed`; an unreachable Gotenberg, or none configured, is
`conversion_unavailable` and logged as a warning. A file that failed (`conversion_failed`, `too_large`) SHALL not be
converted again for 10 minutes (`PREVIEW_FAILURE_TTL_MINUTES`) and answers the same meanwhile; an outage or a busy
answer is not remembered, so the next view tries again.

#### Scenario: Gotenberg is down
- **WHEN** a document is previewed while Gotenberg is stopped
- **THEN** the answer is `unavailable` / `conversion_unavailable` and the drive keeps serving everything else

#### Scenario: A document that cannot be converted
- **WHEN** a corrupt document is previewed twice within ten minutes
- **THEN** both answers are `conversion_failed` and Gotenberg was called once

#### Scenario: Converter saturated
- **WHEN** two conversions run and a third document is previewed and no slot frees in time
- **THEN** the answer is `unavailable` / `conversion_busy` and that document is not marked as failed

### Requirement: The conversion service is internal
Gotenberg SHALL be reachable only by the drive service. In production it sits on an internal network with no route
out, so a link inside an uploaded document cannot be fetched; it publishes no port, runs only its LibreOffice
route and is limited in memory, CPU and processes. Configuration: `GOTENBERG_URL` on the drive service. The drive
reaches the object store over the internal network when `PREVIEW_STORAGE_DIAL_ADDR` is set, with the public host
still sent as `Host` so the signed URL holds.

## Status
Native previews are signed like downloads, so the browser may be told to save rather than show some types
(the storage service does not yet pass a content-disposition override). The client part of this capability
(the viewer, including the table for `sheet`) is not yet written. The viewer contract: `native` text is fetched and
shown as text, never put in a frame.
