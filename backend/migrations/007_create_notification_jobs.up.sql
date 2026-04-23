-- backend/migrations/007_create_notification_jobs.up.sql
CREATE TABLE notification_jobs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id  UUID        REFERENCES properties(id) ON DELETE SET NULL,
    job_type     TEXT        NOT NULL,             -- STALE_LISTING | FOLLOW_UP | DEAL_ALERT
    status       TEXT        NOT NULL DEFAULT 'PENDING', -- PENDING | SENT | FAILED
    twilio_sid   TEXT,
    scheduled_at TIMESTAMPTZ,
    sent_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_jobs_property_id ON notification_jobs (property_id);
CREATE INDEX idx_notification_jobs_status      ON notification_jobs (status);
CREATE INDEX idx_notification_jobs_scheduled   ON notification_jobs (scheduled_at) WHERE status = 'PENDING';
