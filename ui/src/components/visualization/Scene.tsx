/**
 * Main 3D visualization scene for RS-1 occupancy tracking
 *
 * Tesla FSD-style visualization showing tracked people in a room.
 * Supports both simple RoomPlan format and Apple CapturedRoom 3D geometry.
 */
/* eslint-disable react/no-unknown-property */
import { Canvas } from "@react-three/fiber";
import { Suspense } from "react";
import { useRoomPlan } from "@/hooks/useRoomPlan";
import { useWorldState } from "@/hooks/useWorldState";
import { useWorldStateDemo } from "@/hooks/useWorldStateDemo";
import { useRoomPlanStore } from "@/hooks/stores";
import { RoomFloor } from "./Room/RoomFloor";
import { RoomBoundary } from "./Room/RoomBoundary";
import { CapturedRoomRenderer } from "./Room/CapturedRoomRenderer";
import { OccupantManager } from "./Occupants/OccupantManager";
import { SensorIndicator } from "./Sensor/SensorIndicator";
import { CameraController } from "./Camera/CameraController";
import { StatsOverlay } from "./UI/StatsOverlay";

interface SceneProps {
  deviceUrl?: string;
  demo?: boolean;
}

export function Scene({ deviceUrl, demo = false }: SceneProps) {
  // Fetch room plan on mount (skip API call in demo mode)
  useRoomPlan(deviceUrl, demo);

  // Subscribe to worldstate WebRTC channel (or use demo data)
  useWorldState();
  useWorldStateDemo(demo);

  const { roomPlan, loading, error } = useRoomPlanStore();

  if (loading) {
    return (
      <div className="flex h-full w-full items-center justify-center bg-slate-950">
        <div className="text-slate-400">Loading room configuration...</div>
      </div>
    );
  }

  // Calculate camera position based on room dimensions
  const roomWidth = roomPlan?.roomWidth ?? 10;
  const roomHeight = roomPlan?.roomHeight ?? 8;
  const cameraY = Math.max(roomWidth, roomHeight) * 1.2;
  const cameraZ = Math.max(roomWidth, roomHeight) * 0.8;

  // Check if CapturedRoom data is available for 3D room rendering
  const hasCapturedRoom = roomPlan?.hasCapturedRoom ?? false;

  return (
    <div className="relative h-full w-full bg-slate-950">
      <Canvas
        camera={{
          position: [roomWidth / 2, cameraY, roomHeight / 2 + cameraZ],
          fov: 50,
          near: 0.1,
          far: 100,
        }}
        gl={{ antialias: true }}
      >
        {/* Dark background with fog for depth */}
        <color attach="background" args={["#0a0a0f"]} />
        <fog attach="fog" args={["#0a0a0f", 15, 50]} />

        {/* Lighting */}
        <ambientLight intensity={0.4} />
        <directionalLight position={[10, 20, 10]} intensity={0.6} />
        <directionalLight position={[-10, 15, -10]} intensity={0.3} />

        <Suspense fallback={null}>
          {/* Room geometry - use CapturedRoom 3D or simple 2D boundary */}
          {hasCapturedRoom ? (
            <>
              <CapturedRoomRenderer />
              <RoomFloor />
            </>
          ) : (
            <>
              <RoomFloor />
              <RoomBoundary />
            </>
          )}

          <SensorIndicator />
          <OccupantManager />
        </Suspense>

        <CameraController />
      </Canvas>

      {/* Overlay UI */}
      <StatsOverlay />

      {/* Error display */}
      {error && (
        <div className="absolute bottom-4 left-4 rounded bg-red-900/80 px-3 py-2 text-sm text-red-200">
          {error}
        </div>
      )}
    </div>
  );
}
