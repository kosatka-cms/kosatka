-- Create "mail_transport_settings" table
CREATE TABLE `mail_transport_settings` (`id` int unsigned NOT NULL AUTO_INCREMENT, `transport` varchar(16) NOT NULL, `name` varchar(64) NOT NULL, `value` text NOT NULL, PRIMARY KEY (`id`), UNIQUE INDEX `uniq_mail_transport_settings_name` (`transport`, `name`)) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
