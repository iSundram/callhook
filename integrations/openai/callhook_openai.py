"""OpenAI tool-calling → callhook.

Exposes `fire_phone_call` as a function-calling tool so GPT models can
place real AI phone calls. Works with the OpenAI Python SDK (v1.x).

Usage:
    from callhook_openai import tools, handle_tool_call
    # ... in your agent loop, pass `tools` to chat.completions.create;
    # when the model emits a tool call, dispatch via handle_tool_call().
"""

import os
import time

import requests

BASE_URL = os.environ.get("CALLHOOK_BASE_URL", "http://localhost:8080")
TOKEN = os.environ.get("CALLHOOK_INTAKE_TOKEN", "")

# The 8 callhook event types — mirrors backend/internal/events/router.go.
EVENT_TYPES = [
    "invoice.due", "account.warning", "promo.offer", "delivery.window",
    "appointment.reminder", "payment.failed", "subscription.expiring",
    "feedback.request",
]

tools = [{
    "type": "function",
    "function": {
        "name": "fire_phone_call",
        "description": (
            "Place an intelligent AI phone call to a customer via callhook. "
            "The voice agent calls them, handles the conversation with their "
            "account context, and returns a structured outcome."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "event_type": {
                    "type": "string",
                    "enum": EVENT_TYPES,
                    "description": "What the call is about.",
                },
                "customer_id": {
                    "type": "string",
                    "description": "Customer to call, e.g. cus_1002.",
                },
                "phone": {
                    "type": "string",
                    "description": "Optional E.164 phone override.",
                },
                "not_before": {
                    "type": "string",
                    "description": "Optional RFC3339 earliest call time.",
                },
            },
            "required": ["event_type", "customer_id"],
        },
    },
}]


def fire_phone_call(event_type: str, customer_id: str, phone: str = "",
                    not_before: str = "") -> dict:
    """Tool implementation: POST /api/events on the callhook server."""
    resp = requests.post(
        f"{BASE_URL.rstrip('/')}/api/events",
        headers={
            "Content-Type": "application/json",
            **({"Authorization": f"Bearer {TOKEN}"} if TOKEN else {}),
        },
        json={
            "id": f"openai_{customer_id}_{int(time.time() * 1000)}",
            "type": event_type,
            "customer_id": customer_id,
            **({"phone": phone} if phone else {}),
            **({"not_before": not_before} if not_before else {}),
        },
        timeout=10,
    )
    return resp.json()


def handle_tool_call(name: str, args: dict) -> dict:
    """Dispatch a model tool call; returns the JSON to send back as the
    tool result message content."""
    if name != "fire_phone_call":
        return {"error": f"unknown tool: {name}"}
    return fire_phone_call(**args)
