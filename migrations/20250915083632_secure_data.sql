-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS public.secure_data
(
    id BIGSERIAL NOT NULL,
    user_id bigint NOT NULL,
    data TEXT NOT NULL,
    metadata jsonb NOT NULL,
    CONSTRAINT secure_data_pkey PRIMARY KEY (id),
    history_id bigint,
    is_active BOOLEAN
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.secure_data
    OWNER to postgres;

CREATE INDEX IF NOT EXISTS idx_secure_data_gin_data
    ON public.secure_data USING gin
    (metadata)
    TABLESPACE pg_default;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS public.secure_data;
-- +goose StatementEnd
