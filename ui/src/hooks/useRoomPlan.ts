/**
 * Hook to fetch RoomPlan configuration from the device API
 *
 * Fetches room dimensions, sensor pose, and obstacle data from
 * GET /api/setup/roomplan before rendering the 3D scene.
 */
import { useEffect } from "react";
import { useRoomPlanStore } from "./stores";
import type { RoomPlanState, RoomPlanResponse } from "@/types/worldstate";

const DEFAULT_ROOM_PLAN: RoomPlanState = {
  configured: false,
  roomWidth: 10,
  roomHeight: 8,
  sensorPose: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 5, 4, 2.5, 1],
  roomPolygon: [],
  obstacles: [],
};

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

        const data: RoomPlanResponse = await res.json();

        if (data.success && data.configured) {
          setRoomPlan({
            configured: true,
            roomWidth: data.room_width ?? DEFAULT_ROOM_PLAN.roomWidth,
            roomHeight: data.room_height ?? DEFAULT_ROOM_PLAN.roomHeight,
            sensorPose: data.sensor_pose ?? DEFAULT_ROOM_PLAN.sensorPose,
            roomPolygon: data.room_polygon ?? [],
            obstacles: data.obstacles ?? [],
          });
        } else {
          // Use defaults if room not configured
          console.log("[useRoomPlan] Room not configured, using defaults");
          setRoomPlan(DEFAULT_ROOM_PLAN);
        }
      } catch (err) {
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
