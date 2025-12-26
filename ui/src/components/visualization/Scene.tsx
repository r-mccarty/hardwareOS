/**
 * Main 3D visualization scene for RS-1 occupancy tracking
 *
 * Tesla FSD-style visualization with:
 * - Bloom post-processing for signature glow effect
 * - Motion trails and prediction cones on tracked objects
 * - Radar sweep animation on floor grid
 * - Uncertainty halos based on tracking confidence
 */
/* eslint-disable react/no-unknown-property */
import { Canvas } from "@react-three/fiber";
import { Suspense } from "react";
import { EffectComposer, Bloom, Vignette } from "@react-three/postprocessing";
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
        gl={{
          antialias: true,
          alpha: false,
          powerPreference: "high-performance",
        }}
        dpr={[1, 2]} // Responsive pixel ratio
      >
        {/* Deep dark background - Tesla FSD signature */}
        <color attach="background" args={["#030308"]} />
        <fog attach="fog" args={["#030308", 12, 45]} />

        {/* Enhanced lighting for FSD aesthetic */}
        <ambientLight intensity={0.25} color="#4a5568" />
        <directionalLight
          position={[10, 25, 10]}
          intensity={0.5}
          color="#60a5fa"
        />
        <directionalLight
          position={[-10, 20, -10]}
          intensity={0.3}
          color="#3b82f6"
        />
        {/* Subtle fill light from below for that futuristic look */}
        <pointLight position={[0, -5, 0]} intensity={0.1} color="#1e40af" />

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

        {/* Post-processing effects for Tesla FSD glow */}
        <EffectComposer>
          <Bloom
            luminanceThreshold={0.2}
            luminanceSmoothing={0.9}
            intensity={0.8}
            radius={0.8}
          />
          <Vignette
            eskil={false}
            offset={0.3}
            darkness={0.6}
          />
        </EffectComposer>
      </Canvas>

      {/* Overlay UI */}
      <StatsOverlay />

      {/* Scan line overlay effect (CSS) */}
      <div
        className="pointer-events-none absolute inset-0 opacity-[0.03]"
        style={{
          backgroundImage:
            "repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(255,255,255,0.03) 2px, rgba(255,255,255,0.03) 4px)",
        }}
      />

      {/* Corner accents - Tesla FSD UI style */}
      <div className="pointer-events-none absolute left-4 top-4 h-8 w-8 border-l-2 border-t-2 border-blue-500/30" />
      <div className="pointer-events-none absolute right-4 top-4 h-8 w-8 border-r-2 border-t-2 border-blue-500/30" />
      <div className="pointer-events-none absolute bottom-4 left-4 h-8 w-8 border-b-2 border-l-2 border-blue-500/30" />
      <div className="pointer-events-none absolute bottom-4 right-4 h-8 w-8 border-b-2 border-r-2 border-blue-500/30" />

      {/* Error display */}
      {error && (
        <div className="absolute bottom-4 left-4 rounded bg-red-900/80 px-3 py-2 text-sm text-red-200">
          {error}
        </div>
      )}
    </div>
  );
}
