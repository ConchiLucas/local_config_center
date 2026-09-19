-- 仅用于本包的演示导航与只读配置；无真实密钥。
CREATE TABLE IF NOT EXISTS config_groups (id TEXT PRIMARY KEY, label TEXT NOT NULL, sort_order INTEGER NOT NULL, metadata_json TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS config_pages (id TEXT NOT NULL, group_id TEXT NOT NULL REFERENCES config_groups(id), sort_order INTEGER NOT NULL, metadata_json TEXT NOT NULL, PRIMARY KEY(group_id,id));
CREATE TABLE IF NOT EXISTS config_entries (id TEXT PRIMARY KEY, group_id TEXT NOT NULL, page_id TEXT NOT NULL, sort_order INTEGER NOT NULL, revision TEXT NOT NULL, public_json TEXT NOT NULL, FOREIGN KEY(group_id,page_id) REFERENCES config_pages(group_id,id));
