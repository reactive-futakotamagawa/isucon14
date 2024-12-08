ALTER TABLE `chairs` ADD INDEX `access_token` (`access_token`);
ALTER TABLE `ride_statuses` ADD INDEX `idx_ride_statuses_ride_id_created_at` (`ride_id`, `created_at` DESC);
ALTER TABLE `chair_locations` ADD INDEX `idx_chair_locations_chair_id_created_at` (`chair_id`, `created_at`);
ALTER TABLE `chairs` ADD INDEX `idx_chairs_owner_id` (`owner_id`);
ALTER TABLE `rides` ADD INDEX `idx_rides_chair_id_updated_at` (`chair_id`, `updated_at` DESC);
ALTER TABLE `rides` ADD INDEX `idx_rides_user_id_created_at` (`user_id`, `created_at` DESC);
ALTER TABLE `coupons` ADD INDEX `idx_coupons_used_by` (`used_by`);
