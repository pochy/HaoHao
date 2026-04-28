-- name: CreateDriveOCRRun :one
INSERT INTO drive_ocr_runs (
    tenant_id,
    file_object_id,
    file_revision,
    content_sha256,
    engine,
    languages,
    structured_extractor,
    status,
    reason,
    requested_by_user_id,
    outbox_event_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(file_object_id),
    sqlc.arg(file_revision),
    sqlc.arg(content_sha256),
    sqlc.arg(engine),
    sqlc.arg(languages),
    sqlc.arg(structured_extractor),
    'pending',
    sqlc.arg(reason),
    sqlc.narg(requested_by_user_id),
    sqlc.narg(outbox_event_id)
)
ON CONFLICT (file_object_id, file_revision, content_sha256, engine, structured_extractor) DO UPDATE
SET
    status = CASE
        WHEN drive_ocr_runs.status IN ('failed', 'skipped') THEN 'pending'
        ELSE drive_ocr_runs.status
    END,
    reason = EXCLUDED.reason,
    requested_by_user_id = COALESCE(EXCLUDED.requested_by_user_id, drive_ocr_runs.requested_by_user_id),
    outbox_event_id = COALESCE(EXCLUDED.outbox_event_id, drive_ocr_runs.outbox_event_id),
    error_code = CASE
        WHEN drive_ocr_runs.status IN ('failed', 'skipped') THEN NULL
        ELSE drive_ocr_runs.error_code
    END,
    error_message = CASE
        WHEN drive_ocr_runs.status IN ('failed', 'skipped') THEN NULL
        ELSE drive_ocr_runs.error_message
    END,
    updated_at = now()
RETURNING *;

-- name: LinkDriveOCRRunOutboxEvent :one
UPDATE drive_ocr_runs
SET outbox_event_id = sqlc.arg(outbox_event_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: GetDriveOCRRunByPublicID :one
SELECT *
FROM drive_ocr_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id);

-- name: GetLatestDriveOCRRunForFile :one
SELECT *
FROM drive_ocr_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND file_object_id = sqlc.arg(file_object_id)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListDriveOCRPages :many
SELECT *
FROM drive_ocr_pages
WHERE tenant_id = sqlc.arg(tenant_id)
  AND ocr_run_id = sqlc.arg(ocr_run_id)
ORDER BY page_number ASC;

-- name: ListDriveProductExtractionItems :many
SELECT *
FROM drive_product_extraction_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND file_object_id = sqlc.arg(file_object_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: MarkDriveOCRRunRunning :one
UPDATE drive_ocr_runs
SET status = 'running',
    started_at = COALESCE(started_at, now()),
    error_code = NULL,
    error_message = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: MarkDriveOCRRunCompleted :one
UPDATE drive_ocr_runs
SET status = 'completed',
    page_count = sqlc.arg(page_count),
    processed_page_count = sqlc.arg(processed_page_count),
    average_confidence = sqlc.narg(average_confidence),
    extracted_text = sqlc.arg(extracted_text),
    error_code = NULL,
    error_message = NULL,
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: MarkDriveOCRRunFailed :one
UPDATE drive_ocr_runs
SET status = 'failed',
    error_code = sqlc.arg(error_code),
    error_message = left(sqlc.arg(error_message), 2000),
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: MarkDriveOCRRunSkipped :one
UPDATE drive_ocr_runs
SET status = 'skipped',
    error_code = sqlc.arg(error_code),
    error_message = left(sqlc.arg(error_message), 2000),
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: DeleteDriveOCRPagesForRun :exec
DELETE FROM drive_ocr_pages
WHERE tenant_id = sqlc.arg(tenant_id)
  AND ocr_run_id = sqlc.arg(ocr_run_id);

-- name: UpsertDriveOCRPage :one
INSERT INTO drive_ocr_pages (
    tenant_id,
    ocr_run_id,
    file_object_id,
    page_number,
    raw_text,
    average_confidence,
    layout_json,
    boxes_json
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(ocr_run_id),
    sqlc.arg(file_object_id),
    sqlc.arg(page_number),
    sqlc.arg(raw_text),
    sqlc.narg(average_confidence),
    sqlc.arg(layout_json),
    sqlc.arg(boxes_json)
)
ON CONFLICT (ocr_run_id, page_number) DO UPDATE
SET raw_text = EXCLUDED.raw_text,
    average_confidence = EXCLUDED.average_confidence,
    layout_json = EXCLUDED.layout_json,
    boxes_json = EXCLUDED.boxes_json
RETURNING *;

-- name: DeleteDriveProductExtractionItemsForRun :exec
DELETE FROM drive_product_extraction_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND ocr_run_id = sqlc.arg(ocr_run_id);

-- name: CreateDriveProductExtractionItem :one
INSERT INTO drive_product_extraction_items (
    tenant_id,
    ocr_run_id,
    file_object_id,
    item_type,
    name,
    brand,
    manufacturer,
    model,
    sku,
    jan_code,
    category,
    description,
    price,
    promotion,
    availability,
    source_text,
    evidence,
    attributes,
    confidence
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(ocr_run_id),
    sqlc.arg(file_object_id),
    sqlc.arg(item_type),
    sqlc.arg(name),
    sqlc.narg(brand),
    sqlc.narg(manufacturer),
    sqlc.narg(model),
    sqlc.narg(sku),
    sqlc.narg(jan_code),
    sqlc.narg(category),
    sqlc.narg(description),
    sqlc.arg(price),
    sqlc.arg(promotion),
    sqlc.arg(availability),
    sqlc.arg(source_text),
    sqlc.arg(evidence),
    sqlc.arg(attributes),
    sqlc.narg(confidence)
)
RETURNING *;

-- name: CountDriveOCRRunsByStatus :many
SELECT status, count(*)::bigint AS count
FROM drive_ocr_runs
WHERE tenant_id = sqlc.arg(tenant_id)
GROUP BY status
ORDER BY status;
