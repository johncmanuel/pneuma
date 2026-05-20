import type { WSEventPayloads, WSEventType } from "./ws-events";

export type WSMessage<Type extends WSEventType = WSEventType> = {
  type: Type;
  payload: WSEventPayloads[Type];
};

export type WSSend = <Type extends WSEventType>(
  type: Type,
  payload: WSEventPayloads[Type]
) => void;

export type WSOnMessage = (message: WSMessage) => void;

export type WSOnOpen = () => void;
export type WSOnClose = (event: CloseEvent) => void;
export type WSOnError = (event: Event) => void;

export interface WSClientOptions {
  buildUrl: () => string;
  shouldReconnect: () => boolean;
  reconnectDelayMs?: number;
  onMessage: WSOnMessage;
  onOpen?: WSOnOpen;
  onClose?: WSOnClose;
  onError?: WSOnError;
  onParseError?: (error: unknown, raw: string) => void;
}

export interface WSClient {
  connect: () => void;
  disconnect: () => void;
  send: WSSend;
  close: () => void;
  getSocket: () => WebSocket | null;
}

export function createWSClient(options: WSClientOptions): WSClient {
  let socket: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const reconnectDelayMs = options.reconnectDelayMs ?? 3000;

  function clearReconnectTimer() {
    if (reconnectTimer) clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }

  function close() {
    clearReconnectTimer();
    socket?.close();
    socket = null;
  }

  function connect() {
    if (
      socket &&
      (socket.readyState === WebSocket.OPEN ||
        socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    if (socket) {
      try {
        socket.close();
      } catch {
        console.warn("Failed to close existing WebSocket");
      }
      socket = null;
    }

    const url = options.buildUrl();
    if (!url || !options.shouldReconnect()) return;

    const ws = new WebSocket(url);
    socket = ws;

    ws.onopen = () => {
      options.onOpen?.();
    };

    ws.onmessage = (e) => {
      const raw = String(e.data ?? "");
      try {
        const msg = JSON.parse(raw) as WSMessage;
        options.onMessage(msg);
      } catch (err) {
        if (options.onParseError) {
          options.onParseError(err, raw);
        } else {
          console.warn("Failed to parse WebSocket message");
        }
      }
    };

    ws.onclose = (event) => {
      if (socket !== ws) return;
      socket = null;
      options.onClose?.(event);
      if (options.shouldReconnect()) {
        reconnectTimer = setTimeout(connect, reconnectDelayMs);
      }
    };

    ws.onerror = (event) => {
      options.onError?.(event);
      ws.close();
    };
  }

  function disconnect() {
    close();
  }

  const send: WSSend = (type, payload) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type, payload }));
    }
  };

  return {
    connect,
    disconnect,
    send,
    close,
    getSocket: () => socket
  };
}
