PRAGMA foreign_keys = ON;

-- Application users.
CREATE TABLE users (
    id integer PRIMARY KEY AUTOINCREMENT,
    email text NOT NULL UNIQUE COLLATE NOCASE,
    display_name text,
    search_name text GENERATED ALWAYS AS (lower(email)) VIRTUAL,
    created_at text NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

CREATE TABLE posts (
    id integer PRIMARY KEY AUTOINCREMENT,
    author_id integer NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title text NOT NULL,
    status text NOT NULL DEFAULT 'draft',
    created_at text NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT posts_status_check CHECK (status IN ('draft', 'published', 'deleted'))
);

CREATE TABLE tags (
    id integer PRIMARY KEY,
    name text NOT NULL
) WITHOUT ROWID;

CREATE TABLE post_tags (
    post_id integer NOT NULL,
    tag_id integer NOT NULL,
    CONSTRAINT post_tags_pk PRIMARY KEY (post_id, tag_id),
    CONSTRAINT post_tags_post_fk FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE,
    CONSTRAINT post_tags_tag_fk FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX posts_author_created_idx ON posts (author_id, created_at DESC) WHERE status <> 'deleted';

CREATE UNIQUE INDEX posts_title_unique ON posts (title COLLATE NOCASE);

-- Posts that are published.
CREATE VIEW active_posts AS SELECT id, title FROM posts WHERE status = 'published';

CREATE TRIGGER posts_touch_created
    AFTER UPDATE OF title, status ON posts
    FOR EACH ROW
    WHEN old.status <> new.status
BEGIN
    UPDATE posts SET created_at = CURRENT_TIMESTAMP WHERE id = new.id;
END;

ANALYZE;
