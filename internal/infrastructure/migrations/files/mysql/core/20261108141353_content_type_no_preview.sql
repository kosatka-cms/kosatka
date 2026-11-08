-- Modify "content_types" table
ALTER TABLE `content_types` ADD COLUMN `no_preview` bool NOT NULL DEFAULT false;
