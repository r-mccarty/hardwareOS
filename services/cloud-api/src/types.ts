/**
 * OpticWorks Cloud API Types
 * Based on docs/CLOUD_API.md specification
 */

// Environment bindings for Cloudflare Worker
export interface Env {
  DB: D1Database;
  ENVIRONMENT: string;
  // FIRMWARE_BUCKET?: R2Bucket;
}

// Database types
export interface Device {
  id: string;
  serial_number: string;
  product_type: string;
  firmware_version: string | null;
  customer_id: string | null;
  order_id: string | null;
  cloud_token: string | null;
  temp_token: string | null;
  last_seen_at: string | null;
  registered_at: string | null;
  paired_at: string | null;
  metadata: string | null; // JSON string
  created_at: string;
  updated_at: string;
}

export interface DeviceTelemetry {
  id: number;
  device_id: string;
  timestamp: string;
  metrics: string; // JSON string
  events: string | null; // JSON string
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  name: string | null;
  google_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface Session {
  id: string;
  user_id: string;
  token_hash: string;
  expires_at: string;
  created_at: string;
}

// API Request/Response types
export interface RegisterDeviceRequest {
  serial_number: string;
  firmware_version?: string;
  product_type?: string;
}

export interface RegisterDeviceResponse {
  temp_token: string;
  device_id: string;
  status: 'awaiting_pairing';
}

export interface TokenExchangeRequest {
  temp_token: string;
}

export interface TokenExchangeResponse {
  cloud_token: string;
  device_id: string;
  customer_id: string;
}

export interface OTACheckResponse {
  update_available: boolean;
  version?: string;
  url?: string;
  sha256?: string;
  size_bytes?: number;
  release_notes?: string;
  mandatory?: boolean;
  min_version?: string;
  current_version?: string;
}

export interface TelemetryRequest {
  timestamp: string;
  metrics: Record<string, number | string>;
  events?: Array<{
    type: string;
    timestamp: string;
    data?: Record<string, unknown>;
  }>;
}

export interface MeResponse {
  id: string;
  email: string;
  name: string | null;
  devices: Array<{
    id: string;
    serial_number: string;
    product_type: string;
    firmware_version: string | null;
    last_seen_at: string | null;
  }>;
}

export interface ErrorResponse {
  error: string;
  message: string;
}

// Device status enum
export type DeviceStatus = 'awaiting_pairing' | 'paired' | 'offline';
