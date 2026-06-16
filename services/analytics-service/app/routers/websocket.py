"""
WebSocket Router — Real-time analytics endpoints

Endpoints:
  WS /ws/analytics/{short_code}  → Room for a specific short link
  WS /ws/analytics/global        → Global overview (all events)
  GET /ws/stats                  → Connection pool stats
"""
import logging
from fastapi import APIRouter, WebSocket, WebSocketDisconnect
from app.websocket_manager import ws_manager

logger = logging.getLogger(__name__)
router = APIRouter()


@router.websocket("/analytics/{short_code}")
async def ws_analytics_room(ws: WebSocket, short_code: str):
    """
    Real-time feed for a specific short URL.
    Clients connecting here receive every click event for that short_code.
    """
    await ws_manager.connect(ws, short_code=short_code)
    try:
        # Send initial connection confirmation
        await ws.send_json({
            "type":       "connected",
            "short_code": short_code,
            "message":    f"Listening for live clicks on sho.rt/{short_code}",
        })
        # Keep connection alive — wait for client disconnect
        while True:
            await ws.receive_text()
    except WebSocketDisconnect:
        await ws_manager.disconnect(ws, short_code=short_code)
        logger.info(f"WS client left room: {short_code}")


@router.websocket("/global")
async def ws_global_room(ws: WebSocket):
    """
    Real-time feed for ALL click events across the platform.
    Used by the main overview dashboard.
    """
    await ws_manager.connect(ws, short_code=None)
    try:
        await ws.send_json({
            "type":    "connected",
            "message": "Listening for all platform click events",
        })
        while True:
            await ws.receive_text()
    except WebSocketDisconnect:
        await ws_manager.disconnect(ws, short_code=None)
        logger.info("WS client left global room")


@router.get("/stats")
async def ws_stats():
    """Returns current WebSocket connection pool statistics."""
    return ws_manager.room_count()
