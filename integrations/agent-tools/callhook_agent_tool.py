"""AutoGen / CrewAI tool → callhook.

Works with both frameworks (both accept plain callables or wrapped tools):

AutoGen:
    from callhook_agent_tool import fire_phone_call
    assistant = AssistantAgent("caller", ...)
    user_proxy.register_function({"fire_phone_call": fire_phone_call})

CrewAI:
    from crewai.tools import tool
    from callhook_agent_tool import fire_call
    @tool("FirePhoneCall")
    def make_call(customer_id: str, event_type: str) -> str:
        return fire_call(event_type, customer_id)
"""

import os
import time

import requests

BASE_URL = os.environ.get("CALLHOOK_BASE_URL", "http://localhost:8080")
TOKEN = os.environ.get("CALLHOOK_INTAKE_TOKEN", "")


def fire_call(event_type: str, customer_id: str, phone: str = "") -> str:
    """Place an AI phone call; returns a readable status line."""
    resp = requests.post(
        f"{BASE_URL.rstrip('/')}/api/events",
        headers={
            "Content-Type": "application/json",
            **({"Authorization": f"Bearer {TOKEN}"} if TOKEN else {}),
        },
        json={
            "id": f"agent_{customer_id}_{event_type}_{int(time.time() * 1000)}",
            "type": event_type,
            "customer_id": customer_id,
            **({"phone": phone} if phone else {}),
        },
        timeout=10,
    )
    result = resp.json()
    return (
        f"Call placed: session={result.get('session_id')}, "
        f"status={result.get('status')}"
    )


# AutoGen-style registration name.
fire_phone_call = fire_call
