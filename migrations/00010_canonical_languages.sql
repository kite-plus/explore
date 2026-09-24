-- +goose Up
-- Language tags used to be stored as feeds declared them, so zh_CN never
-- matched ?lang=zh. Rewrite them with hyphens and BCP 47 letter case, and
-- blank ones as und. A value that is no tag at all, such as English, is only
-- lower-cased here, where normalize.Language would make it und.

-- The first subtag is the language; until a singleton starts an extension,
-- four letters are a script and two letters a region.
-- +goose StatementBegin
CREATE FUNCTION canonical_language(tag text) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
    SELECT coalesce(string_agg(CASE
               WHEN pos = 1 OR extension THEN lower(subtag)
               WHEN subtag ~ '^[A-Za-z]{4}$' THEN initcap(subtag)
               WHEN subtag ~ '^[A-Za-z]{2}$' THEN upper(subtag)
               ELSE lower(subtag)
           END, '-' ORDER BY pos), 'und')
    FROM (
        SELECT subtag, pos, bool_or(length(subtag) = 1) OVER (ORDER BY pos) AS extension
        FROM unnest(string_to_array(replace(btrim(tag), '_', '-'), '-')) WITH ORDINALITY AS s (subtag, pos)
    ) AS subtags
$$;
-- +goose StatementEnd

UPDATE blogs SET language = canonical_language(language)
WHERE language <> canonical_language(language);

-- A pending submission's report supplies the language when it is approved.
UPDATE submissions
SET check_report = jsonb_set(check_report, '{language}', to_jsonb(canonical_language(check_report->>'language')))
WHERE status = 'pending' AND check_report->>'language' <> canonical_language(check_report->>'language');

DROP FUNCTION canonical_language(text);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
