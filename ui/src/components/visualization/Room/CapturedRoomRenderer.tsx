/**
 * CapturedRoom Renderer - Orchestrates 3D room geometry rendering
 *
 * Renders walls, doors, windows, and furniture from Apple RoomPlan CapturedRoom data.
 * Provides a Tesla FSD-style visualization of the scanned room.
 */
/* eslint-disable react/no-unknown-property */
import { useRoomPlanStore } from "@/hooks/stores";
import { WallsRenderer } from "./Wall3D";
import { DoorsRenderer } from "./Door3D";
import { WindowsRenderer } from "./Window3D";
import { FurnitureRenderer } from "./Furniture3D";

/**
 * Main CapturedRoom renderer component
 *
 * Reads CapturedRoom data from the RoomPlan store and renders all
 * architectural elements (walls, doors, windows) and furniture.
 */
export function CapturedRoomRenderer() {
  const { roomPlan } = useRoomPlanStore();

  // Only render if CapturedRoom data is available
  if (!roomPlan?.hasCapturedRoom || !roomPlan.capturedRoom) {
    return null;
  }

  const { surfaces, objects } = roomPlan.capturedRoom;

  return (
    <group name="captured-room">
      {/* Architectural surfaces */}
      <WallsRenderer surfaces={surfaces} />
      <DoorsRenderer surfaces={surfaces} />
      <WindowsRenderer surfaces={surfaces} />

      {/* Furniture objects */}
      <FurnitureRenderer objects={objects} />

      {/* Floor plane with subtle grid overlay */}
      <CapturedRoomFloor />
    </group>
  );
}

/**
 * Floor plane for CapturedRoom
 *
 * Uses room dimensions from RoomPlan store and renders a subtle grid.
 */
function CapturedRoomFloor() {
  const { roomPlan } = useRoomPlanStore();

  if (!roomPlan) return null;

  const { roomWidth, roomHeight } = roomPlan; // roomHeight is actually depth (Z)

  return (
    <group position={[roomWidth / 2, 0, roomHeight / 2]}>
      {/* Floor plane */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.005, 0]}>
        <planeGeometry args={[roomWidth, roomHeight]} />
        <meshStandardMaterial color="#0f172a" roughness={0.9} metalness={0} />
      </mesh>
    </group>
  );
}

export default CapturedRoomRenderer;
