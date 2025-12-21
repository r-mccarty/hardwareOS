-- OpticWorks Cloud API Database Schema
-- Based on docs/CLOUD_API.md specification

-- Users table (for cloud frontend authentication)
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  name TEXT,
  google_id TEXT UNIQUE,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

-- User sessions table
CREATE TABLE IF NOT EXISTS user_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);

-- Device table (devices registered with the cloud)
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  serial_number TEXT UNIQUE NOT NULL,
  product_type TEXT DEFAULT 'rs1',
  firmware_version TEXT,
  customer_id TEXT REFERENCES users(id),
  order_id TEXT,
  cloud_token TEXT,
  temp_token TEXT,
  last_seen_at TEXT,
  registered_at TEXT,
  paired_at TEXT,
  metadata TEXT, -- JSON
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_devices_serial ON devices(serial_number);
CREATE INDEX IF NOT EXISTS idx_devices_customer ON devices(customer_id);
CREATE INDEX IF NOT EXISTS idx_devices_cloud_token ON devices(cloud_token);
CREATE INDEX IF NOT EXISTS idx_devices_temp_token ON devices(temp_token);

-- Device telemetry table
CREATE TABLE IF NOT EXISTS device_telemetry (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL REFERENCES devices(id),
  timestamp TEXT NOT NULL,
  metrics TEXT NOT NULL, -- JSON
  events TEXT, -- JSON
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_telemetry_device ON device_telemetry(device_id);
CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON device_telemetry(timestamp);

-- Device OTA updates tracking
CREATE TABLE IF NOT EXISTS device_ota_updates (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL REFERENCES devices(id),
  from_version TEXT NOT NULL,
  to_version TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  status TEXT DEFAULT 'pending', -- pending, downloading, applying, completed, failed
  error_message TEXT,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ota_device ON device_ota_updates(device_id);
CREATE INDEX IF NOT EXISTS idx_ota_status ON device_ota_updates(status);

-- Firmware releases table (for OTA)
CREATE TABLE IF NOT EXISTS firmware_releases (
  id TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  product_type TEXT DEFAULT 'rs1',
  url TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  release_notes TEXT,
  mandatory INTEGER DEFAULT 0,
  min_version TEXT,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_releases_version ON firmware_releases(version);
CREATE INDEX IF NOT EXISTS idx_releases_product ON firmware_releases(product_type);

-- Insert a demo user for development
INSERT OR IGNORE INTO users (id, email, name)
VALUES ('usr_demo', 'demo@optic.works', 'Demo User');

-- Insert a demo device for development (pre-registered, awaiting pairing)
INSERT OR IGNORE INTO devices (id, serial_number, product_type, registered_at, temp_token)
VALUES ('dev_demo', 'RS1-DEMO001', 'rs1', datetime('now'), 'demo_temp_token_12345');
