-- Migration 0002: append-only audit log for exam end-time extensions.
-- The Go server also runs GORM AutoMigrate at startup, so this file documents
-- the incremental schema change and can be applied manually if needed.

CREATE TABLE IF NOT EXISTS exam_extensions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL,
    operator_name VARCHAR(64) DEFAULT '',
    old_end_time DATETIME(3) NULL,
    new_end_time DATETIME(3) NULL,
    extend_minutes DOUBLE NOT NULL DEFAULT 0,
    affected_attempts INT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_exam_extensions_exam_id (exam_id),
    KEY idx_exam_extensions_operator_id (operator_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
