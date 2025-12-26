/**
 * Grid floor component with Tesla FSD-style radar sweep effect
 */
/* eslint-disable react/no-unknown-property */
import { useRef, useMemo, useEffect, useState } from "react";
import { useFrame } from "@react-three/fiber";
import { Grid } from "@react-three/drei";
import * as THREE from "three";
import { useRoomPlanStore } from "@/hooks/stores";

// Radar sweep shader for Tesla FSD-style scanning effect
const RadarSweepMaterial = {
  uniforms: {
    time: { value: 0 },
    color: { value: new THREE.Color("#2563eb") },
    sweepWidth: { value: 0.15 },
    fadeLength: { value: 0.3 },
  },
  vertexShader: `
    varying vec2 vUv;
    void main() {
      vUv = uv;
      gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
    }
  `,
  fragmentShader: `
    uniform float time;
    uniform vec3 color;
    uniform float sweepWidth;
    uniform float fadeLength;
    varying vec2 vUv;

    void main() {
      // Calculate angle from center
      vec2 centered = vUv - 0.5;
      float angle = atan(centered.y, centered.x);
      float normalizedAngle = (angle + 3.14159) / (2.0 * 3.14159);

      // Sweep position (0-1, rotating)
      float sweepPos = fract(time * 0.15);

      // Distance from sweep line
      float dist = abs(normalizedAngle - sweepPos);
      dist = min(dist, 1.0 - dist); // Handle wrap-around

      // Fade behind sweep
      float behindSweep = normalizedAngle - sweepPos;
      if (behindSweep < 0.0) behindSweep += 1.0;

      // Intensity based on distance from sweep
      float intensity = 0.0;
      if (dist < sweepWidth) {
        intensity = 1.0 - (dist / sweepWidth);
      }

      // Trailing fade
      if (behindSweep < fadeLength && behindSweep > 0.0) {
        float trailIntensity = (1.0 - behindSweep / fadeLength) * 0.5;
        intensity = max(intensity, trailIntensity);
      }

      // Distance from center for radial fade
      float distFromCenter = length(centered) * 2.0;
      float radialFade = smoothstep(1.0, 0.3, distFromCenter);

      float alpha = intensity * radialFade * 0.4;
      gl_FragColor = vec4(color, alpha);
    }
  `,
};

export function RoomFloor() {
  const { roomPlan } = useRoomPlanStore();
  const width = roomPlan?.roomWidth ?? 10;
  const height = roomPlan?.roomHeight ?? 8;

  const sweepRef = useRef<THREE.Mesh>(null);
  const pulseRingRef = useRef<THREE.Mesh>(null);

  // Create shader material once using lazy state initializer
  const [sweepMaterial] = useState(
    () =>
      new THREE.ShaderMaterial({
        uniforms: {
          time: { value: 0 },
          color: { value: new THREE.Color("#2563eb") },
          sweepWidth: { value: 0.08 },
          fadeLength: { value: 0.25 },
        },
        vertexShader: RadarSweepMaterial.vertexShader,
        fragmentShader: RadarSweepMaterial.fragmentShader,
        transparent: true,
        side: THREE.DoubleSide,
        depthWrite: false,
      }),
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      sweepMaterial.dispose();
    };
  }, [sweepMaterial]);

  // Animate radar sweep - update uniform via mesh ref
  useFrame(state => {
    if (sweepRef.current) {
      const material = sweepRef.current.material as THREE.ShaderMaterial;
      material.uniforms.time.value = state.clock.elapsedTime;
    }

    // Pulse ring expansion
    if (pulseRingRef.current) {
      const scale = 1 + (state.clock.elapsedTime % 3) / 3;
      pulseRingRef.current.scale.set(scale, scale, 1);
      const opacity = 0.3 * (1 - (state.clock.elapsedTime % 3) / 3);
      (pulseRingRef.current.material as THREE.MeshBasicMaterial).opacity = opacity;
    }
  });

  // Get sensor position for sweep origin
  const sensorPose = roomPlan?.sensorPose;
  const sensorPos = useMemo(() => {
    if (sensorPose && sensorPose.length >= 15) {
      return [sensorPose[12], sensorPose[14]] as const; // x, z from matrix
    }
    return [width / 2, height / 2] as const; // Default to center
  }, [sensorPose, width, height]);

  return (
    <group position={[width / 2, 0, height / 2]}>
      {/* Main grid - Tesla FSD style blue lines */}
      <Grid
        args={[width * 2, height * 2]}
        cellSize={0.5}
        cellThickness={0.6}
        cellColor="#1a365d"
        sectionSize={1}
        sectionThickness={1.2}
        sectionColor="#2563eb"
        fadeDistance={35}
        fadeStrength={1.2}
        infiniteGrid={false}
      />

      {/* Secondary finer grid for depth */}
      <Grid
        args={[width * 2, height * 2]}
        cellSize={0.25}
        cellThickness={0.3}
        cellColor="#0f172a"
        sectionSize={0.5}
        sectionThickness={0.4}
        sectionColor="#1e3a5f"
        fadeDistance={20}
        fadeStrength={1.5}
        infiniteGrid={false}
        position={[0, -0.001, 0]}
      />

      {/* Radar sweep overlay */}
      <mesh
        ref={sweepRef}
        rotation={[-Math.PI / 2, 0, 0]}
        position={[sensorPos[0] - width / 2, 0.02, sensorPos[1] - height / 2]}
        material={sweepMaterial}
      >
        <circleGeometry args={[Math.max(width, height) * 0.8, 64]} />
      </mesh>

      {/* Expanding pulse ring from sensor */}
      <mesh
        ref={pulseRingRef}
        rotation={[-Math.PI / 2, 0, 0]}
        position={[sensorPos[0] - width / 2, 0.015, sensorPos[1] - height / 2]}
      >
        <ringGeometry args={[2, 2.1, 64]} />
        <meshBasicMaterial color="#3b82f6" transparent opacity={0.3} side={THREE.DoubleSide} />
      </mesh>

      {/* Center crosshair at sensor position */}
      <group position={[sensorPos[0] - width / 2, 0.01, sensorPos[1] - height / 2]}>
        {/* Horizontal line */}
        <mesh rotation={[-Math.PI / 2, 0, 0]}>
          <planeGeometry args={[0.8, 0.01]} />
          <meshBasicMaterial color="#3b82f6" transparent opacity={0.5} />
        </mesh>
        {/* Vertical line */}
        <mesh rotation={[-Math.PI / 2, 0, Math.PI / 2]}>
          <planeGeometry args={[0.8, 0.01]} />
          <meshBasicMaterial color="#3b82f6" transparent opacity={0.5} />
        </mesh>
        {/* Center dot */}
        <mesh rotation={[-Math.PI / 2, 0, 0]}>
          <circleGeometry args={[0.05, 16]} />
          <meshBasicMaterial color="#60a5fa" />
        </mesh>
      </group>

      {/* Distance rings from sensor */}
      {[2, 4, 6].map(radius => (
        <mesh
          key={radius}
          rotation={[-Math.PI / 2, 0, 0]}
          position={[sensorPos[0] - width / 2, 0.005, sensorPos[1] - height / 2]}
        >
          <ringGeometry args={[radius - 0.02, radius, 64]} />
          <meshBasicMaterial color="#1e40af" transparent opacity={0.2} side={THREE.DoubleSide} />
        </mesh>
      ))}

      {/* Subtle floor plane for ambient occlusion effect */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.02, 0]} receiveShadow>
        <planeGeometry args={[width, height]} />
        <meshStandardMaterial color="#050510" transparent opacity={0.8} />
      </mesh>

      {/* Edge glow */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.001, 0]}>
        <ringGeometry args={[Math.max(width, height) * 0.48, Math.max(width, height) * 0.52, 64]} />
        <meshBasicMaterial color="#1e40af" transparent opacity={0.15} side={THREE.DoubleSide} />
      </mesh>
    </group>
  );
}
