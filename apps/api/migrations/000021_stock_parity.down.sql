ALTER TABLE `stock_transfers`
  DROP COLUMN `transfer_date`,
  DROP COLUMN `vehicle_plate`,
  DROP COLUMN `driver_name`;
ALTER TABLE `stock_locations`
  DROP COLUMN `sort_order`,
  DROP COLUMN `is_primary`,
  DROP COLUMN `is_picking_area`;
