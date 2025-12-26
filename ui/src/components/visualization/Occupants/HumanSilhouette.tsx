/**
 * Human silhouette mesh for tracked occupants - Tesla FSD style
 *
 * Features:
 * - Motion trails with fading history
 * - Prediction cones showing projected movement
 * - Uncertainty halos based on tracking confidence
 * - Color coded by sensor source:
 *   - Cyan (#06b6d4): Fused (both radar + vision)
 *   - Blue (#3b82f6): Radar only
 *   - Green (#22c55e): Vision only
 */
/* eslint-disable react/no-unknown-property */
import { useRef, useMemo, useEffect, useState } from "react";
import { useFrame } from "@react-three/fiber";
import { Line } from "@react-three/drei";
import * as THREE from "three";
import type { Group, Mesh } from "three";
import type { TrackedObject } from "@/types/worldstate";

// Tesla FSD-inspired color palette
const COLORS = {
  fused: "#06b6d4", // Cyan - highest confidence
  radarOnly: "#3b82f6", // Blue
  visionOnly: "#22c55e", // Green
  prediction: "#f59e0b", // Amber for prediction cone
  trail: "#06b6d4", // Trail color (will be tinted by source)
  uncertainty: "#ef4444", // Red for low confidence
};

// Motion trail settings
const TRAIL_LENGTH = 20; // Number of historical positions to keep
const TRAIL_UPDATE_INTERVAL = 100; // ms between trail point captures

// Prediction settings
const PREDICTION_TIME = 0.75; // seconds to project ahead

interface HumanSilhouetteProps {
  object: TrackedObject;
}

interface TrailPoint {
  x: number;
  z: number;
  timestamp: number;
}

export function HumanSilhouette({ object }: HumanSilhouetteProps) {
  const groupRef = useRef<Group>(null);
  const pulseRef = useRef<Mesh>(null);
  const scanLineRef = useRef<Mesh>(null);
  const uncertaintyRef = useRef<Mesh>(null);
  const trailRef = useRef<TrailPoint[]>([]);
  const lastTrailUpdate = useRef<number>(0);

  // Determine color based on source
  const color = useMemo(() => {
    if (object.is_radar_only) return COLORS.radarOnly;
    if (object.is_vision_only) return COLORS.visionOnly;
    return COLORS.fused;
  }, [object.is_radar_only, object.is_vision_only]);

  // Parse color for Three.js
  const threeColor = useMemo(() => new THREE.Color(color), [color]);

  // Position: x/y from WorldState map to Three.js x/z (Y is up in Three.js)
  const position: [number, number, number] = [object.x, 0, object.y];

  // Update motion trail
  useEffect(() => {
    const now = Date.now();
    if (now - lastTrailUpdate.current > TRAIL_UPDATE_INTERVAL) {
      trailRef.current.push({
        x: object.x,
        z: object.y,
        timestamp: now,
      });
      // Keep only recent trail points
      if (trailRef.current.length > TRAIL_LENGTH) {
        trailRef.current.shift();
      }
      lastTrailUpdate.current = now;
    }
  }, [object.x, object.y]);

  // Trail points for rendering - store in state to trigger re-renders
  const [trailPoints, setTrailPoints] = useState<[number, number, number][]>([]);

  // Update trail points when position changes
  useEffect(() => {
    const points: [number, number, number][] = trailRef.current.map((p, i) => [
      p.x,
      0.05 + i * 0.002, // Slight elevation to prevent z-fighting
      p.z,
    ]);
    // Add current position
    points.push([object.x, 0.05 + points.length * 0.002, object.y]);
    setTrailPoints(points);
  }, [object.x, object.y]);

  // Prediction point (where occupant will be in PREDICTION_TIME seconds)
  const predictionPoint = useMemo((): [number, number, number] => {
    return [object.x + object.vx * PREDICTION_TIME, 0.1, object.y + object.vy * PREDICTION_TIME];
  }, [object.x, object.y, object.vx, object.vy]);

  // Heading arrow points (enhanced)
  const headingPoints = useMemo(() => {
    const length = 0.6 + object.speed * 0.3; // Length scales with speed
    const rad = (object.heading * Math.PI) / 180;
    return [[0, 0.15, 0] as const, [Math.sin(rad) * length, 0.15, Math.cos(rad) * length] as const];
  }, [object.heading, object.speed]);

  // Uncertainty radius based on inverse confidence
  const uncertaintyRadius = useMemo(() => {
    return 0.3 + (1 - object.confidence) * 0.5; // 0.3m at 100% confidence, 0.8m at 0%
  }, [object.confidence]);

  // Emissive intensity based on confidence
  const emissiveIntensity = useMemo(() => {
    return 0.15 + object.confidence * 0.35;
  }, [object.confidence]);

  // Animation loop
  useFrame(state => {
    const time = state.clock.elapsedTime;

    // Pulse animation
    if (pulseRef.current) {
      const scale = 1 + Math.sin(time * 3) * 0.08;
      pulseRef.current.scale.setScalar(scale);
    }

    // Scanning line rotation (Tesla FSD signature effect)
    if (scanLineRef.current) {
      scanLineRef.current.rotation.y = time * 2;
    }

    // Uncertainty halo pulsing (faster pulse = lower confidence)
    if (uncertaintyRef.current) {
      const pulseSpeed = 2 + (1 - object.confidence) * 4; // Faster when less confident
      const opacity = 0.1 + Math.sin(time * pulseSpeed) * 0.05;
      (uncertaintyRef.current.material as THREE.MeshBasicMaterial).opacity = opacity;
    }
  });

  return (
    <group ref={groupRef} position={position}>
      {/* === MOTION TRAIL === */}
      {trailPoints.length > 2 && (
        <Line
          points={trailPoints}
          color={color}
          lineWidth={2}
          transparent
          opacity={0.4}
          // Vertex colors for fading effect would go here
        />
      )}

      {/* === UNCERTAINTY HALO === */}
      <mesh ref={uncertaintyRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.01, 0]}>
        <ringGeometry args={[uncertaintyRadius - 0.05, uncertaintyRadius, 64]} />
        <meshBasicMaterial
          color={object.confidence < 0.5 ? COLORS.uncertainty : color}
          transparent
          opacity={0.15}
          side={THREE.DoubleSide}
        />
      </mesh>

      {/* === SCANNING LINE (Tesla FSD signature) === */}
      <group ref={scanLineRef} position={[0, 0.03, 0]}>
        <mesh rotation={[-Math.PI / 2, 0, 0]}>
          <planeGeometry args={[uncertaintyRadius * 2, 0.02]} />
          <meshBasicMaterial color={color} transparent opacity={0.6} side={THREE.DoubleSide} />
        </mesh>
      </group>

      {/* === BASE RING === */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.02, 0]}>
        <ringGeometry args={[0.28, 0.35, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.8} />
      </mesh>

      {/* === PULSE RING === */}
      <mesh ref={pulseRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.015, 0]}>
        <ringGeometry args={[0.35, 0.42, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.4} />
      </mesh>

      {/* === BODY WIREFRAME (Tesla FSD style) === */}
      <mesh position={[0, 1.0, 0]}>
        <capsuleGeometry args={[0.22, 0.85, 4, 12]} />
        <meshStandardMaterial
          color={color}
          transparent
          opacity={0.7}
          emissive={threeColor}
          emissiveIntensity={emissiveIntensity}
          wireframe={false}
        />
      </mesh>
      {/* Body wireframe overlay */}
      <mesh position={[0, 1.0, 0]}>
        <capsuleGeometry args={[0.24, 0.87, 4, 12]} />
        <meshBasicMaterial color={color} wireframe transparent opacity={0.5} />
      </mesh>

      {/* === HEAD === */}
      <mesh position={[0, 1.75, 0]}>
        <sphereGeometry args={[0.18, 12, 12]} />
        <meshStandardMaterial
          color={color}
          transparent
          opacity={0.85}
          emissive={threeColor}
          emissiveIntensity={emissiveIntensity + 0.1}
        />
      </mesh>
      {/* Head wireframe overlay */}
      <mesh position={[0, 1.75, 0]}>
        <sphereGeometry args={[0.2, 12, 12]} />
        <meshBasicMaterial color={color} wireframe transparent opacity={0.4} />
      </mesh>

      {/* === HEADING DIRECTION ARROW === */}
      {object.speed > 0.08 && (
        <>
          <Line points={headingPoints} color={color} lineWidth={4} transparent opacity={0.9} />
          {/* Arrow head */}
          <mesh
            position={[
              Math.sin((object.heading * Math.PI) / 180) * (0.6 + object.speed * 0.3),
              0.15,
              Math.cos((object.heading * Math.PI) / 180) * (0.6 + object.speed * 0.3),
            ]}
            rotation={[-Math.PI / 2, 0, -((object.heading * Math.PI) / 180) + Math.PI / 2]}
          >
            <coneGeometry args={[0.08, 0.15, 8]} />
            <meshBasicMaterial color={color} transparent opacity={0.9} />
          </mesh>
        </>
      )}

      {/* === VELOCITY VECTOR === */}
      {object.speed > 0.15 && (
        <Line
          points={[
            [0, 1.0, 0],
            [object.vx * 0.6, 1.0, object.vy * 0.6],
          ]}
          color="#ffffff"
          lineWidth={2}
          transparent
          opacity={0.5}
          dashed
          dashSize={0.05}
          gapSize={0.03}
        />
      )}

      {/* === PREDICTION CONE === */}
      {object.speed > 0.2 && (
        <group>
          {/* Prediction line */}
          <Line
            points={[
              [0, 0.1, 0],
              [predictionPoint[0] - object.x, 0.1, predictionPoint[2] - object.y],
            ]}
            color={COLORS.prediction}
            lineWidth={2}
            transparent
            opacity={0.5}
            dashed
            dashSize={0.1}
            gapSize={0.05}
          />
          {/* Prediction target marker */}
          <mesh
            position={[predictionPoint[0] - object.x, 0.05, predictionPoint[2] - object.y]}
            rotation={[-Math.PI / 2, 0, 0]}
          >
            <ringGeometry args={[0.15, 0.2, 16]} />
            <meshBasicMaterial color={COLORS.prediction} transparent opacity={0.4} />
          </mesh>
          {/* Prediction uncertainty cone */}
          <mesh
            position={[
              (predictionPoint[0] - object.x) / 2,
              0.02,
              (predictionPoint[2] - object.y) / 2,
            ]}
            rotation={[-Math.PI / 2, 0, -Math.atan2(object.vx, object.vy)]}
          >
            <planeGeometry
              args={[
                0.3 + (1 - object.confidence) * 0.4, // Width based on uncertainty
                Math.sqrt(
                  Math.pow(predictionPoint[0] - object.x, 2) +
                    Math.pow(predictionPoint[2] - object.y, 2),
                ),
              ]}
            />
            <meshBasicMaterial
              color={COLORS.prediction}
              transparent
              opacity={0.15}
              side={THREE.DoubleSide}
            />
          </mesh>
        </group>
      )}

      {/* === CONFIDENCE INDICATOR (vertical bar) === */}
      <mesh position={[-0.4, 0.1 + object.confidence * 0.4, 0]}>
        <boxGeometry args={[0.03, object.confidence * 0.8, 0.03]} />
        <meshBasicMaterial
          color={
            object.confidence > 0.7
              ? color
              : object.confidence > 0.4
                ? COLORS.prediction
                : COLORS.uncertainty
          }
          transparent
          opacity={0.7}
        />
      </mesh>
      {/* Confidence bar outline */}
      <mesh position={[-0.4, 0.5, 0]}>
        <boxGeometry args={[0.04, 0.82, 0.04]} />
        <meshBasicMaterial color="#333" wireframe transparent opacity={0.3} />
      </mesh>

      {/* === TRACK ID INDICATOR (small sphere at top) === */}
      <mesh position={[0, 2.0, 0]}>
        <sphereGeometry args={[0.05, 8, 8]} />
        <meshBasicMaterial color={color} />
      </mesh>
    </group>
  );
}
