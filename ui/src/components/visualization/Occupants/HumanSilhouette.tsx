/**
 * Human silhouette mesh for tracked occupants
 *
 * Color coded by sensor source:
 * - Cyan (#06b6d4): Fused (both radar + vision)
 * - Blue (#3b82f6): Radar only
 * - Green (#22c55e): Vision only
 */
/* eslint-disable react/no-unknown-property */
import { useRef, useMemo } from "react";
import { useFrame } from "@react-three/fiber";
import { Line } from "@react-three/drei";
import type { Group, Mesh } from "three";
import type { TrackedObject } from "@/types/worldstate";

// Theme colors
const COLORS = {
  fused: "#06b6d4",
  radarOnly: "#3b82f6",
  visionOnly: "#22c55e",
};

interface HumanSilhouetteProps {
  object: TrackedObject;
}

export function HumanSilhouette({ object }: HumanSilhouetteProps) {
  const groupRef = useRef<Group>(null);
  const pulseRef = useRef<Mesh>(null);

  // Determine color based on source
  const color = useMemo(() => {
    if (object.is_radar_only) return COLORS.radarOnly;
    if (object.is_vision_only) return COLORS.visionOnly;
    return COLORS.fused;
  }, [object.is_radar_only, object.is_vision_only]);

  // Heading arrow points
  const headingPoints = useMemo(() => {
    const length = 0.5;
    // Heading is degrees from +Y axis, convert to radians
    const rad = (object.heading * Math.PI) / 180;
    return [[0, 0.1, 0] as const, [Math.sin(rad) * length, 0.1, Math.cos(rad) * length] as const];
  }, [object.heading]);

  // Pulse animation based on confidence
  useFrame(state => {
    if (pulseRef.current) {
      const scale = 1 + Math.sin(state.clock.elapsedTime * 2) * 0.05 * object.confidence;
      pulseRef.current.scale.setScalar(scale);
    }
  });

  // Position: x/y from WorldState map to Three.js x/z (Y is up in Three.js)
  const position: [number, number, number] = [object.x, 0, object.y];

  return (
    <group ref={groupRef} position={position}>
      {/* Base indicator ring */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.02, 0]}>
        <ringGeometry args={[0.3, 0.4, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.6} />
      </mesh>

      {/* Pulse effect */}
      <mesh ref={pulseRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.01, 0]}>
        <ringGeometry args={[0.4, 0.45, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.3} />
      </mesh>

      {/* Body - Capsule */}
      <mesh position={[0, 1.0, 0]}>
        <capsuleGeometry args={[0.25, 0.8, 8, 16]} />
        <meshStandardMaterial
          color={color}
          transparent
          opacity={0.8}
          emissive={color}
          emissiveIntensity={0.2}
        />
      </mesh>

      {/* Head */}
      <mesh position={[0, 1.7, 0]}>
        <sphereGeometry args={[0.2, 16, 16]} />
        <meshStandardMaterial
          color={color}
          transparent
          opacity={0.9}
          emissive={color}
          emissiveIntensity={0.3}
        />
      </mesh>

      {/* Heading direction arrow */}
      {object.speed > 0.1 && (
        <Line points={headingPoints} color={color} lineWidth={3} transparent opacity={0.8} />
      )}

      {/* Velocity vector */}
      {object.speed > 0.2 && (
        <Line
          points={[
            [0, 1.0, 0],
            [object.vx * 0.5, 1.0, object.vy * 0.5],
          ]}
          color="#ffffff"
          lineWidth={1}
          transparent
          opacity={0.4}
        />
      )}
    </group>
  );
}
