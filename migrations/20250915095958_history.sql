-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS public.history
(
    id BIGSERIAL NOT NULL,
    user_id bigint NOT NULL,
    secure_data_id bigint NOT NULL,
    method VARCHAR(25) COLLATE pg_catalog."default" NOT NULL,
    CONSTRAINT history_pkey PRIMARY KEY (id)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.history
    OWNER to postgres;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS public.history;
-- +goose StatementEnd
