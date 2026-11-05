-- Create "hello_greetings" table
CREATE TABLE `hello_greetings` (`id` int NOT NULL AUTO_INCREMENT, `text` varchar(255) NOT NULL, `created_at` datetime NOT NULL DEFAULT (CURRENT_TIMESTAMP), PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
