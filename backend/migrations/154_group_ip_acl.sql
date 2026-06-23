-- Add group-level IP ACL fields so gateway requests can be restricted by group.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS ip_whitelist JSONB DEFAULT NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS ip_blacklist JSONB DEFAULT NULL;

COMMENT ON COLUMN groups.ip_whitelist IS 'JSON array of allowed IPs/CIDRs for this group, e.g. ["10.0.0.0/8", "203.0.113.10"]';
COMMENT ON COLUMN groups.ip_blacklist IS 'JSON array of blocked IPs/CIDRs for this group, e.g. ["1.2.3.4", "198.51.100.0/24"]';
