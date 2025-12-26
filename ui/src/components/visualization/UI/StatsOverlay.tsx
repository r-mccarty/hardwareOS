/**
 * Tesla FSD-style HUD overlay displaying occupancy info and diagnostics
 *
 * Features:
 * - Live occupancy count with confidence meter
 * - Connection status with frame rate indicator
 * - Tracking source breakdown
 * - CapturedRoom geometry info
 */
import { useState, useEffect, useMemo, useRef } from "react";
import { useWorldStateStore, useRoomPlanStore } from "@/hooks/stores";

export function StatsOverlay() {
  const { worldState, isConnected, lastUpdate } = useWorldStateStore();
  const { roomPlan } = useRoomPlanStore();
  const [isStale, setIsStale] = useState(false);
  const [frameRate, setFrameRate] = useState(0);
  const lastFrameNumberRef = useRef(0);
  const lastFrameTimeRef = useRef(0);

  const occupantCount = worldState?.occupant_count ?? 0;
  const frameNumber = worldState?.frame_number ?? 0;

  // Calculate frame rate
  useEffect(() => {
    if (frameNumber !== lastFrameNumberRef.current) {
      const now = Date.now();
      const elapsed = now - lastFrameTimeRef.current;
      if (elapsed > 0) {
        const instantFps = 1000 / elapsed;
        setFrameRate(prev => prev * 0.8 + instantFps * 0.2); // Smoothed
      }
      lastFrameNumberRef.current = frameNumber;
      lastFrameTimeRef.current = now;
    }
  }, [frameNumber]);

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

  // Calculate tracking source breakdown
  const trackingStats = useMemo(() => {
    if (!worldState?.objects) return { fused: 0, radar: 0, vision: 0, avgConfidence: 0 };

    let fused = 0,
      radar = 0,
      vision = 0,
      totalConfidence = 0;

    for (const obj of worldState.objects) {
      if (obj.is_radar_only) radar++;
      else if (obj.is_vision_only) vision++;
      else fused++;
      totalConfidence += obj.confidence;
    }

    return {
      fused,
      radar,
      vision,
      avgConfidence:
        worldState.objects.length > 0 ? totalConfidence / worldState.objects.length : 0,
    };
  }, [worldState]);

  // CapturedRoom stats
  const hasCapturedRoom = roomPlan?.hasCapturedRoom ?? false;
  const surfaceCount = roomPlan?.capturedRoom?.surfaces?.length ?? 0;
  const objectCount = roomPlan?.capturedRoom?.objects?.length ?? 0;

  // Connection status color
  const statusColor = isConnected && !isStale ? "#22c55e" : isStale ? "#eab308" : "#ef4444";

  return (
    <div className="pointer-events-none absolute inset-0 p-4">
      {/* Top bar */}
      <div className="flex items-start justify-between">
        {/* Left: Occupancy count - Tesla FSD style big number */}
        <div className="relative">
          <div className="rounded-lg border border-slate-700/50 bg-slate-900/90 px-5 py-3 backdrop-blur-md">
            <div className="flex items-baseline gap-2">
              <span
                className="font-mono text-5xl font-bold tracking-tight"
                style={{ color: occupantCount > 0 ? "#06b6d4" : "#64748b" }}
              >
                {occupantCount}
              </span>
              <span className="text-lg text-slate-500">
                {occupantCount === 1 ? "person" : "people"}
              </span>
            </div>
            {/* Confidence bar */}
            {occupantCount > 0 && (
              <div className="mt-2 flex items-center gap-2">
                <span className="text-[10px] tracking-wider text-slate-500 uppercase">
                  Confidence
                </span>
                <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
                  <div
                    className="h-full rounded-full transition-all duration-300"
                    style={{
                      width: `${trackingStats.avgConfidence * 100}%`,
                      backgroundColor:
                        trackingStats.avgConfidence > 0.7
                          ? "#22c55e"
                          : trackingStats.avgConfidence > 0.4
                            ? "#eab308"
                            : "#ef4444",
                    }}
                  />
                </div>
                <span className="w-8 text-right font-mono text-xs text-slate-400">
                  {Math.round(trackingStats.avgConfidence * 100)}%
                </span>
              </div>
            )}
          </div>
          {/* Animated corner brackets */}
          <div className="absolute -top-1 -left-1 h-3 w-3 border-t-2 border-l-2 border-cyan-500/50" />
          <div className="absolute -top-1 -right-1 h-3 w-3 border-t-2 border-r-2 border-cyan-500/50" />
          <div className="absolute -bottom-1 -left-1 h-3 w-3 border-b-2 border-l-2 border-cyan-500/50" />
          <div className="absolute -right-1 -bottom-1 h-3 w-3 border-r-2 border-b-2 border-cyan-500/50" />
        </div>

        {/* Right: Connection status */}
        <div className="rounded-lg border border-slate-700/50 bg-slate-900/90 px-4 py-3 backdrop-blur-md">
          <div className="flex items-center gap-3">
            {/* Animated pulse indicator */}
            <div className="relative">
              <div
                className="h-3 w-3 rounded-full"
                style={{
                  backgroundColor: statusColor,
                  boxShadow: `0 0 12px ${statusColor}`,
                }}
              />
              {isConnected && !isStale && (
                <div
                  className="absolute inset-0 animate-ping rounded-full opacity-75"
                  style={{ backgroundColor: statusColor }}
                />
              )}
            </div>
            <div>
              <div className="font-medium text-slate-200">
                {isConnected && !isStale ? "LIVE" : isStale ? "STALE" : "OFFLINE"}
              </div>
              <div className="font-mono text-xs text-slate-500">
                {frameRate > 0 ? `${frameRate.toFixed(1)} Hz` : "-- Hz"}
              </div>
            </div>
          </div>
          {worldState && (
            <div className="mt-2 border-t border-slate-800 pt-2">
              <div className="font-mono text-xs text-slate-600">
                Frame #{frameNumber.toLocaleString()}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Tracking source breakdown - center top */}
      {occupantCount > 0 && (
        <div className="absolute top-4 left-1/2 -translate-x-1/2">
          <div className="flex items-center gap-4 rounded-full border border-slate-700/50 bg-slate-900/80 px-4 py-2 backdrop-blur-md">
            {trackingStats.fused > 0 && (
              <div className="flex items-center gap-1.5">
                <div className="h-2 w-2 rounded-full bg-cyan-500 shadow-[0_0_6px_rgba(6,182,212,0.5)]" />
                <span className="font-mono text-sm text-cyan-400">{trackingStats.fused}</span>
              </div>
            )}
            {trackingStats.radar > 0 && (
              <div className="flex items-center gap-1.5">
                <div className="h-2 w-2 rounded-full bg-blue-500 shadow-[0_0_6px_rgba(59,130,246,0.5)]" />
                <span className="font-mono text-sm text-blue-400">{trackingStats.radar}</span>
              </div>
            )}
            {trackingStats.vision > 0 && (
              <div className="flex items-center gap-1.5">
                <div className="h-2 w-2 rounded-full bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]" />
                <span className="font-mono text-sm text-green-400">{trackingStats.vision}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Bottom left: Room info */}
      {roomPlan && (
        <div className="absolute bottom-4 left-4">
          <div className="rounded-lg border border-slate-700/50 bg-slate-900/80 px-3 py-2 backdrop-blur-md">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2 text-xs">
                <span className="text-slate-500">ROOM</span>
                <span className="font-mono text-slate-300">
                  {roomPlan.roomWidth.toFixed(1)} × {roomPlan.roomHeight.toFixed(1)}m
                </span>
                {!roomPlan.configured && (
                  <span className="text-[10px] text-amber-500">(defaults)</span>
                )}
              </div>
              {hasCapturedRoom && (
                <div className="flex items-center gap-2">
                  <span className="rounded bg-blue-500/20 px-1.5 py-0.5 text-[10px] font-medium text-blue-400">
                    3D SCAN
                  </span>
                  <span className="font-mono text-[10px] text-slate-500">
                    {surfaceCount} surfaces · {objectCount} objects
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Legend - bottom right */}
      <div className="absolute right-4 bottom-4">
        <div className="rounded-lg border border-slate-700/50 bg-slate-900/80 px-3 py-2 backdrop-blur-md">
          <div className="flex flex-col gap-1.5 text-xs">
            <div className="mb-1 text-[10px] font-medium tracking-wider text-slate-500">
              TRACKING SOURCE
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-cyan-500" />
              <span className="text-slate-400">Fused (Radar + Vision)</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-blue-500" />
              <span className="text-slate-400">Radar only</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-green-500" />
              <span className="text-slate-400">Vision only</span>
            </div>

            <div className="my-1 border-t border-slate-800" />

            <div className="text-[10px] font-medium tracking-wider text-slate-500">INDICATORS</div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-4 rounded bg-amber-500/50" />
              <span className="text-slate-400">Prediction path</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full border border-slate-500" />
              <span className="text-slate-400">Uncertainty halo</span>
            </div>

            {/* Room geometry legend when CapturedRoom is active */}
            {hasCapturedRoom && (
              <>
                <div className="my-1 border-t border-slate-800" />
                <div className="text-[10px] font-medium tracking-wider text-slate-500">
                  ROOM GEOMETRY
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded bg-slate-600" />
                  <span className="text-slate-400">Walls</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded bg-orange-500" />
                  <span className="text-slate-400">Doors</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded bg-sky-400" />
                  <span className="text-slate-400">Windows</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded bg-violet-500" />
                  <span className="text-slate-400">Furniture</span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Center: OpticWorks branding */}
      <div className="absolute bottom-4 left-1/2 -translate-x-1/2">
        <div className="text-center">
          <div className="text-[10px] tracking-[0.3em] text-slate-600">OPTICWORKS</div>
          <div className="font-mono text-xs text-slate-700">RS-1</div>
        </div>
      </div>
    </div>
  );
}
