/**
 * Parser for Apple RoomPlan CapturedRoom JSON
 *
 * Converts CapturedRoom JSON export into ExtendedRoomPlanState for visualization.
 */

import type { CapturedRoomJSON, CapturedRoomObject } from "@/types/capturedRoom";
import type { ExtendedRoomPlanState, Point2D, Obstacle } from "@/types/worldstate";

/**
 * Parse a CapturedRoom JSON export into ExtendedRoomPlanState
 *
 * Extracts room dimensions from wall positions and converts
 * the CapturedRoom format for use in the 3D visualization.
 */
export function parseCapturedRoom(json: CapturedRoomJSON): ExtendedRoomPlanState {
  // Extract room bounds from walls or floor
  const bounds = calculateRoomBounds(json);

  // Convert floors to room polygon (2D footprint)
  const roomPolygon = extractRoomPolygon(json);

  // Convert objects to simple obstacles for backward compatibility
  const obstacles = convertObjectsToObstacles(json.objects);

  // Default sensor pose (identity at room center, 2.5m height)
  const sensorPose = createDefaultSensorPose(bounds.width, bounds.depth);

  return {
    // Base RoomPlanState fields (backward compatible)
    configured: true,
    roomWidth: bounds.width,
    roomHeight: bounds.depth, // Note: roomHeight in our model is actually depth (Z)
    sensorPose,
    roomPolygon,
    obstacles,

    // Extended CapturedRoom data
    capturedRoom: {
      identifier: json.identifier,
      surfaces: json.surfaces,
      objects: json.objects,
      floors: json.floors,
    },

    // Flag indicating CapturedRoom data is available
    hasCapturedRoom: true,
  };
}

/**
 * Calculate room bounding box from walls and floors
 */
interface RoomBounds {
  minX: number;
  maxX: number;
  minY: number; // Height
  maxY: number;
  minZ: number;
  maxZ: number;
  width: number; // X extent
  height: number; // Y extent (vertical)
  depth: number; // Z extent
}

function calculateRoomBounds(json: CapturedRoomJSON): RoomBounds {
  let minX = Infinity,
    maxX = -Infinity;
  let minY = Infinity,
    maxY = -Infinity;
  let minZ = Infinity,
    maxZ = -Infinity;

  // Get bounds from walls
  const walls = json.surfaces.filter(s => s.category === "wall");

  for (const wall of walls) {
    const [x, y, z] = getPositionFromTransform(wall.transform);
    const [w, h, d] = wall.dimensions;

    // Wall extends from center position by half dimensions
    // Account for wall rotation (approximate - assumes axis-aligned or 90° rotated)
    const yaw = getYRotationFromTransform(wall.transform);
    const isRotated90 = Math.abs(Math.abs(yaw) - Math.PI / 2) < 0.1;

    const halfW = isRotated90 ? d / 2 : w / 2;
    const halfD = isRotated90 ? w / 2 : d / 2;

    minX = Math.min(minX, x - halfW);
    maxX = Math.max(maxX, x + halfW);
    minY = Math.min(minY, y - h / 2);
    maxY = Math.max(maxY, y + h / 2);
    minZ = Math.min(minZ, z - halfD);
    maxZ = Math.max(maxZ, z + halfD);
  }

  // Also check floor polygons for more accurate XZ bounds
  if (json.floors) {
    for (const floor of json.floors) {
      for (const corner of floor.polygonCorners) {
        minX = Math.min(minX, corner.x);
        maxX = Math.max(maxX, corner.x);
        minZ = Math.min(minZ, corner.z);
        maxZ = Math.max(maxZ, corner.z);
      }
    }
  }

  // Handle case where no geometry found
  if (minX === Infinity) {
    return {
      minX: 0,
      maxX: 5,
      minY: 0,
      maxY: 2.8,
      minZ: 0,
      maxZ: 4,
      width: 5,
      height: 2.8,
      depth: 4,
    };
  }

  return {
    minX,
    maxX,
    minY,
    maxY,
    minZ,
    maxZ,
    width: maxX - minX,
    height: maxY - minY,
    depth: maxZ - minZ,
  };
}

/**
 * Extract room polygon (2D floor outline) from floors or walls
 */
function extractRoomPolygon(json: CapturedRoomJSON): Point2D[] {
  // Prefer floor polygon if available
  if (json.floors && json.floors.length > 0) {
    const mainFloor = json.floors[0];
    return mainFloor.polygonCorners.map(p => ({
      x: p.x,
      y: p.z, // Map 3D Z to 2D Y
    }));
  }

  // Fall back to wall corner extraction
  const walls = json.surfaces.filter(s => s.category === "wall");
  if (walls.length === 0) {
    return [];
  }

  // Extract wall endpoints and create convex hull
  const corners: Point2D[] = [];
  for (const wall of walls) {
    const [x, , z] = getPositionFromTransform(wall.transform);
    const [w] = wall.dimensions;
    const yaw = getYRotationFromTransform(wall.transform);

    // Calculate wall endpoints based on rotation
    const cos = Math.cos(yaw);
    const sin = Math.sin(yaw);
    const halfW = w / 2;

    // Add both endpoints of the wall
    corners.push({ x: x + cos * halfW, y: z + sin * halfW });
    corners.push({ x: x - cos * halfW, y: z - sin * halfW });
  }

  // Return simple convex hull (for now, just return corners)
  return simplifyPolygon(corners);
}

/**
 * Simplify polygon by removing duplicate/close points
 */
function simplifyPolygon(points: Point2D[], threshold = 0.1): Point2D[] {
  if (points.length === 0) return [];

  const result: Point2D[] = [points[0]];

  for (let i = 1; i < points.length; i++) {
    const last = result[result.length - 1];
    const dist = Math.sqrt((points[i].x - last.x) ** 2 + (points[i].y - last.y) ** 2);
    if (dist > threshold) {
      result.push(points[i]);
    }
  }

  return result;
}

/**
 * Convert CapturedRoom objects to simple Obstacle format for backward compatibility
 */
function convertObjectsToObstacles(objects: CapturedRoomObject[]): Obstacle[] {
  return objects.map(obj => {
    const [x, , z] = getPositionFromTransform(obj.transform);
    const [w, , d] = obj.dimensions;
    const yaw = getYRotationFromTransform(obj.transform);

    // Create bounding box polygon for the object
    const polygon = createRotatedRect(x, z, w, d, yaw);

    return {
      type: obj.category,
      polygon,
    };
  });
}

/**
 * Create a rotated rectangle polygon
 */
function createRotatedRect(
  cx: number,
  cz: number,
  width: number,
  depth: number,
  yaw: number,
): Point2D[] {
  const cos = Math.cos(yaw);
  const sin = Math.sin(yaw);
  const hw = width / 2;
  const hd = depth / 2;

  // Four corners of rectangle, rotated around center
  const corners = [
    { dx: -hw, dz: -hd },
    { dx: hw, dz: -hd },
    { dx: hw, dz: hd },
    { dx: -hw, dz: hd },
  ];

  return corners.map(({ dx, dz }) => ({
    x: cx + dx * cos - dz * sin,
    y: cz + dx * sin + dz * cos, // y in 2D = z in 3D
  }));
}

/**
 * Create default sensor pose at room center, looking down
 */
function createDefaultSensorPose(roomWidth: number, roomDepth: number): number[] {
  // Identity rotation, positioned at center of room, 2.5m height
  const x = roomWidth / 2;
  const y = 2.5; // Height
  const z = roomDepth / 2;

  // Column-major 4x4 identity matrix with translation
  return [
    1,
    0,
    0,
    0, // Column 0
    0,
    1,
    0,
    0, // Column 1
    0,
    0,
    1,
    0, // Column 2
    x,
    y,
    z,
    1, // Column 3 (translation)
  ];
}

/**
 * Extract position [x, y, z] from column-major 4x4 transform
 */
function getPositionFromTransform(transform: number[]): [number, number, number] {
  return [transform[12], transform[13], transform[14]];
}

/**
 * Extract Y-axis rotation (yaw) from transform matrix
 */
function getYRotationFromTransform(transform: number[]): number {
  // For Y-axis rotation, use atan2 of rotation matrix elements
  // R = [cos(y) 0 sin(y); 0 1 0; -sin(y) 0 cos(y)]
  // In column-major: r00=transform[0], r20=transform[2], r02=transform[8], r22=transform[10]
  return Math.atan2(transform[8], transform[0]);
}

/**
 * Validate CapturedRoom JSON structure
 */
export function validateCapturedRoom(
  json: unknown,
): { valid: true; data: CapturedRoomJSON } | { valid: false; error: string } {
  if (!json || typeof json !== "object") {
    return { valid: false, error: "Invalid JSON: not an object" };
  }

  const obj = json as Record<string, unknown>;

  if (typeof obj.identifier !== "string") {
    return { valid: false, error: "Missing or invalid identifier" };
  }

  if (!Array.isArray(obj.surfaces)) {
    return { valid: false, error: "Missing or invalid surfaces array" };
  }

  if (!Array.isArray(obj.objects)) {
    return { valid: false, error: "Missing or invalid objects array" };
  }

  // Validate surfaces
  for (let i = 0; i < obj.surfaces.length; i++) {
    const surface = obj.surfaces[i] as Record<string, unknown>;
    if (!surface.identifier || !surface.category || !surface.dimensions || !surface.transform) {
      return { valid: false, error: `Invalid surface at index ${i}` };
    }
  }

  // Validate objects
  for (let i = 0; i < obj.objects.length; i++) {
    const object = obj.objects[i] as Record<string, unknown>;
    if (!object.identifier || !object.category || !object.dimensions || !object.transform) {
      return { valid: false, error: `Invalid object at index ${i}` };
    }
  }

  return { valid: true, data: json as CapturedRoomJSON };
}
