/**
 * Demo route for 3D visualization testing
 *
 * Access at /demo to see the visualization with simulated occupants
 */
import { Scene } from "@/components/visualization/Scene";

export default function DemoRoute() {
  return (
    <div className="h-screen w-screen bg-slate-950">
      <Scene demo={true} />
    </div>
  );
}
