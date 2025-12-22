/**
 * Mock CapturedRoom data for demo mode
 *
 * Simulates a 6m x 5m office/living room with:
 * - 4 walls with varying heights
 * - 1 door in the right wall
 * - 2 windows (back wall and left wall)
 * - Furniture: desk, chair, sofa, coffee table, bookshelf
 */

import type { CapturedRoomJSON } from "@/types/capturedRoom";

/**
 * Create a column-major 4x4 identity matrix with translation and optional Y rotation
 *
 * @param x - X position
 * @param y - Y position (height)
 * @param z - Z position (depth)
 * @param yawDeg - Y-axis rotation in degrees (default 0)
 */
function createTransform(x: number, y: number, z: number, yawDeg = 0): number[] {
  const yaw = (yawDeg * Math.PI) / 180;
  const cos = Math.cos(yaw);
  const sin = Math.sin(yaw);

  // Column-major 4x4 matrix with Y-axis rotation
  return [
    cos,
    0,
    -sin,
    0, // Column 0
    0,
    1,
    0,
    0, // Column 1
    sin,
    0,
    cos,
    0, // Column 2
    x,
    y,
    z,
    1, // Column 3 (translation)
  ];
}

/**
 * Room dimensions: 6m wide (X) x 5m deep (Z) x 2.8m tall (Y)
 */
const ROOM_WIDTH = 6;
const ROOM_DEPTH = 5;
const WALL_HEIGHT = 2.8;
const WALL_THICKNESS = 0.15;

export const MOCK_CAPTURED_ROOM: CapturedRoomJSON = {
  identifier: "demo-office-001",

  surfaces: [
    // =========================================================================
    // WALLS
    // =========================================================================

    // Back wall (Z = 0, faces +Z)
    {
      identifier: "wall-back",
      category: "wall",
      dimensions: [ROOM_WIDTH, WALL_HEIGHT, WALL_THICKNESS],
      transform: createTransform(ROOM_WIDTH / 2, WALL_HEIGHT / 2, 0, 0),
      confidence: "high",
    },

    // Front wall (Z = ROOM_DEPTH, faces -Z)
    {
      identifier: "wall-front",
      category: "wall",
      dimensions: [ROOM_WIDTH, WALL_HEIGHT, WALL_THICKNESS],
      transform: createTransform(ROOM_WIDTH / 2, WALL_HEIGHT / 2, ROOM_DEPTH, 0),
      confidence: "high",
    },

    // Left wall (X = 0, faces +X)
    {
      identifier: "wall-left",
      category: "wall",
      dimensions: [ROOM_DEPTH, WALL_HEIGHT, WALL_THICKNESS],
      transform: createTransform(0, WALL_HEIGHT / 2, ROOM_DEPTH / 2, 90),
      confidence: "high",
    },

    // Right wall (X = ROOM_WIDTH, faces -X)
    {
      identifier: "wall-right",
      category: "wall",
      dimensions: [ROOM_DEPTH, WALL_HEIGHT, WALL_THICKNESS],
      transform: createTransform(ROOM_WIDTH, WALL_HEIGHT / 2, ROOM_DEPTH / 2, 90),
      confidence: "high",
    },

    // =========================================================================
    // DOOR
    // =========================================================================

    // Door in right wall (near front)
    {
      identifier: "door-main",
      category: "door",
      dimensions: [0.9, 2.1, 0.1],
      transform: createTransform(ROOM_WIDTH, 2.1 / 2, 1.2, 90),
      confidence: "high",
      parent: "wall-right",
    },

    // =========================================================================
    // WINDOWS
    // =========================================================================

    // Window in back wall (centered)
    {
      identifier: "window-back",
      category: "window",
      dimensions: [1.5, 1.2, 0.1],
      transform: createTransform(ROOM_WIDTH / 2, 1.6, 0, 0),
      confidence: "high",
      parent: "wall-back",
    },

    // Window in left wall
    {
      identifier: "window-left",
      category: "window",
      dimensions: [1.2, 1.0, 0.1],
      transform: createTransform(0, 1.5, 2.5, 90),
      confidence: "high",
      parent: "wall-left",
    },
  ],

  objects: [
    // =========================================================================
    // FURNITURE
    // =========================================================================

    // Desk against back wall
    {
      identifier: "desk-main",
      category: "table",
      dimensions: [1.6, 0.75, 0.8],
      transform: createTransform(1.5, 0.375, 0.6, 0),
      confidence: "high",
    },

    // Office chair at desk
    {
      identifier: "chair-desk",
      category: "chair",
      dimensions: [0.6, 1.1, 0.6],
      transform: createTransform(1.5, 0.55, 1.3, 0),
      confidence: "high",
    },

    // Sofa against front wall
    {
      identifier: "sofa-main",
      category: "sofa",
      dimensions: [2.2, 0.85, 0.95],
      transform: createTransform(ROOM_WIDTH / 2, 0.425, ROOM_DEPTH - 0.6, 0),
      confidence: "high",
    },

    // Coffee table in front of sofa
    {
      identifier: "table-coffee",
      category: "table",
      dimensions: [1.2, 0.45, 0.6],
      transform: createTransform(ROOM_WIDTH / 2, 0.225, ROOM_DEPTH - 1.8, 0),
      confidence: "medium",
    },

    // Bookshelf against left wall
    {
      identifier: "shelf-books",
      category: "storage",
      dimensions: [0.4, 1.8, 1.0],
      transform: createTransform(0.3, 0.9, 3.8, 90),
      confidence: "high",
    },

    // Side table next to sofa
    {
      identifier: "table-side",
      category: "table",
      dimensions: [0.5, 0.55, 0.5],
      transform: createTransform(4.5, 0.275, ROOM_DEPTH - 0.5, 0),
      confidence: "medium",
    },

    // Plant stand (treated as storage)
    {
      identifier: "plant-stand",
      category: "storage",
      dimensions: [0.35, 0.8, 0.35],
      transform: createTransform(5.5, 0.4, 0.4, 0),
      confidence: "low",
    },
  ],

  floors: [
    {
      identifier: "floor-main",
      transform: createTransform(ROOM_WIDTH / 2, 0, ROOM_DEPTH / 2, 0),
      polygonCorners: [
        { x: 0, y: 0, z: 0 },
        { x: ROOM_WIDTH, y: 0, z: 0 },
        { x: ROOM_WIDTH, y: 0, z: ROOM_DEPTH },
        { x: 0, y: 0, z: ROOM_DEPTH },
      ],
    },
  ],

  sections: [
    {
      identifier: "section-work",
      center: { x: 1.5, y: 0, z: 1.0 },
      label: "Work Area",
    },
    {
      identifier: "section-lounge",
      center: { x: ROOM_WIDTH / 2, y: 0, z: ROOM_DEPTH - 1.5 },
      label: "Lounge Area",
    },
  ],
};

/**
 * Get the mock captured room as an ExtendedRoomPlanState
 * (pre-parsed for direct use in stores)
 */
export function getMockRoomPlanState() {
  // Import parser dynamically to avoid circular deps
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { parseCapturedRoom } = require("./capturedRoomParser");
  return parseCapturedRoom(MOCK_CAPTURED_ROOM);
}
