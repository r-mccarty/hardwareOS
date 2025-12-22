/**
 * Demo mode hook that simulates WorldState updates
 *
 * Generates fake occupants walking around the room for visualization testing.
 */
import { useEffect, useRef } from "react";
import { useWorldStateStore, useRoomPlanStore } from "./stores";
import type { WorldStateMessage, TrackedObject } from "@/types/worldstate";

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

export function useWorldStateDemo(enabled: boolean = true) {
  const { setWorldState, setConnected } = useWorldStateStore();
  const { roomPlan } = useRoomPlanStore();
  const occupantsRef = useRef<DemoOccupant[]>([]);
  const frameRef = useRef(0);

  useEffect(() => {
    if (!enabled) return;

    const roomWidth = roomPlan?.roomWidth ?? 10;
    const roomHeight = roomPlan?.roomHeight ?? 8;

    // Initialize demo occupants
    occupantsRef.current = [
      {
        id: "demo_001",
        x: roomWidth * 0.3,
        y: roomHeight * 0.3,
        vx: 0.3,
        vy: 0.2,
        targetX: roomWidth * 0.7,
        targetY: roomHeight * 0.7,
        isRadarOnly: false,
        isVisionOnly: false,
      },
      {
        id: "demo_002",
        x: roomWidth * 0.7,
        y: roomHeight * 0.5,
        vx: -0.2,
        vy: 0.1,
        targetX: roomWidth * 0.2,
        targetY: roomHeight * 0.8,
        isRadarOnly: true,
        isVisionOnly: false,
      },
      {
        id: "demo_003",
        x: roomWidth * 0.5,
        y: roomHeight * 0.8,
        vx: 0.1,
        vy: -0.3,
        targetX: roomWidth * 0.5,
        targetY: roomHeight * 0.2,
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
          // Pick new random target
          occ.targetX = Math.random() * (roomWidth - 2) + 1;
          occ.targetY = Math.random() * (roomHeight - 2) + 1;
        }

        // Update velocity toward target
        const speed = 0.3 + Math.random() * 0.2;
        occ.vx = (dx / dist) * speed * (0.8 + Math.random() * 0.4);
        occ.vy = (dy / dist) * speed * (0.8 + Math.random() * 0.4);

        // Update position
        occ.x += occ.vx * 0.033; // ~30fps
        occ.y += occ.vy * 0.033;

        // Clamp to room bounds
        occ.x = Math.max(0.5, Math.min(roomWidth - 0.5, occ.x));
        occ.y = Math.max(0.5, Math.min(roomHeight - 0.5, occ.y));

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
