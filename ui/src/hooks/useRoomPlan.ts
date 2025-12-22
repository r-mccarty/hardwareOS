/**
 * Hook to fetch RoomPlan configuration from the device API
 *
 * Fetches room dimensions, sensor pose, and obstacle data from
 * GET /api/setup/roomplan before rendering the 3D scene.
 *
 * Supports both simple RoomPlan format and Apple CapturedRoom JSON.
 */
import { useEffect } from "react";
import type { ExtendedRoomPlanState, RoomPlanResponse } from "@/types/worldstate";
import { parseCapturedRoom, validateCapturedRoom } from "@/utils/capturedRoomParser";
import { useRoomPlanStore } from "./stores";

const DEFAULT_ROOM_PLAN: ExtendedRoomPlanState = {
  configured: false,
  roomWidth: 10,
  roomHeight: 8,
  sensorPose: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 5, 4, 2.5, 1],
  roomPolygon: [],
  obstacles: [],
  hasCapturedRoom: false,
};

// Extended API response type that may include captured_room
interface ExtendedRoomPlanResponse extends RoomPlanResponse {
  captured_room?: unknown; // Raw CapturedRoom JSON from API
}

export function useRoomPlan(deviceUrl?: string, skip?: boolean) {
  const { roomPlan, loading, error, setRoomPlan, setLoading, setError } = useRoomPlanStore();

  useEffect(() => {
    // Skip fetching if requested (e.g., demo mode)
    if (skip) {
      setRoomPlan(DEFAULT_ROOM_PLAN);
      return;
    }

    async function fetchRoomPlan() {
      setLoading(true);
      setError(null);

      try {
        // Build the API URL - use relative path if on device, absolute if cloud
        const baseUrl = deviceUrl || "";
        const url = `${baseUrl}/api/setup/roomplan`;

        const res = await fetch(url);

        if (!res.ok) {
          throw new Error(`HTTP ${res.status}: ${res.statusText}`);
        }

        const data: ExtendedRoomPlanResponse = await res.json();

        if (data.success && data.configured) {
          // Check if CapturedRoom data is present
          if (data.captured_room) {
            const validation = validateCapturedRoom(data.captured_room);
            if (validation.valid) {
              // Parse CapturedRoom into ExtendedRoomPlanState
              const parsed = parseCapturedRoom(validation.data);
              // Override sensor pose if provided in API response
              if (data.sensor_pose) {
                parsed.sensorPose = data.sensor_pose;
              }
              setRoomPlan(parsed);
              return;
            } else {
              console.warn("[useRoomPlan] Invalid CapturedRoom data:", validation.error);
            }
          }

          // Fall back to simple RoomPlan format
          setRoomPlan({
            configured: true,
            roomWidth: data.room_width ?? DEFAULT_ROOM_PLAN.roomWidth,
            roomHeight: data.room_height ?? DEFAULT_ROOM_PLAN.roomHeight,
            sensorPose: data.sensor_pose ?? DEFAULT_ROOM_PLAN.sensorPose,
            roomPolygon: data.room_polygon ?? [],
            obstacles: data.obstacles ?? [],
            hasCapturedRoom: false,
          });
        } else {
          // Use defaults if room not configured
          console.log("[useRoomPlan] Room not configured, using defaults");
          setRoomPlan(DEFAULT_ROOM_PLAN);
        }
      } catch {
        // Silently use defaults - API may not be available (e.g., demo mode)
        setRoomPlan(DEFAULT_ROOM_PLAN);
      } finally {
        setLoading(false);
      }
    }

    fetchRoomPlan();
  }, [deviceUrl, skip, setRoomPlan, setLoading, setError]);

  return { roomPlan, loading, error };
}
