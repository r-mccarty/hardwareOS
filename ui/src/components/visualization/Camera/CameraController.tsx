/**
 * Camera controller with orbit controls
 */
import { OrbitControls } from "@react-three/drei";
import { useRoomPlanStore } from "@/hooks/stores";

export function CameraController() {
  const { roomPlan } = useRoomPlanStore();

  // Center orbit controls on room center
  const width = roomPlan?.roomWidth ?? 10;
  const height = roomPlan?.roomHeight ?? 8;

  return (
    <OrbitControls
      target={[width / 2, 0, height / 2]}
      enablePan={true}
      enableZoom={true}
      enableRotate={true}
      minDistance={2}
      maxDistance={50}
      maxPolarAngle={Math.PI / 2 - 0.1} // Prevent going below floor
      minPolarAngle={0.1}
    />
  );
}
