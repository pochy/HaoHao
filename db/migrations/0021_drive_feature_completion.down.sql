DROP TABLE IF EXISTS drive_item_tags;
DROP TABLE IF EXISTS drive_file_previews;
DROP TABLE IF EXISTS drive_item_activities;
DROP TABLE IF EXISTS drive_starred_items;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS description;

ALTER TABLE drive_folders
    DROP COLUMN IF EXISTS description;
