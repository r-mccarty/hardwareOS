/**
 * Apple RoomPlan CapturedRoom TypeScript Types
 *
 * These types match the JSON export format from Apple's RoomPlan framework.
 * CapturedRoom is exported from iPhone/iPad LiDAR room scans.
 *
 * @see https://developer.apple.com/documentation/roomplan/capturedroom
 */

// -----------------------------------------------------------------------------
// Confidence Level
// -----------------------------------------------------------------------------

export type ConfidenceLevel = "low" | "medium" | "high";

// -----------------------------------------------------------------------------
// Surface Types (walls, doors, windows, openings)
// -----------------------------------------------------------------------------

export type SurfaceCategory = "wall" | "door" | "window" | "opening";

/**
 * A 2D planar architectural structure detected by RoomPlan.
 *
 * Surfaces represent walls, doors, windows, and openings.
 * Each has dimensions, a 4x4 transform matrix, and confidence level.
 */
export interface CapturedRoomSurface {
  /** Unique identifier for this surface */
  identifier: string;

  /** Type of surface: wall, door, window, or opening */
  category: SurfaceCategory;

  /**
   * Dimensions in meters: [width, height, depth]
   * - width: horizontal extent
   * - height: vertical extent
   * - depth: thickness (typically small for walls ~0.1-0.2m)
   */
  dimensions: [number, number, number];

  /**
   * 4x4 transformation matrix (column-major order, 16 elements)
   *
   * Column-major layout:
   * [m0, m1, m2, m3, m4, m5, m6, m7, m8, m9, m10, m11, m12, m13, m14, m15]
   *
   * As a 4x4 matrix:
   * | m0  m4  m8  m12 |   | r00 r01 r02 tx |
   * | m1  m5  m9  m13 | = | r10 r11 r12 ty |
   * | m2  m6  m10 m14 |   | r20 r21 r22 tz |
   * | m3  m7  m11 m15 |   | 0   0   0   1  |
   *
   * Position is at indices [12, 13, 14] = [x, y, z]
   */
  transform: number[];

  /** Confidence level of the detection */
  confidence: ConfidenceLevel;

  /**
   * Parent surface identifier (iOS 17+)
   * e.g., a window's parent is the wall it belongs to
   */
  parent?: string;

  /**
   * Polygon corners for non-rectangular surfaces (iOS 17+)
   * Used for curved walls or walls with beams
   */
  polygonCorners?: Point3D[];

  /**
   * Edge information for the surface
   * Each edge is defined by indices into polygonCorners
   */
  edges?: SurfaceEdge[];
}

export interface SurfaceEdge {
  /** Start point index in polygonCorners */
  startIndex: number;
  /** End point index in polygonCorners */
  endIndex: number;
  /** Edge category */
  category: "wall" | "opening" | "window" | "door";
}

// -----------------------------------------------------------------------------
// Object Types (furniture)
// -----------------------------------------------------------------------------

/**
 * Furniture categories recognized by RoomPlan
 */
export type ObjectCategory =
  // Seating
  | "chair"
  | "sofa"
  // Tables
  | "table"
  // Bedroom
  | "bed"
  // Storage
  | "storage"
  | "cabinet"
  | "shelf"
  // Kitchen appliances
  | "refrigerator"
  | "stove"
  | "oven"
  | "dishwasher"
  | "sink"
  // Bathroom
  | "toilet"
  | "bathtub"
  // Laundry
  | "washer"
  | "dryer"
  // Other
  | "fireplace"
  | "stairs"
  | "television"
  | "screen"
  // Fallback
  | "unknown";

/**
 * A 3D object (furniture) detected by RoomPlan.
 *
 * Objects are represented as oriented bounding boxes.
 */
export interface CapturedRoomObject {
  /** Unique identifier for this object */
  identifier: string;

  /** Category of furniture */
  category: ObjectCategory;

  /**
   * Dimensions in meters: [width, height, depth]
   * These define an oriented bounding box (not axis-aligned)
   */
  dimensions: [number, number, number];

  /**
   * 4x4 transformation matrix (column-major order, 16 elements)
   * Same format as CapturedRoomSurface.transform
   */
  transform: number[];

  /** Confidence level of the detection */
  confidence: ConfidenceLevel;

  /**
   * Parent object identifier (iOS 17+)
   * e.g., a chair's parent might be a table
   */
  parent?: string;

  /**
   * Additional attributes (iOS 17+)
   * Provides more detail about object configuration
   */
  attributes?: ObjectAttributes;
}

/**
 * Additional attributes for objects (iOS 17+)
 */
export interface ObjectAttributes {
  /** For chairs: has arms, has wheels, etc. */
  chairType?: "office" | "dining" | "armchair" | "stool";

  /** For tables: shape */
  tableShape?: "rectangular" | "circular" | "oval";

  /** For storage: open or closed */
  storageType?: "open" | "closed" | "mixed";
}

// -----------------------------------------------------------------------------
// Floor Types
// -----------------------------------------------------------------------------

/**
 * Floor section detected by RoomPlan (iOS 17+)
 */
export interface CapturedRoomFloor {
  /** Unique identifier for this floor section */
  identifier: string;

  /**
   * 4x4 transformation matrix (column-major order)
   * Typically identity or near-identity for floors
   */
  transform: number[];

  /**
   * Polygon corners defining the floor boundary
   * Points are in 3D but typically all have y=0
   */
  polygonCorners: Point3D[];
}

// -----------------------------------------------------------------------------
// Section Types (iOS 17+)
// -----------------------------------------------------------------------------

/**
 * A section represents a distinct area within a room (iOS 17+)
 */
export interface CapturedRoomSection {
  /** Unique identifier */
  identifier: string;

  /** Center point of the section */
  center: Point3D;

  /** Label for the section if recognized */
  label?: string;
}

// -----------------------------------------------------------------------------
// Point Types
// -----------------------------------------------------------------------------

/**
 * 3D point in RoomPlan coordinate space
 *
 * Coordinate system:
 * - X: horizontal (left-right)
 * - Y: vertical (up, perpendicular to floor)
 * - Z: horizontal (front-back)
 */
export interface Point3D {
  x: number;
  y: number;
  z: number;
}

// -----------------------------------------------------------------------------
// Main CapturedRoom JSON Structure
// -----------------------------------------------------------------------------

/**
 * Complete CapturedRoom JSON export from Apple RoomPlan
 *
 * This is the root structure exported when saving a room scan as JSON.
 *
 * @example
 * ```json
 * {
 *   "identifier": "room-001",
 *   "surfaces": [...],
 *   "objects": [...],
 *   "floors": [...]
 * }
 * ```
 */
export interface CapturedRoomJSON {
  /** Unique identifier for this captured room */
  identifier: string;

  /**
   * Architectural surfaces: walls, doors, windows, openings
   */
  surfaces: CapturedRoomSurface[];

  /**
   * Furniture and objects detected in the room
   */
  objects: CapturedRoomObject[];

  /**
   * Floor polygon(s) (iOS 17+)
   * May contain multiple floors for multi-level rooms
   */
  floors?: CapturedRoomFloor[];

  /**
   * Room sections (iOS 17+)
   * Distinct areas within the room
   */
  sections?: CapturedRoomSection[];
}

// -----------------------------------------------------------------------------
// Utility Types
// -----------------------------------------------------------------------------

/**
 * Extracted transform components from a 4x4 matrix
 */
export interface TransformComponents {
  /** Position [x, y, z] extracted from matrix columns 12, 13, 14 */
  position: [number, number, number];

  /** Rotation as Euler angles [x, y, z] in radians */
  rotation: [number, number, number];

  /** Scale factors [x, y, z] (usually [1, 1, 1] for RoomPlan) */
  scale: [number, number, number];
}

/**
 * Helper to extract position from a column-major 4x4 transform matrix
 */
export function extractPosition(transform: number[]): [number, number, number] {
  return [transform[12], transform[13], transform[14]];
}

/**
 * Helper to extract rotation (Euler angles) from a column-major 4x4 transform matrix
 *
 * Uses the rotation submatrix at indices [0,1,2], [4,5,6], [8,9,10]
 */
export function extractRotation(transform: number[]): [number, number, number] {
  // Extract rotation matrix elements
  const r00 = transform[0];
  const r10 = transform[1];
  const r20 = transform[2];
  const r21 = transform[6];
  const r22 = transform[10];

  // Calculate Euler angles (XYZ order)
  const x = Math.atan2(r21, r22);
  const y = Math.atan2(-r20, Math.sqrt(r21 * r21 + r22 * r22));
  const z = Math.atan2(r10, r00);

  return [x, y, z];
}

/**
 * Helper to extract Y-axis rotation (yaw) from transform
 * Most useful for placing objects on a floor plane
 */
export function extractYRotation(transform: number[]): number {
  // For Y-axis rotation, use atan2 of the X and Z components of the X-axis
  return Math.atan2(transform[8], transform[0]);
}
