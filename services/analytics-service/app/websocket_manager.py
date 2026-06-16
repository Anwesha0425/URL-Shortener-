"""
WebSocket Manager — Real-time analytics push

Architecture:
  Kafka (url.clicked) → Analytics Consumer → WebSocket Manager → Browser Clients

The WebSocket manager maintains a connection pool per short_code.
When a new click event arrives, it broadcasts live stats to all
connected dashboard clients watching that short_code.

Connection lifecycle:
  Client connects    → subscribe to short_code room
  Click event fires  → compute live agg → broadcast to room
  Client disconnects → cleanup from room
"""

import json
import logging
from collections import defaultdict
from datetime import datetime, timezone
from typing import DefaultDict, Set

from fastapi import WebSocket

logger = logging.getLogger(__name__)


class WebSocketManager:
    """
    Manages WebSocket connections grouped by short_code (rooms).
    Thread-safe for asyncio event loop.
    """

    def __init__(self):
        # rooms: short_code → set of connected WebSocket clients
        self._rooms: DefaultDict[str, Set[WebSocket]] = defaultdict(set)
        # global room: receives ALL click events (for the overview dashboard)
        self._global: Set[WebSocket] = set()
        # live counters: short_code → click count in current window
        self._live_counts: DefaultDict[str, int] = defaultdict(int)

    async def connect(self, ws: WebSocket, short_code: str | None = None):
        await ws.accept()
        if short_code:
            self._rooms[short_code].add(ws)
            logger.info(f"WS client joined room: {short_code}")
        else:
            self._global.add(ws)
            logger.info("WS client joined global room")

    async def disconnect(self, ws: WebSocket, short_code: str | None = None):
        if short_code:
            self._rooms[short_code].discard(ws)
            if not self._rooms[short_code]:
                del self._rooms[short_code]
        else:
            self._global.discard(ws)

    async def broadcast_click(self, event: dict):
        """
        Called by the Kafka consumer for every click event.
        Broadcasts real-time stats to the relevant room and global room.
        """
        short_code = event.get("short_code", "")
        self._live_counts[short_code] += 1

        payload = {
            "type": "click_event",
            "short_code": short_code,
            "country": event.get("country", "Unknown"),
            "referrer": event.get("referrer", ""),
            "timestamp": event.get("timestamp", datetime.now(timezone.utc).isoformat()),
            "live_count": self._live_counts[short_code],
        }

        # Broadcast to specific room
        await self._broadcast_to_room(short_code, payload)

        # Broadcast to global overview room
        await self._broadcast_to_set(self._global, payload)

    async def _broadcast_to_room(self, short_code: str, payload: dict):
        await self._broadcast_to_set(self._rooms.get(short_code, set()), payload)

    async def _broadcast_to_set(self, connections: Set[WebSocket], payload: dict):
        dead: list[WebSocket] = []
        message = json.dumps(payload)

        for ws in connections:
            try:
                await ws.send_text(message)
            except Exception:
                dead.append(ws)

        for ws in dead:
            connections.discard(ws)

    def room_count(self) -> dict:
        return {
            "rooms": len(self._rooms),
            "global_clients": len(self._global),
            "total_clients": sum(len(s) for s in self._rooms.values())
            + len(self._global),
        }


# Singleton used across the app
ws_manager = WebSocketManager()
