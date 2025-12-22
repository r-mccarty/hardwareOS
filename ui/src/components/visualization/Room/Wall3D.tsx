/**
 * 3D Wall Renderer for Apple RoomPlan CapturedRoom
 *
 * Renders walls as semi-transparent boxes using dimensions and transform from CapturedRoom.
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import type { CapturedRoomSurface } from "@/types/worldstate";
import { extractPosition, extractYRotation } from "@/types/capturedRoom";

interface Wall3DProps {
  surface: CapturedRoomSurface;
}

/**
 * Render a single wall as a 3D box
 */
export function Wall3D({ surface }: Wall3DProps) {
  const { position, rotation, dimensions } = useMemo(() => {
    const [x, y, z] = extractPosition(surface.transform);
    const yRot = extractYRotation(surface.transform);
    const [width, height, depth] = surface.dimensions;

    return {
      position: [x, y, z] as [number, number, number],
      rotation: [0, yRot, 0] as [number, number, number],
      dimensions: [width, depth, height] as [number, number, number], // Swap depth/height for Three.js
    };
  }, [surface]);

  return (
    <mesh position={position} rotation={rotation}>
      <boxGeometry args={dimensions} />
      <meshStandardMaterial
        color="#1e293b" // slate-800
        transparent
        opacity={0.4}
        roughness={0.8}
        metalness={0.1}
      />
    </mesh>
  );
}

interface WallsRendererProps {
  surfaces: CapturedRoomSurface[];
}

/**
 * Render all walls from CapturedRoom surfaces
 */
export function WallsRenderer({ surfaces }: WallsRendererProps) {
  const walls = useMemo(() => surfaces.filter(s => s.category === "wall"), [surfaces]);

  return (
    <group name="walls">
      {walls.map(wall => (
        <Wall3D key={wall.identifier} surface={wall} />
      ))}
    </group>
  );
}
