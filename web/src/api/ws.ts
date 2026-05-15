// Multiplexed WebSocket client. The server upgrades at
// /api/v1/applications/{name}/events; on the same socket the client can
// subscribe/unsubscribe to additional topics via simple JSON messages.

import type { WSMessage } from "./types";

export interface AppEventsConn {
  close(): void;
  subscribe(topic: string): void;
  unsubscribe(topic: string): void;
}

export function openAppEvents(
  name: string,
  token: string | undefined,
  onMessage: (m: WSMessage) => void,
  onError?: (e: unknown) => void,
): AppEventsConn {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const qs = token ? `?token=${encodeURIComponent(token)}` : "";
  const url = `${proto}://${location.host}/api/v1/applications/${name}/events${qs}`;
  let ws: WebSocket | null = new WebSocket(url);

  ws.onmessage = (e) => {
    try {
      onMessage(JSON.parse(e.data));
    } catch (err) {
      onError?.(err);
    }
  };
  ws.onerror = (e) => onError?.(e);

  return {
    close: () => ws?.close(),
    subscribe: (topic) => ws?.send(JSON.stringify({ action: "subscribe", topic })),
    unsubscribe: (topic) => ws?.send(JSON.stringify({ action: "unsubscribe", topic })),
  };
}
