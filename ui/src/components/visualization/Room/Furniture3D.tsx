/**
 * 3D Furniture Renderer for Apple RoomPlan CapturedRoom
 *
 * Renders furniture as category-colored bounding boxes.
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import { Line } from "@react-three/drei";
import type { CapturedRoomObject, ObjectCategory } from "@/types/worldstate";
import { extractPosition, extractYRotation } from "@/types/capturedRoom";

/**
 * Get color for furniture category
 */
function getCategoryColor(category: ObjectCategory | string): string {
  switch (category) {
    // Tables
    case "table":
      return "#6366f1"; // indigo-500
    // Seating
    case "chair":
      return "#8b5cf6"; // violet-500
    case "sofa":
    case "bed":
      return "#ec4899"; // pink-500
    // Storage
    case "storage":
    case "cabinet":
    case "shelf":
      return "#f59e0b"; // amber-500
    // Kitchen appliances
    case "refrigerator":
    case "stove":
    case "oven":
    case "dishwasher":
    case "sink":
      return "#14b8a6"; // teal-500
    // Bathroom
    case "toilet":
    case "bathtub":
      return "#06b6d4"; // cyan-500
    // Laundry
    case "washer":
    case "dryer":
      return "#10b981"; // emerald-500
    // Electronics
    case "television":
    case "screen":
      return "#3b82f6"; // blue-500
    // Other
    case "fireplace":
      return "#ef4444"; // red-500
    case "stairs":
      return "#78716c"; // stone-500
    default:
      return "#9ca3af"; // gray-400
  }
}

interface Furniture3DProps {
  object: CapturedRoomObject;
}

/**
 * Render a single furniture item as a bounding box
 */
export function Furniture3D({ object }: Furniture3DProps) {
  const { position, rotation, dimensions, edgePoints, color } = useMemo(() => {
    const [x, y, z] = extractPosition(object.transform);
    const yRot = extractYRotation(object.transform);
    const [width, height, depth] = object.dimensions;
    const categoryColor = getCategoryColor(object.category);

    // Create bounding box edge points for wireframe
    const hw = width / 2;
    const hh = height / 2;
    const hd = depth / 2;
    const cos = Math.cos(yRot);
    const sin = Math.sin(yRot);

    // 8 corners of the bounding box
    const corners = [
      [-hw, -hh, -hd],
      [hw, -hh, -hd],
      [hw, -hh, hd],
      [-hw, -hh, hd],
      [-hw, hh, -hd],
      [hw, hh, -hd],
      [hw, hh, hd],
      [-hw, hh, hd],
    ];

    // Transform corners to world space
    const worldCorners = corners.map(([lx, ly, lz]) => {
      const rx = lx * cos - lz * sin;
      const rz = lx * sin + lz * cos;
      return [x + rx, y + ly, z + rz] as [number, number, number];
    });

    // Bottom edges
    const bottomEdges: [number, number, number][][] = [
      [worldCorners[0], worldCorners[1]],
      [worldCorners[1], worldCorners[2]],
      [worldCorners[2], worldCorners[3]],
      [worldCorners[3], worldCorners[0]],
    ];

    // Top edges
    const topEdges: [number, number, number][][] = [
      [worldCorners[4], worldCorners[5]],
      [worldCorners[5], worldCorners[6]],
      [worldCorners[6], worldCorners[7]],
      [worldCorners[7], worldCorners[4]],
    ];

    // Vertical edges
    const verticalEdges: [number, number, number][][] = [
      [worldCorners[0], worldCorners[4]],
      [worldCorners[1], worldCorners[5]],
      [worldCorners[2], worldCorners[6]],
      [worldCorners[3], worldCorners[7]],
    ];

    return {
      position: [x, y, z] as [number, number, number],
      rotation: [0, yRot, 0] as [number, number, number],
      dimensions: [width, height, depth] as [number, number, number],
      edgePoints: [...bottomEdges, ...topEdges, ...verticalEdges],
      color: categoryColor,
    };
  }, [object]);

  return (
    <group name={`furniture-${object.identifier}`}>
      {/* Wireframe edges */}
      {edgePoints.map((edge, i) => (
        <Line key={i} points={edge} color={color} lineWidth={1.5} />
      ))}

      {/* Semi-transparent fill */}
      <mesh position={position} rotation={rotation}>
        <boxGeometry args={dimensions} />
        <meshStandardMaterial color={color} transparent opacity={0.25} roughness={0.7} />
      </mesh>
    </group>
  );
}

interface FurnitureRendererProps {
  objects: CapturedRoomObject[];
}

/**
 * Render all furniture from CapturedRoom objects
 */
export function FurnitureRenderer({ objects }: FurnitureRendererProps) {
  return (
    <group name="furniture">
      {objects.map(obj => (
        <Furniture3D key={obj.identifier} object={obj} />
      ))}
    </group>
  );
}
