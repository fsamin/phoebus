-- Add slug columns to learning_paths, modules, and steps

ALTER TABLE learning_paths ADD COLUMN slug TEXT;
ALTER TABLE modules ADD COLUMN slug TEXT;
ALTER TABLE steps ADD COLUMN slug TEXT;

-- Backfill slugs from titles: lowercase, replace non-alphanum with hyphens, collapse, trim
UPDATE learning_paths SET slug = regexp_replace(
    regexp_replace(
        regexp_replace(lower(title), '[^a-z0-9]+', '-', 'g'),
        '^-|-$', '', 'g'
    ),
    '-{2,}', '-', 'g'
) WHERE slug IS NULL;

UPDATE modules SET slug = regexp_replace(
    regexp_replace(
        regexp_replace(lower(title), '[^a-z0-9]+', '-', 'g'),
        '^-|-$', '', 'g'
    ),
    '-{2,}', '-', 'g'
) WHERE slug IS NULL;

UPDATE steps SET slug = regexp_replace(
    regexp_replace(
        regexp_replace(lower(title), '[^a-z0-9]+', '-', 'g'),
        '^-|-$', '', 'g'
    ),
    '-{2,}', '-', 'g'
) WHERE slug IS NULL AND deleted_at IS NULL;

-- Handle duplicate slugs for learning_paths by appending numeric suffix
DO $$
DECLARE
    r RECORD;
    n INT;
BEGIN
    FOR r IN
        SELECT id, slug, ROW_NUMBER() OVER (PARTITION BY slug ORDER BY created_at) AS rn
        FROM learning_paths
        WHERE deleted_at IS NULL
    LOOP
        IF r.rn > 1 THEN
            UPDATE learning_paths SET slug = r.slug || '-' || r.rn WHERE id = r.id;
        END IF;
    END LOOP;
END $$;

-- Handle duplicate slugs for modules within same learning_path
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT id, slug, learning_path_id,
               ROW_NUMBER() OVER (PARTITION BY learning_path_id, slug ORDER BY position) AS rn
        FROM modules
    LOOP
        IF r.rn > 1 THEN
            UPDATE modules SET slug = r.slug || '-' || r.rn WHERE id = r.id;
        END IF;
    END LOOP;
END $$;

-- Handle duplicate slugs for steps within same module
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT id, slug, module_id,
               ROW_NUMBER() OVER (PARTITION BY module_id, slug ORDER BY position) AS rn
        FROM steps
        WHERE deleted_at IS NULL
    LOOP
        IF r.rn > 1 THEN
            UPDATE steps SET slug = r.slug || '-' || r.rn WHERE id = r.id;
        END IF;
    END LOOP;
END $$;

-- Now make slug NOT NULL
ALTER TABLE learning_paths ALTER COLUMN slug SET NOT NULL;
ALTER TABLE modules ALTER COLUMN slug SET NOT NULL;
ALTER TABLE steps ALTER COLUMN slug SET DEFAULT '';

-- Create unique indexes
CREATE UNIQUE INDEX idx_learning_paths_slug ON learning_paths(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_modules_slug ON modules(learning_path_id, slug);
CREATE UNIQUE INDEX idx_steps_slug ON steps(module_id, slug) WHERE deleted_at IS NULL;
