-- Full-text search over posts.
--
-- Runs after 20260916094300_schema.sql, which drops and recreates every table,
-- so posts exists here but has no search_vector yet. Both the title and the
-- content are indexed; the title carries weight A and the content weight B, which
-- ts_rank scores 1.0 and 0.4 respectively, so a title match outranks a content
-- match by 2.5x.
--
-- The configuration is 'english' (stemming + stopword removal): a search for
-- "template" matches "templates". Vietnamese is deliberately not handled, so no
-- unaccent extension is involved and ts_headline returns the original text.
ALTER TABLE posts
ADD COLUMN IF NOT EXISTS search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;

CREATE INDEX IF NOT EXISTS idx_posts_search_vector ON posts USING GIN (search_vector);
