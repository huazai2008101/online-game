import type { GameClientOptions, ConnectionStatus } from './types';

/**
 * Manages the WebSocket connection with reconnection and heartbeat.
 */
export class Connection {
  private ws: WebSocket | null = null;
  private status: ConnectionStatus = 'disconnected';
  private reconnectAttempts = 0;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private seq = 0;

  private url: string;
  private token: string;
  private opts: GameClientOptions;
  private onMessage: (data: any) => void;
  private onStatusChange: (status: ConnectionStatus) => void;

  constructor(
    opts: GameClientOptions,
    onMessage: (data: any) => void,
    onStatusChange: (status: ConnectionStatus) => void
  ) {
    this.opts = opts;
    this.token = opts.token || '';
    this.url = this.buildUrl(opts);
    this.onMessage = onMessage;
    this.onStatusChange = onStatusChange;
  }

  private buildUrl(opts: GameClientOptions): string {
    if (opts.serverUrl) return opts.serverUrl;

    // Build from URL params or defaults
    const params = new URLSearchParams(window.location.search);
    const token = opts.token || params.get('token') || '';
    const gameId = opts.gameId || params.get('gameId') || '';
    const roomId = opts.roomId || params.get('roomId') || '';

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${proto}//${host}/ws?token=${encodeURIComponent(token)}&gameId=${encodeURIComponent(gameId)}&roomId=${encodeURIComponent(roomId)}`;
  }

  connect(): void {
    if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
      return;
    }

    this.setStatus('connecting');

    try {
      this.ws = new WebSocket(this.url);
    } catch (e) {
      console.error('[Connection] Failed to create WebSocket:', e);
      this.handleClose();
      return;
    }

    this.ws.onopen = () => {
      this.setStatus('connected');
      this.reconnectAttempts = 0;
      this.startHeartbeat();
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data);
        this.onMessage(data);
      } catch (e) {
        console.error('[Connection] Failed to parse message:', e);
      }
    };

    this.ws.onclose = () => {
      this.stopHeartbeat();
      this.handleClose();
    };

    this.ws.onerror = () => {
      // onclose will fire after this
    };
  }

  send(type: string, data: any): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[Connection] Cannot send, not connected');
      return;
    }
    const msg = { type, data, seq: ++this.seq, ts: Date.now() };
    this.ws.send(JSON.stringify(msg));
  }

  disconnect(): void {
    this.stopReconnect();
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close(1000, 'client disconnect');
      this.ws = null;
    }
    this.setStatus('disconnected');
  }

  getStatus(): ConnectionStatus {
    return this.status;
  }

  private handleClose(): void {
    this.ws = null;
    this.setStatus('disconnected');

    if (this.opts.reconnect && this.reconnectAttempts < this.opts.maxReconnectAttempts) {
      this.setStatus('reconnecting');
      this.reconnectAttempts++;
      const delay = this.opts.reconnectInterval * this.reconnectAttempts;
      this.reconnectTimer = setTimeout(() => {
        this.connect();
      }, delay);
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      this.send('ping', { ts: Date.now() });
    }, this.opts.heartbeatInterval);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private stopReconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.reconnectAttempts = 0;
  }

  private setStatus(status: ConnectionStatus): void {
    if (this.status !== status) {
      this.status = status;
      this.onStatusChange(status);
    }
  }
}
