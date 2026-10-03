PRAGMA foreign_keys = ON;

-- Application users.
CREATE TABLE IF NOT EXISTS users (
    id integer PRIMARY KEY AUTOINCREMENT,
    email text NOT NULL UNIQUE COLLATE NOCASE,
    display_name text,
    search_name text GENERATED ALWAYS AS (lower(email)) VIRTUAL,
    created_at text NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at text NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

CREATE TABLE posts (
    id integer PRIMARY KEY AUTOINCREMENT,
    author_id integer NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    slug text NOT NULL,
    title text NOT NULL,
    body text,
    status text NOT NULL DEFAULT 'draft',
    view_count integer NOT NULL DEFAULT 0,
    published_at text,
    created_at text NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT posts_status_check CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE TABLE tags (
    id integer PRIMARY KEY,
    name text NOT NULL UNIQUE COLLATE NOCASE
) WITHOUT ROWID;

CREATE TABLE post_tags (
    post_id integer NOT NULL,
    tag_id integer NOT NULL,
    CONSTRAINT post_tags_pk PRIMARY KEY (post_id, tag_id),
    CONSTRAINT post_tags_post_fk FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE,
    CONSTRAINT post_tags_tag_fk FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX IF NOT EXISTS posts_author_published_idx ON posts (author_id, published_at DESC) WHERE status = 'published';

CREATE UNIQUE INDEX IF NOT EXISTS posts_slug_unique ON posts (slug COLLATE NOCASE);

-- Posts visible to readers.
CREATE VIEW IF NOT EXISTS published_posts (id, author_id, slug, title, published_at) AS SELECT id, author_id, slug, title, published_at FROM posts WHERE status = 'published';

-- Keeps updated_at current on user edits.
CREATE TRIGGER users_touch_updated_at
    AFTER UPDATE OF email, display_name ON users
    FOR EACH ROW
    WHEN old.updated_at = new.updated_at
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = new.id;
END;
