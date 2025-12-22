/**
 * Demo mode hook that simulates WorldState updates
 *
 * Generates fake occupants walking around the room for visualization testing.
 * In demo mode, also loads mock CapturedRoom data for 3D room rendering.
 */
import { useEffect, useRef } from "react";
import type { WorldStateMessage, TrackedObject } from "@/types/worldstate";
import { MOCK_CAPTURED_ROOM } from "@/utils/mockCapturedRoom";
import { parseCapturedRoom } from "@/utils/capturedRoomParser";
import { useWorldStateStore, useRoomPlanStore } from "./stores";

interface DemoOccupant {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  targetX: number;
  targetY: number;
  isRadarOnly: boolean;
  isVisionOnly: boolean;
}

// Furniture bounding boxes to avoid (extracted from mock data)
interface BoundingBox {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
}

/**
 * Extract furniture bounding boxes for collision avoidance
 */
function getFurnitureBounds(): BoundingBox[] {
  return MOCK_CAPTURED_ROOM.objects.map(obj => {
    const [x, , z] = [obj.transform[12], obj.transform[13], obj.transform[14]];
    const [w, , d] = obj.dimensions;
    return {
      minX: x - w / 2 - 0.3, // Add padding
      maxX: x + w / 2 + 0.3,
      minZ: z - d / 2 - 0.3,
      maxZ: z + d / 2 + 0.3,
    };
  });
}

/**
 * Check if a point is inside any furniture bounding box
 */
function isInsideFurniture(x: number, z: number, bounds: BoundingBox[]): boolean {
  return bounds.some(b => x >= b.minX && x <= b.maxX && z >= b.minZ && z <= b.maxZ);
}

/**
 * Find a valid target position that avoids furniture
 */
function findValidTarget(
  roomWidth: number,
  roomDepth: number,
  bounds: BoundingBox[],
  maxAttempts = 20,
): { x: number; z: number } {
  for (let i = 0; i < maxAttempts; i++) {
    const x = Math.random() * (roomWidth - 1) + 0.5;
    const z = Math.random() * (roomDepth - 1) + 0.5;
    if (!isInsideFurniture(x, z, bounds)) {
      return { x, z };
    }
  }
  // Fall back to center if no valid position found
  return { x: roomWidth / 2, z: roomDepth / 2 };
}

export function useWorldStateDemo(enabled = true) {
  const { setWorldState, setConnected } = useWorldStateStore();
  const { roomPlan, setRoomPlan } = useRoomPlanStore();
  const occupantsRef = useRef<DemoOccupant[]>([]);
  const frameRef = useRef(0);
  const furnitureBoundsRef = useRef<BoundingBox[]>([]);

  // Load mock CapturedRoom data when demo mode is enabled
  useEffect(() => {
    if (!enabled) return;

    // Parse and set mock CapturedRoom data
    const mockRoomPlan = parseCapturedRoom(MOCK_CAPTURED_ROOM);
    setRoomPlan(mockRoomPlan);

    // Cache furniture bounds for collision avoidance
    furnitureBoundsRef.current = getFurnitureBounds();
  }, [enabled, setRoomPlan]);

  // Simulate occupant movement
  useEffect(() => {
    if (!enabled || !roomPlan) return;

    const roomWidth = roomPlan.roomWidth;
    const roomDepth = roomPlan.roomHeight; // roomHeight is actually depth (Z)
    const bounds = furnitureBoundsRef.current;

    // Initialize demo occupants at valid positions
    const startPositions = [
      findValidTarget(roomWidth, roomDepth, bounds),
      findValidTarget(roomWidth, roomDepth, bounds),
      findValidTarget(roomWidth, roomDepth, bounds),
    ];

    occupantsRef.current = [
      {
        id: "demo_001",
        x: startPositions[0].x,
        y: startPositions[0].z, // y in 2D = z in 3D
        vx: 0.3,
        vy: 0.2,
        targetX: findValidTarget(roomWidth, roomDepth, bounds).x,
        targetY: findValidTarget(roomWidth, roomDepth, bounds).z,
        isRadarOnly: false,
        isVisionOnly: false,
      },
      {
        id: "demo_002",
        x: startPositions[1].x,
        y: startPositions[1].z,
        vx: -0.2,
        vy: 0.1,
        targetX: findValidTarget(roomWidth, roomDepth, bounds).x,
        targetY: findValidTarget(roomWidth, roomDepth, bounds).z,
        isRadarOnly: true,
        isVisionOnly: false,
      },
      {
        id: "demo_003",
        x: startPositions[2].x,
        y: startPositions[2].z,
        vx: 0.1,
        vy: -0.3,
        targetX: findValidTarget(roomWidth, roomDepth, bounds).x,
        targetY: findValidTarget(roomWidth, roomDepth, bounds).z,
        isRadarOnly: false,
        isVisionOnly: true,
      },
    ];

    setConnected(true);

    const interval = setInterval(() => {
      frameRef.current++;

      // Update occupant positions
      const objects: TrackedObject[] = occupantsRef.current.map(occ => {
        // Move toward target
        const dx = occ.targetX - occ.x;
        const dy = occ.targetY - occ.y;
        const dist = Math.sqrt(dx * dx + dy * dy);

        if (dist < 0.5) {
          // Pick new random target (avoiding furniture)
          const newTarget = findValidTarget(roomWidth, roomDepth, bounds);
          occ.targetX = newTarget.x;
          occ.targetY = newTarget.z;
        }

        // Update velocity toward target
        const speed = 0.4 + Math.random() * 0.2;
        occ.vx = (dx / dist) * speed * (0.8 + Math.random() * 0.4);
        occ.vy = (dy / dist) * speed * (0.8 + Math.random() * 0.4);

        // Update position
        let newX = occ.x + occ.vx * 0.033; // ~30fps
        let newY = occ.y + occ.vy * 0.033;

        // Check for furniture collision and bounce
        if (isInsideFurniture(newX, newY, bounds)) {
          // Pick a new target and reverse direction
          const newTarget = findValidTarget(roomWidth, roomDepth, bounds);
          occ.targetX = newTarget.x;
          occ.targetY = newTarget.z;
          occ.vx = -occ.vx * 0.5;
          occ.vy = -occ.vy * 0.5;
          newX = occ.x + occ.vx * 0.033;
          newY = occ.y + occ.vy * 0.033;
        }

        occ.x = newX;
        occ.y = newY;

        // Clamp to room bounds
        occ.x = Math.max(0.5, Math.min(roomWidth - 0.5, occ.x));
        occ.y = Math.max(0.5, Math.min(roomDepth - 0.5, occ.y));

        // Calculate heading (degrees from +Y axis)
        const heading = Math.atan2(occ.vx, occ.vy) * (180 / Math.PI);

        return {
          track_id: occ.id,
          x: occ.x,
          y: occ.y,
          vx: occ.vx,
          vy: occ.vy,
          confidence: 0.85 + Math.random() * 0.15,
          heading: heading,
          speed: Math.sqrt(occ.vx * occ.vx + occ.vy * occ.vy),
          is_radar_only: occ.isRadarOnly,
          is_vision_only: occ.isVisionOnly,
        };
      });

      const message: WorldStateMessage = {
        timestamp_ns: Date.now() * 1_000_000,
        frame_number: frameRef.current,
        occupant_count: objects.length,
        objects,
      };

      setWorldState(message);
    }, 33); // ~30fps

    return () => {
      clearInterval(interval);
      setConnected(false);
    };
  }, [enabled, roomPlan, setWorldState, setConnected]);
}
