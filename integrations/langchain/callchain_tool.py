"""LangChain tool → callhook.

Exposes `fire_phone_call` as a LangChain tool so agents can place real
AI phone calls. Compatible with LangChain's tool/StructuredTool API.

Usage (classic):
    from callchain_tool import callhook_tool
    tools = [callhook_tool]

Usage (modern @tool decorator equivalent kept for compatibility):
    from callchain_tool import fire_call
    from langchain_core.tools import tool
    pydantic_fire = tool(fire_call)   # if you prefer to wrap it yourself
"""

import os
import time
from typing import Optional

import requests
from langchain_core.tools import StructuredTool
from pydantic import BaseModel, Field

BASE_URL = os.environ.get("CALLHOOK_BASE_URL", "http://localhost:8080")
TOKEN = os.environ.get("CALLHOOK_INTAKE_TOKEN", "")


class FireCallInput(BaseModel):
    event_type: str = Field(
        description=(
            "One of: invoice.due, account.warning, promo.offer, "
            "delivery.window, appointment.reminder, payment.failed, "
            "subscription.expiring, feedback.request"
        )
    )
    customer_id: str = Field(description="Customer to call, e.g. cus_1002")
    phone: Optional[str] = Field(
        default="", description="Optional E.164 phone override"
    )
    not_before: Optional[str] = Field(
        default="", description="Optional RFC3339 earliest call time"
    )


def fire_call(event_type: str, customer_id: str, phone: str = "",
              not_before: str = "") -> dict:
    """Place an AI phone call to a customer. Returns status + session id."""
    resp = requests.post(
        f"{BASE_URL.rstrip('/')}/api/events",
        headers={
            "Content-Type": "application/json",
            **({"Authorization": f"Bearer {TOKEN}"} if TOKEN else {}),
        },
        json={
            "id": f"lc_{customer_id}_{event_type}_{int(time.time() * 1000)}",
            "type": event_type,
            "customer_id": customer_id,
            **({"phone": phone} if phone else {}),
            **({"not_before": not_before} if not_before else {}),
        },
        timeout=10,
    )
    return resp.json()


callhook_tool = StructuredTool.from_function(
    func=fire_call,
    name="fire_phone_call",
    description=(
        "Place an AI phone call via callhook. Use for invoice follow-ups, "
        "lead qualification, appointment confirmations — anything where the "
        "customer should be reached by voice. Returns the call status and "
        "session id."
    ),
    args_schema=FireCallInput,
)
