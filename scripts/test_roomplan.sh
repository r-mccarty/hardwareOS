#!/bin/bash
# Upload RoomPlan fixture to RS-1 and verify
#
# Usage:
#   ./scripts/test_roomplan.sh <device-ip> [fixture.json]
#
# Examples:
#   ./scripts/test_roomplan.sh 192.168.1.100
#   ./scripts/test_roomplan.sh 192.168.1.100 testdata/roomplan/real_scans/office.json

set -e

DEVICE_IP="${1:-192.168.1.100}"
FIXTURE="${2:-testdata/roomplan/fixtures/simple_rectangle.json}"

if [ ! -f "$FIXTURE" ]; then
    echo "Error: Fixture file not found: $FIXTURE"
    exit 1
fi

echo "=== RoomPlan Integration Test ==="
echo "Device: $DEVICE_IP"
echo "Fixture: $FIXTURE"
echo ""

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed"
    exit 1
fi

# Check if device is reachable
echo "Checking device connectivity..."
if ! curl -s --connect-timeout 5 "http://$DEVICE_IP/api/health" > /dev/null 2>&1; then
    echo "Warning: Device may not be reachable at $DEVICE_IP"
fi

# Parse CapturedRoom and create RS-1 config
echo "Parsing CapturedRoom fixture..."

# Calculate room bounds from walls
CONFIG=$(cat "$FIXTURE" | jq '
  # Extract wall surfaces
  .surfaces as $surfaces |
  ($surfaces | map(select(.category == "wall"))) as $walls |

  # Calculate bounds from wall transforms (position is in columns 12,13,14 for x,y,z)
  ($walls | map(.transform[12]) | min) as $minX |
  ($walls | map(.transform[12]) | max) as $maxX |
  ($walls | map(.transform[14]) | min) as $minZ |
  ($walls | map(.transform[14]) | max) as $maxZ |

  # Add wall dimensions to get full extent
  ($walls | map(.dimensions[0]) | max // 0) as $maxWallWidth |

  # Compute room dimensions
  (($maxX - $minX) + $maxWallWidth) as $roomWidth |
  (($maxZ - $minZ) + $maxWallWidth) as $roomHeight |

  # Center of room for default sensor position
  (($minX + $maxX) / 2) as $centerX |
  (($minZ + $maxZ) / 2) as $centerZ |

  # Build RS-1 config
  {
    room_width: (if $roomWidth > 0 then $roomWidth else 5.0 end),
    room_height: (if $roomHeight > 0 then $roomHeight else 4.0 end),
    sensor_pose: {
      m: [1,0,0,0, 0,1,0,0, 0,0,1,0, $centerX,0,$centerZ,1]
    },
    captured_room: .
  }
')

echo "Calculated config:"
echo "$CONFIG" | jq '{room_width, room_height, sensor_pose}'
echo ""

# Upload to RS-1
echo "Uploading to RS-1..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://$DEVICE_IP/api/setup/roomplan" \
  -H "Content-Type: application/json" \
  -d "$CONFIG")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "Response (HTTP $HTTP_CODE):"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
echo ""

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: Upload failed with status $HTTP_CODE"
    exit 1
fi

# Verify configuration was applied
echo "Verifying configuration..."
sleep 0.5

VERIFY=$(curl -s "http://$DEVICE_IP/api/setup/roomplan")
echo "Current configuration:"
echo "$VERIFY" | jq '{configured, room_width, room_height, last_updated}' 2>/dev/null || echo "$VERIFY"
echo ""

# Check if CapturedRoom is present
HAS_CAPTURED=$(echo "$VERIFY" | jq 'has("captured_room")' 2>/dev/null || echo "false")
if [ "$HAS_CAPTURED" == "true" ]; then
    SURFACE_COUNT=$(echo "$VERIFY" | jq '.captured_room.surfaces | length' 2>/dev/null || echo "0")
    OBJECT_COUNT=$(echo "$VERIFY" | jq '.captured_room.objects | length' 2>/dev/null || echo "0")
    echo "CapturedRoom: $SURFACE_COUNT surfaces, $OBJECT_COUNT objects"
else
    echo "Warning: CapturedRoom data not found in response"
fi

echo ""
echo "=== Test Complete ==="
echo "Open http://$DEVICE_IP to view 3D visualization"
