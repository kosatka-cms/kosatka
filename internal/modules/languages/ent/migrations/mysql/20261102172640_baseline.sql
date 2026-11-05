-- Create "language_settings" table
CREATE TABLE `language_settings` (`id` int unsigned NOT NULL AUTO_INCREMENT, `translatable_types` json NULL, `untranslated` varchar(16) NOT NULL DEFAULT 'original', PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- Create "language_visibility" table
CREATE TABLE `language_visibility` (`id` int unsigned NOT NULL AUTO_INCREMENT, `kind` varchar(64) NOT NULL, `ref` varchar(64) NOT NULL, `langs` json NOT NULL, PRIMARY KEY (`id`), UNIQUE INDEX `language_visibility_kind_ref` (`kind`, `ref`)) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- Create "text_translations" table
CREATE TABLE `text_translations` (`id` int unsigned NOT NULL AUTO_INCREMENT, `kind` varchar(64) NOT NULL, `ref` varchar(64) NOT NULL, `lang` varchar(12) NOT NULL, `value` text NOT NULL, PRIMARY KEY (`id`), UNIQUE INDEX `text_translations_kind_ref_lang` (`kind`, `ref`, `lang`)) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- Create "node_translations" table
CREATE TABLE `node_translations` (`id` int unsigned NOT NULL AUTO_INCREMENT, `tnid` int unsigned NOT NULL, `outdated` bool NOT NULL DEFAULT false, `nid` int unsigned NOT NULL, PRIMARY KEY (`id`), UNIQUE INDEX `node_translations_nid` (`nid`), INDEX `node_translations_tnid` (`tnid`), CONSTRAINT `node_translations_nodes_node` FOREIGN KEY (`nid`) REFERENCES `nodes` (`id`) ON DELETE CASCADE) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
