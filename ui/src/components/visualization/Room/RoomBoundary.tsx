/**
 * Room boundary outline using polygon from RoomPlan API
 * Falls back to rectangular boundary if no polygon defined
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import { Line } from "@react-three/drei";
import { useRoomPlanStore } from "@/hooks/stores";
import type { Point2D } from "@/types/worldstate";

export function RoomBoundary() {
  const { roomPlan } = useRoomPlanStore();

  const boundaryPoints = useMemo(() => {
    const width = roomPlan?.roomWidth ?? 10;
    const height = roomPlan?.roomHeight ?? 8;

    // Use room polygon if defined, otherwise create rectangle
    if (roomPlan?.roomPolygon && roomPlan.roomPolygon.length >= 3) {
      // Convert 2D polygon to 3D points (Y = 0 is floor level)
      const points = roomPlan.roomPolygon.map((p: Point2D) => [p.x, 0.02, p.y] as const);
      // Close the loop
      points.push(points[0]);
      return points;
    }

    // Default rectangular boundary
    return [
      [0, 0.02, 0] as const,
      [width, 0.02, 0] as const,
      [width, 0.02, height] as const,
      [0, 0.02, height] as const,
      [0, 0.02, 0] as const, // Close the loop
    ];
  }, [roomPlan]);

  return (
    <group>
      {/* Room boundary line */}
      <Line points={boundaryPoints} color="#3b82f6" lineWidth={2} transparent opacity={0.8} />

      {/* Subtle wall indicators */}
      {boundaryPoints.slice(0, -1).map((point, i) => {
        const next = boundaryPoints[i + 1];
        const midX = (point[0] + next[0]) / 2;
        const midZ = (point[2] + next[2]) / 2;

        return (
          <mesh key={i} position={[midX, 0.5, midZ]}>
            <boxGeometry args={[0.02, 1, 0.02]} />
            <meshStandardMaterial color="#1e3a5f" transparent opacity={0.3} />
          </mesh>
        );
      })}

      {/* Obstacles */}
      {roomPlan?.obstacles?.map((obstacle, i) => (
        <ObstacleShape key={i} polygon={obstacle.polygon} />
      ))}
    </group>
  );
}

interface ObstacleShapeProps {
  polygon: Point2D[];
}

function ObstacleShape({ polygon }: ObstacleShapeProps) {
  const points = useMemo(() => {
    if (polygon.length < 3) return [];
    const pts = polygon.map((p: Point2D) => [p.x, 0.02, p.y] as const);
    pts.push(pts[0]); // Close the loop
    return pts;
  }, [polygon]);

  if (points.length === 0) return null;

  return <Line points={points} color="#ef4444" lineWidth={1.5} transparent opacity={0.6} />;
}
