-- Modify "view_displays" table
ALTER TABLE "view_displays" ADD COLUMN "skip_items" bigint NOT NULL DEFAULT 0;
