/**
 * 3D Door Renderer for Apple RoomPlan CapturedRoom
 *
 * Renders doors as highlighted openings with frame outline.
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import { Line } from "@react-three/drei";
import type { CapturedRoomSurface } from "@/types/worldstate";
import { extractPosition, extractYRotation } from "@/types/capturedRoom";

interface Door3DProps {
  surface: CapturedRoomSurface;
}

/**
 * Render a single door as a frame outline
 */
export function Door3D({ surface }: Door3DProps) {
  const { position, rotation, dimensions, framePoints } = useMemo(() => {
    const [x, y, z] = extractPosition(surface.transform);
    const yRot = extractYRotation(surface.transform);
    const [width, height] = surface.dimensions;

    // Create door frame points (rectangle in local space)
    const hw = width / 2;
    const hh = height / 2;
    const cos = Math.cos(yRot);
    const sin = Math.sin(yRot);

    // Transform frame corners to world space
    const corners = [
      [-hw, -hh],
      [hw, -hh],
      [hw, hh],
      [-hw, hh],
      [-hw, -hh], // Close the loop
    ];

    const worldCorners = corners.map(([lx, ly]) => {
      // Rotate around Y axis and translate
      return [x + lx * cos, y + ly, z + lx * sin] as [number, number, number];
    });

    return {
      position: [x, y, z] as [number, number, number],
      rotation: [0, yRot, 0] as [number, number, number],
      dimensions: [width, height] as [number, number],
      framePoints: worldCorners,
    };
  }, [surface]);

  return (
    <group name={`door-${surface.identifier}`}>
      {/* Door frame outline */}
      <Line
        points={framePoints}
        color="#f97316" // orange-500
        lineWidth={3}
      />

      {/* Semi-transparent door fill */}
      <mesh position={position} rotation={rotation}>
        <planeGeometry args={dimensions} />
        <meshBasicMaterial color="#f97316" transparent opacity={0.15} side={2} />
      </mesh>
    </group>
  );
}

interface DoorsRendererProps {
  surfaces: CapturedRoomSurface[];
}

/**
 * Render all doors from CapturedRoom surfaces
 */
export function DoorsRenderer({ surfaces }: DoorsRendererProps) {
  const doors = useMemo(() => surfaces.filter(s => s.category === "door"), [surfaces]);

  return (
    <group name="doors">
      {doors.map(door => (
        <Door3D key={door.identifier} surface={door} />
      ))}
    </group>
  );
}
