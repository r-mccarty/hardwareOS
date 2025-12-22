/**
 * Grid floor component using room dimensions from RoomPlan API
 */
/* eslint-disable react/no-unknown-property */
import { Grid } from "@react-three/drei";
import { useRoomPlanStore } from "@/hooks/stores";

export function RoomFloor() {
  const { roomPlan } = useRoomPlanStore();
  const width = roomPlan?.roomWidth ?? 10;
  const height = roomPlan?.roomHeight ?? 8;

  return (
    <group position={[width / 2, 0, height / 2]}>
      {/* Main grid - Tesla FSD style blue lines */}
      <Grid
        args={[width * 2, height * 2]}
        cellSize={0.5}
        cellThickness={0.5}
        cellColor="#1e3a5f"
        sectionSize={1}
        sectionThickness={1}
        sectionColor="#2563eb"
        fadeDistance={40}
        fadeStrength={1}
        infiniteGrid={false}
      />

      {/* Subtle floor plane for shadows */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.01, 0]} receiveShadow>
        <planeGeometry args={[width, height]} />
        <meshStandardMaterial color="#0a0a0f" transparent opacity={0.5} />
      </mesh>
    </group>
  );
}
