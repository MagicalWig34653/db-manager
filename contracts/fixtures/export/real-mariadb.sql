-- DB Manager SQL export 1; same-dialect tables only
-- Snapshot: per database; no server-wide snapshot. See EXPORT-FORMAT.
SET NAMES utf8mb4;
SET time_zone='+00:00';
CREATE TABLE `ex"`` port';--` (
  `id` bigint(20) DEFAULT NULL,
  `text_value` text DEFAULT NULL,
  `empty_value` text DEFAULT NULL,
  `nil_value` text DEFAULT NULL,
  `bytes_value` blob DEFAULT NULL,
  `exact_value` decimal(40,20) DEFAULT NULL,
  `utc_value` timestamp(6) NULL DEFAULT NULL,
  KEY `export index` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
INSERT INTO `ex"`` port';--` (`id`,`text_value`,`empty_value`,`nil_value`,`bytes_value`,`exact_value`,`utc_value`) VALUES (9007199254740993,CONVERT(X'4772c3bcc39f652027e99baa2c206c696e650a3d312b31' USING utf8mb4),CONVERT(X'' USING utf8mb4),NULL,X'00ff27225c',12345678901234567890.12345678901234567890,'2026-01-02 03:04:05.123456');
-- DBMANAGER EXPORT COMPLETE
