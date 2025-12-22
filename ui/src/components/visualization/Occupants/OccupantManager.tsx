/**
 * Manages rendering of all tracked occupants from WorldState
 */
import { useWorldStateStore } from "@/hooks/stores";
import { HumanSilhouette } from "./HumanSilhouette";

export function OccupantManager() {
  const { worldState } = useWorldStateStore();

  if (!worldState?.objects) {
    return null;
  }

  return (
    <group>
      {worldState.objects.map(obj => (
        <HumanSilhouette key={obj.track_id} object={obj} />
      ))}
    </group>
  );
}
