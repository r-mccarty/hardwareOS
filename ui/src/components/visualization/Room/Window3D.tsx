/**
 * 3D Window Renderer for Apple RoomPlan CapturedRoom
 *
 * Renders windows with a glass-like effect and frame outline.
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import { Line } from "@react-three/drei";
import type { CapturedRoomSurface } from "@/types/worldstate";
import { extractPosition, extractYRotation } from "@/types/capturedRoom";

interface Window3DProps {
  surface: CapturedRoomSurface;
}

/**
 * Render a single window with glass effect
 */
export function Window3D({ surface }: Window3DProps) {
  const { position, rotation, dimensions, framePoints } = useMemo(() => {
    const [x, y, z] = extractPosition(surface.transform);
    const yRot = extractYRotation(surface.transform);
    const [width, height] = surface.dimensions;

    // Create window frame points (rectangle in local space)
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
    <group name={`window-${surface.identifier}`}>
      {/* Window frame outline */}
      <Line
        points={framePoints}
        color="#3b82f6" // blue-500
        lineWidth={2}
      />

      {/* Glass effect - semi-transparent with slight reflection */}
      <mesh position={position} rotation={rotation}>
        <planeGeometry args={dimensions} />
        <meshStandardMaterial
          color="#60a5fa" // blue-400
          transparent
          opacity={0.2}
          roughness={0.1}
          metalness={0.3}
          side={2}
        />
      </mesh>

      {/* Window cross dividers (optional decorative) */}
      <WindowDividers
        position={position}
        rotation={rotation}
        width={dimensions[0]}
        height={dimensions[1]}
      />
    </group>
  );
}

interface WindowDividersProps {
  position: [number, number, number];
  rotation: [number, number, number];
  width: number;
  height: number;
}

/**
 * Optional cross dividers for window
 */
function WindowDividers({ position, rotation, width, height }: WindowDividersProps) {
  const [x, y, z] = position;
  const yRot = rotation[1];
  const cos = Math.cos(yRot);
  const sin = Math.sin(yRot);

  // Horizontal divider
  const hLine: [number, number, number][] = [
    [x - (width / 2) * cos, y, z - (width / 2) * sin],
    [x + (width / 2) * cos, y, z + (width / 2) * sin],
  ];

  // Vertical divider
  const vLine: [number, number, number][] = [
    [x, y - height / 2, z],
    [x, y + height / 2, z],
  ];

  return (
    <>
      <Line points={hLine} color="#94a3b8" lineWidth={1} /> {/* slate-400 */}
      <Line points={vLine} color="#94a3b8" lineWidth={1} />
    </>
  );
}

interface WindowsRendererProps {
  surfaces: CapturedRoomSurface[];
}

/**
 * Render all windows from CapturedRoom surfaces
 */
export function WindowsRenderer({ surfaces }: WindowsRendererProps) {
  const windows = useMemo(() => surfaces.filter(s => s.category === "window"), [surfaces]);

  return (
    <group name="windows">
      {windows.map(window => (
        <Window3D key={window.identifier} surface={window} />
      ))}
    </group>
  );
}
