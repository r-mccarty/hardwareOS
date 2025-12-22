/**
 * WorldState and RoomPlan types for RS-1 3D visualization
 */

// Tracked object from WorldState stream (30Hz via WebRTC)
export interface TrackedObject {
  track_id: string;
  x: number; // meters (room coordinates)
  y: number; // meters (room coordinates)
  vx: number; // m/s
  vy: number; // m/s
  confidence: number; // 0-1
  heading: number; // degrees from +Y axis
  speed: number; // m/s
  is_radar_only: boolean;
  is_vision_only: boolean;
}

// WorldState message from WebRTC DataChannel
export interface WorldStateMessage {
  timestamp_ns: number;
  frame_number: number;
  occupant_count: number;
  objects: TrackedObject[];
}

// 2D point for polygons
export interface Point2D {
  x: number;
  y: number;
}

// Obstacle in room
export interface Obstacle {
  type: string;
  polygon: Point2D[];
}

// RoomPlan from GET /api/setup/roomplan
export interface RoomPlanResponse {
  success: boolean;
  configured: boolean;
  message?: string;
  room_width?: number;
  room_height?: number;
  sensor_pose?: number[]; // 16-element column-major 4x4 matrix
  room_polygon?: Point2D[];
  obstacles?: Obstacle[];
  last_updated?: string;
}

// Normalized RoomPlan state (camelCase)
export interface RoomPlanState {
  configured: boolean;
  roomWidth: number;
  roomHeight: number;
  sensorPose: number[];
  roomPolygon: Point2D[];
  obstacles: Obstacle[];
}

// Re-export CapturedRoom types for convenience
export type {
  CapturedRoomJSON,
  CapturedRoomSurface,
  CapturedRoomObject,
  CapturedRoomFloor,
  SurfaceCategory,
  ObjectCategory,
  ConfidenceLevel,
  Point3D,
} from "./capturedRoom";

// Extended RoomPlan state with Apple CapturedRoom data
export interface ExtendedRoomPlanState extends RoomPlanState {
  /**
   * Full CapturedRoom data from Apple RoomPlan export
   * Contains detailed 3D geometry for walls, doors, windows, and furniture
   */
  capturedRoom?: {
    identifier: string;
    surfaces: import("./capturedRoom").CapturedRoomSurface[];
    objects: import("./capturedRoom").CapturedRoomObject[];
    floors?: import("./capturedRoom").CapturedRoomFloor[];
  };

  /**
   * Flag indicating CapturedRoom data is available
   * When true, visualization should use 3D room geometry
   * When false, fall back to simple 2D boundary rendering
   */
  hasCapturedRoom: boolean;
}
