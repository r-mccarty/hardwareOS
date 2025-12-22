/**
 * Stats overlay displaying occupancy info and connection status
 *
 * Shows CapturedRoom info when Apple RoomPlan data is available.
 */
import { useState, useEffect } from "react";
import { useWorldStateStore, useRoomPlanStore } from "@/hooks/stores";

export function StatsOverlay() {
  const { worldState, isConnected, lastUpdate } = useWorldStateStore();
  const { roomPlan } = useRoomPlanStore();
  const [isStale, setIsStale] = useState(false);

  const occupantCount = worldState?.occupant_count ?? 0;
  const frameNumber = worldState?.frame_number ?? 0;

  // Check for stale connection periodically
  useEffect(() => {
    const checkStale = () => {
      if (lastUpdate) {
        setIsStale(Date.now() - lastUpdate > 1000);
      } else {
        setIsStale(false);
      }
    };

    checkStale();
    const interval = setInterval(checkStale, 500);
    return () => clearInterval(interval);
  }, [lastUpdate]);

  // CapturedRoom stats
  const hasCapturedRoom = roomPlan?.hasCapturedRoom ?? false;
  const surfaceCount = roomPlan?.capturedRoom?.surfaces?.length ?? 0;
  const objectCount = roomPlan?.capturedRoom?.objects?.length ?? 0;

  return (
    <div className="pointer-events-none absolute top-0 right-0 left-0 p-4">
      <div className="flex items-start justify-between">
        {/* Left: Occupancy count */}
        <div className="rounded-lg bg-slate-900/80 px-4 py-3 backdrop-blur-sm">
          <div className="text-3xl font-bold text-white">{occupantCount}</div>
          <div className="text-xs tracking-wider text-slate-400 uppercase">
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
              {isConnected && !isStale ? "Live" : isStale ? "Stale" : "Disconnected"}
            </span>
          </div>
          {worldState && <div className="mt-1 text-xs text-slate-500">Frame {frameNumber}</div>}
        </div>
      </div>

      {/* Bottom left: Room info */}
      {roomPlan && (
        <div className="absolute bottom-4 left-4 rounded bg-slate-900/60 px-3 py-2 backdrop-blur-sm">
          <div className="flex flex-col gap-1">
            <div className="text-xs text-slate-400">
              Room: {roomPlan.roomWidth.toFixed(1)}m × {roomPlan.roomHeight.toFixed(1)}m
              {!roomPlan.configured && " (defaults)"}
            </div>
            {hasCapturedRoom && (
              <div className="flex items-center gap-2">
                <span className="rounded bg-blue-500/20 px-1.5 py-0.5 text-[10px] font-medium text-blue-400">
                  RoomPlan
                </span>
                <span className="text-[10px] text-slate-500">
                  {surfaceCount} surfaces · {objectCount} objects
                </span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Legend */}
      <div className="absolute right-4 bottom-4 rounded bg-slate-900/60 px-3 py-2 backdrop-blur-sm">
        <div className="flex flex-col gap-1 text-xs">
          <div className="mb-1 text-[10px] font-medium tracking-wider text-slate-500 uppercase">
            Tracking Source
          </div>
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

          {/* Room geometry legend when CapturedRoom is active */}
          {hasCapturedRoom && (
            <>
              <div className="mt-2 mb-1 text-[10px] font-medium tracking-wider text-slate-500 uppercase">
                Room Geometry
              </div>
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded bg-slate-700" />
                <span className="text-slate-400">Walls</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded bg-orange-500" />
                <span className="text-slate-400">Doors</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded bg-blue-400" />
                <span className="text-slate-400">Windows</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded bg-indigo-500" />
                <span className="text-slate-400">Furniture</span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
