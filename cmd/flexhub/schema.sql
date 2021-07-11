DROP DATABASE IF EXISTS `flex`;
CREATE DATABASE `flex`;
USE `flex`;

CREATE TABLE `tasks` (
    `id` BIGINT(20) PRIMARY KEY AUTO_INCREMENT,
    `priority` INT(10) NOT NULL,
    `state` ENUM('PENDING', 'RUNNING', 'FINISHED') NOT NULL DEFAULT 'PENDING',
    `worker` VARCHAR(128) NULL,
    `queued` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `started` TIMESTAMP NULL,
    `finished` TIMESTAMP NULL,
    `request` MEDIUMBLOB NOT NULL,
    `response` MEDIUMBLOB NULL
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;

CREATE INDEX `tasks_queue` ON `tasks` (`state`, `priority` DESC, `id` ASC);
