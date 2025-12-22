/**
 * Stats overlay displaying occupancy info and connection status
 */
import { useWorldStateStore, useRoomPlanStore } from "@/hooks/stores";

export function StatsOverlay() {
  const { worldState, isConnected, lastUpdate } = useWorldStateStore();
  const { roomPlan } = useRoomPlanStore();

  const occupantCount = worldState?.occupant_count ?? 0;
  const frameNumber = worldState?.frame_number ?? 0;

  // Calculate time since last update
  const timeSinceUpdate = lastUpdate ? Date.now() - lastUpdate : null;
  const isStale = timeSinceUpdate !== null && timeSinceUpdate > 1000;

  return (
    <div className="pointer-events-none absolute left-0 right-0 top-0 p-4">
      <div className="flex items-start justify-between">
        {/* Left: Occupancy count */}
        <div className="rounded-lg bg-slate-900/80 px-4 py-3 backdrop-blur-sm">
          <div className="text-3xl font-bold text-white">{occupantCount}</div>
          <div className="text-xs uppercase tracking-wider text-slate-400">
            {occupantCount === 1 ? "Occupant" : "Occupants"}
          </div>
        </div>

        {/* Right: Connection status */}
        <div className="rounded-lg bg-slate-900/80 px-4 py-3 backdrop-blur-sm">
          <div className="flex items-center gap-2">
            <div
              className={`h-2 w-2 rounded-full ${
                isConnected && !isStale
                  ? "bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.5)]"
                  : isStale
                    ? "bg-yellow-500"
                    : "bg-red-500"
              }`}
            />
            <span className="text-sm text-slate-300">
              {isConnected && !isStale
                ? "Live"
                : isStale
                  ? "Stale"
                  : "Disconnected"}
            </span>
          </div>
          {worldState && (
            <div className="mt-1 text-xs text-slate-500">
              Frame {frameNumber}
            </div>
          )}
        </div>
      </div>

      {/* Bottom: Room info */}
      {roomPlan && (
        <div className="absolute bottom-4 left-4 rounded bg-slate-900/60 px-3 py-2 backdrop-blur-sm">
          <div className="text-xs text-slate-400">
            Room: {roomPlan.roomWidth}m × {roomPlan.roomHeight}m
            {!roomPlan.configured && " (defaults)"}
          </div>
        </div>
      )}

      {/* Legend */}
      <div className="absolute bottom-4 right-4 rounded bg-slate-900/60 px-3 py-2 backdrop-blur-sm">
        <div className="flex flex-col gap-1 text-xs">
          <div className="flex items-center gap-2">
            <div className="h-2 w-2 rounded-full bg-cyan-500" />
            <span className="text-slate-400">Fused</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="h-2 w-2 rounded-full bg-blue-500" />
            <span className="text-slate-400">Radar only</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="h-2 w-2 rounded-full bg-green-500" />
            <span className="text-slate-400">Vision only</span>
          </div>
        </div>
      </div>
    </div>
  );
}
