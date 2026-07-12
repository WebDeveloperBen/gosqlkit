package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

// EnableForeignKeys turns on foreign-key enforcement, which SQLite leaves off by
// default. It renders before the structured schema.
var EnableForeignKeys = sqlite.RawSQL("enable_foreign_keys", "PRAGMA foreign_keys = ON;").
	Before().
	Down("PRAGMA foreign_keys = OFF;")

var Users = sqlite.Table(
	"users",
	sqlite.Integer("id").PrimaryKey().AutoIncrement(),
	sqlite.Text("email").NotNull().Unique().Collate(sqlite.NoCase),
	sqlite.Text("display_name"),
	sqlite.Text("search_name").GeneratedAlwaysAsVirtual("lower(email)"),
	sqlite.Text("created_at").NotNull().DefaultCurrentTimestamp(),
	sqlite.Text("updated_at").NotNull().DefaultCurrentTimestamp(),
).
	Strict().
	Comment("Application users.")

var UsersTouchUpdatedAt = sqlite.Trigger("users_touch_updated_at", "users").
	After().
	UpdateOf("email", "display_name").
	ForEachRow().
	When("old.updated_at = new.updated_at").
	Body("UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = new.id;").
	Comment("Keeps updated_at current on user edits.")

var Posts = sqlite.Table(
	"posts",
	sqlite.Integer("id").PrimaryKey().AutoIncrement(),
	sqlite.Integer("author_id").NotNull().References("users", "id").OnDelete(sqlite.Cascade),
	sqlite.Text("slug").NotNull(),
	sqlite.Text("title").NotNull(),
	sqlite.Text("body"),
	sqlite.Text("status").NotNull().DefaultString("draft"),
	sqlite.Integer("view_count").NotNull().DefaultInt(0),
	sqlite.Text("published_at"),
	sqlite.Text("created_at").NotNull().DefaultCurrentTimestamp(),
	sqlite.Check("posts_status_check", "status IN ('draft', 'published', 'archived')"),
	sqlite.UniqueIndexOn("posts_slug_unique", sqlite.IndexColumn("slug").Collate(sqlite.NoCase)),
	sqlite.IndexOn(
		"posts_author_published_idx",
		sqlite.IndexColumn("author_id"),
		sqlite.IndexColumn("published_at").Desc(),
	).Where("status = 'published'"),
)

var Tags = sqlite.Table(
	"tags",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Text("name").NotNull().Unique().Collate(sqlite.NoCase),
).
	WithoutRowID()

var PostTags = sqlite.Table(
	"post_tags",
	sqlite.Integer("post_id").NotNull(),
	sqlite.Integer("tag_id").NotNull(),
	sqlite.PrimaryKey("post_tags_pk", "post_id", "tag_id"),
	sqlite.ForeignKey("post_tags_post_fk", "post_id").
		References("posts", "id").
		OnDelete(sqlite.Cascade),
	sqlite.ForeignKey("post_tags_tag_fk", "tag_id").
		References("tags", "id").
		OnDelete(sqlite.Cascade).
		InitiallyDeferred(),
)

var PublishedPosts = sqlite.View("published_posts").
	As("SELECT id, author_id, slug, title, published_at FROM posts WHERE status = 'published'").
	Columns("id", "author_id", "slug", "title", "published_at").
	DependsOn("posts").
	Comment("Posts visible to readers.")
