/**
 * Sensor position and FOV indicator
 *
 * Uses sensor_pose from RoomPlan API (column-major 4x4 matrix)
 * Position is at indices [12, 13, 14] = [x, y, z]
 */
/* eslint-disable react/no-unknown-property */
import { useMemo } from "react";
import { useRoomPlanStore } from "@/hooks/stores";

export function SensorIndicator() {
  const { roomPlan } = useRoomPlanStore();

  // Extract position from column-major 4x4 matrix
  const position = useMemo<[number, number, number]>(() => {
    const pose = roomPlan?.sensorPose ?? [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 5, 4, 2.5, 1];
    // Column-major: translation is at indices 12, 13, 14
    const x = pose[12];
    const y = pose[13]; // Height in room coords
    const z = pose[14]; // Depth in room coords

    // Swap Y/Z for Three.js coordinate system (Y is up)
    return [x, z, y];
  }, [roomPlan?.sensorPose]);

  return (
    <group position={position}>
      {/* Sensor body */}
      <mesh>
        <boxGeometry args={[0.15, 0.1, 0.15]} />
        <meshStandardMaterial color="#f97316" emissive="#f97316" emissiveIntensity={0.5} />
      </mesh>

      {/* LED indicator */}
      <mesh position={[0, 0.06, 0]}>
        <sphereGeometry args={[0.03, 8, 8]} />
        <meshBasicMaterial color="#22c55e" />
      </mesh>

      {/* FOV cone - pointing downward */}
      <mesh position={[0, -1.5, 0]} rotation={[Math.PI, 0, 0]}>
        <coneGeometry args={[2, 3, 32, 1, true]} />
        <meshBasicMaterial color="#f97316" transparent opacity={0.08} side={2} />
      </mesh>

      {/* Mounting pole */}
      <mesh position={[0, 0.3, 0]}>
        <cylinderGeometry args={[0.02, 0.02, 0.5, 8]} />
        <meshStandardMaterial color="#374151" />
      </mesh>
    </group>
  );
}
