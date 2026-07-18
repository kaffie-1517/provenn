-- Initial schema for the ProveNN invoice verification platform.
-- Based on docs/LLD.md section 2.

CREATE TABLE companies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    plan        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partners (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    api_key_hash  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('provider', 'employee', 'company_admin', 'platform_admin')),
    company_id    UUID REFERENCES companies(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id        UUID REFERENCES partners(id),
    provider_user_id  UUID REFERENCES users(id),
    reference_code    TEXT NOT NULL UNIQUE,
    purchase_ref      TEXT,
    amount_cents      INTEGER NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'INR',
    vendor_name       TEXT NOT NULL,
    invoice_date      DATE NOT NULL,
    status            TEXT NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'ready')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoice_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id      UUID NOT NULL REFERENCES invoices(id),
    version_number  INTEGER NOT NULL,
    sha256_hash     TEXT NOT NULL,
    storage_key     TEXT NOT NULL,
    uploaded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    uploaded_by     UUID REFERENCES users(id)
);

CREATE TABLE billing_events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id       UUID NOT NULL REFERENCES invoices(id),
    idempotency_key  TEXT NOT NULL UNIQUE,
    amount_cents     INTEGER NOT NULL,
    billed_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE verifications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id          UUID REFERENCES invoices(id),
    company_id          UUID NOT NULL REFERENCES companies(id),
    submitted_by        UUID NOT NULL REFERENCES users(id),
    submitted_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_hash      TEXT NOT NULL,
    matched_version_id  UUID REFERENCES invoice_versions(id),
    result              TEXT NOT NULL CHECK (result IN ('match', 'mismatch', 'not_found')),
    approval_status     TEXT NOT NULL DEFAULT 'pending' CHECK (approval_status IN ('pending', 'approved', 'rejected')),
    approved_by         UUID REFERENCES users(id),
    approved_at         TIMESTAMPTZ
);

-- Indexes (LLD §2).
-- invoices.reference_code and billing_events.idempotency_key already have
-- unique indexes from their UNIQUE constraints. The composite indexes on
-- verifications are the ones that need explicit creation.
CREATE INDEX idx_verifications_company_submitted ON verifications(company_id, submitted_at);
CREATE INDEX idx_verifications_company_approval  ON verifications(company_id, approval_status);
