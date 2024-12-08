ALTER TABLE `chairs` ADD INDEX `access_token` (`access_token`);
ALTER TABLE `ride_statuses` ADD INDEX `idx_ride_statuses_ride_id_created_at` (`ride_id`, `created_at` DESC);
ALTER TABLE `chair_locations` ADD INDEX `idx_chair_locations_chair_id_created_at` (`chair_id`, `created_at`);
