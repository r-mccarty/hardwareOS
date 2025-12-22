/**
 * Hook to subscribe to WorldState updates from WebRTC DataChannel
 *
 * The RS-1 device streams occupancy data at 30Hz via a WebRTC DataChannel
 * labeled "worldstate". This hook listens for that channel and updates
 * the Zustand store.
 */
import { useEffect, useRef } from "react";
import { useRTCStore, useWorldStateStore } from "./stores";
import type { WorldStateMessage } from "@/types/worldstate";

export function useWorldState() {
  const { peerConnection, peerConnectionState } = useRTCStore();
  const { setWorldState, setConnected } = useWorldStateStore();
  const channelRef = useRef<RTCDataChannel | null>(null);

  useEffect(() => {
    if (!peerConnection || peerConnectionState !== "connected") {
      setConnected(false);
      return;
    }

    // Listen for server-created data channels
    const handleDataChannel = (event: RTCDataChannelEvent) => {
      const channel = event.channel;

      if (channel.label !== "worldstate") {
        return;
      }

      console.log("[useWorldState] WorldState channel received");
      channelRef.current = channel;

      channel.onopen = () => {
        console.log("[useWorldState] WorldState channel opened");
        setConnected(true);
      };

      channel.onclose = () => {
        console.log("[useWorldState] WorldState channel closed");
        setConnected(false);
        channelRef.current = null;
      };

      channel.onerror = (ev: Event) => {
        console.error("[useWorldState] WorldState channel error:", ev);
        setConnected(false);
      };

      channel.onmessage = (event: MessageEvent) => {
        try {
          const message: WorldStateMessage = JSON.parse(event.data);
          setWorldState(message);
        } catch (err) {
          console.error("[useWorldState] Failed to parse WorldState message:", err);
        }
      };
    };

    peerConnection.addEventListener("datachannel", handleDataChannel);

    return () => {
      peerConnection.removeEventListener("datachannel", handleDataChannel);
      if (channelRef.current) {
        channelRef.current.close();
        channelRef.current = null;
      }
      setConnected(false);
    };
  }, [peerConnection, peerConnectionState, setWorldState, setConnected]);

  return useWorldStateStore();
}
