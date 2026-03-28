/**
 * GameClient base class for browser game developers.
 *
 * Extend this class and override lifecycle hooks to build your game UI.
 *
 * Usage:
 *   import { GameClient } from '@gameplatform/client-sdk'
 *   class MyGameUI extends GameClient {
 *     onGameStart(data) { this.renderBoard(data.state) }
 *     onStateUpdate(state) { this.updateBoard(state) }
 *   }
 *   const game = new MyGameUI({ autoConnect: true })
 */

import { EventEmitter } from './EventEmitter';
import { Connection } from './Connection';
import type {
  GameClientOptions,
  GameConfig,
  RoomInfo,
  PlayerInfo,
  GameStartData,
  GameResults,
  GameError,
  ConnectionStatus,
} from './types';

const defaultOptions: GameClientOptions = {
  autoConnect: true,
  reconnect: true,
  reconnectInterval: 3000,
  maxReconnectAttempts: 5,
  heartbeatInterval: 30000,
};

export abstract class GameClient {
  protected events: EventEmitter;
  private conn: Connection;
  private opts: GameClientOptions;

  // Cached state
  private _state: Record<string, any> = {};
  private _roomInfo: RoomInfo | null = null;
  private _config: GameConfig | null = null;
  private _playerId: string = '';

  constructor(options: Partial<GameClientOptions> = {}) {
    this.opts = { ...defaultOptions, ...options };
    this.events = new EventEmitter();
    this.conn = new Connection(
      this.opts,
      (data) => this.handleMessage(data),
      (status) => this.handleStatusChange(status)
    );

    this.setupInternalListeners();

    if (this.opts.autoConnect) {
      this.connect();
    }
  }

  // ========== Player Actions ==========

  sendAction(action: string, data: any = {}): void {
    this.conn.send('action', { action, data });
  }

  ready(): void {
    this.conn.send('ready', {});
  }

  chat(message: string): void {
    this.conn.send('chat', { message });
  }

  // ========== State Queries ==========

  getState(): Record<string, any> {
    return this._state;
  }

  getRoomInfo(): RoomInfo | null {
    return this._roomInfo;
  }

  getMyPlayerId(): string {
    return this._playerId;
  }

  getStatus(): ConnectionStatus {
    return this.conn.getStatus();
  }

  // ========== Connection ==========

  connect(): void {
    this.conn.connect();
  }

  disconnect(): void {
    this.conn.disconnect();
  }

  // ========== Audio (optional, override in subclass) ==========

  playSound(_name: string): void {}
  playBGM(_name: string): void {}
  stopBGM(): void {}

  // ========== Logging ==========

  log(...args: any[]): void {
    console.log('[Game]', ...args);
  }

  warn(...args: any[]): void {
    console.warn('[Game]', ...args);
  }

  error(...args: any[]): void {
    console.error('[Game]', ...args);
  }

  // ========== Lifecycle Hooks (override in subclass) ==========

  protected onInit(_config: GameConfig): void {}
  protected onRoomJoined(_roomInfo: RoomInfo): void {}
  protected onPlayerJoin(_player: PlayerInfo): void {}
  protected onPlayerLeave(_playerId: string, _reason: string): void {}
  protected onPlayerReady(_playerId: string): void {}
  protected onAllReady(_playerCount: number): void {}
  protected onGameStart(_data: GameStartData): void {}
  protected onStateUpdate(_state: any): void {}
  protected onPrivateMessage(_event: string, _data: any): void {}
  protected onPlayerAction(_playerId: string, _action: string, _data: any): void {}
  protected onChat(_from: string, _message: string, _nickname: string): void {}
  protected onGameEnd(_results: GameResults): void {}
  protected onError(_error: GameError): void {}
  protected onDisconnect(_reason: string): void {}
  protected onReconnect(): void {}

  // ========== Internal ==========

  private setupInternalListeners(): void {
    this.events.on('init', (config: GameConfig) => {
      this._config = config;
      this.onInit(config);
    });

    this.events.on('roomJoined', (roomInfo: RoomInfo) => {
      this._roomInfo = roomInfo;
      this.onRoomJoined(roomInfo);
    });

    this.events.on('playerJoin', (data: any) => {
      this.onPlayerJoin(data);
    });

    this.events.on('playerLeave', (data: any) => {
      this.onPlayerLeave(data.playerId, data.reason);
    });

    this.events.on('playerReady', (data: any) => {
      this.onPlayerReady(data.playerId);
    });

    this.events.on('allReady', (data: any) => {
      this.onAllReady(data.playerCount);
    });

    this.events.on('gameStart', (data: any) => {
      this.onGameStart(data);
    });

    this.events.on('stateUpdate', (state: any) => {
      this._state = state;
      this.onStateUpdate(state);
    });

    this.events.on('privateMessage', (event: string, data: any) => {
      this.onPrivateMessage(event, data);
    });

    this.events.on('playerAction', (playerId: string, action: string, data: any) => {
      this.onPlayerAction(playerId, action, data);
    });

    this.events.on('chat', (data: any) => {
      this.onChat(data.from, data.message, data.nickname);
    });

    this.events.on('gameEnd', (results: GameResults) => {
      this.onGameEnd(results);
    });

    this.events.on('error', (error: GameError) => {
      this.onError(error);
    });
  }

  private handleMessage(msg: any): void {
    const { type, data } = msg;
    switch (type) {
      case 'connected':
        this._playerId = data.playerId || '';
        this.events.emit('init', data.config || {});
        if (data.roomInfo) {
          this.events.emit('roomJoined', data.roomInfo);
        }
        break;
      case 'player_join':
        this.events.emit('playerJoin', data);
        break;
      case 'player_leave':
        this.events.emit('playerLeave', data);
        break;
      case 'player_ready':
        this.events.emit('playerReady', data);
        break;
      case 'all_ready':
        this.events.emit('allReady', data);
        break;
      case 'game_start':
        this.events.emit('gameStart', data);
        break;
      case 'state_update':
        this.events.emit('stateUpdate', data.state || data);
        break;
      case 'private_message':
        this.events.emit('privateMessage', data.event, data.data);
        break;
      case 'player_action':
        this.events.emit('playerAction', data.playerId, data.action, data.data);
        break;
      case 'chat':
        this.events.emit('chat', data);
        break;
      case 'game_end':
        this.events.emit('gameEnd', data.results || data);
        break;
      case 'error':
        this.events.emit('error', data);
        break;
      case 'pong':
        // heartbeat response, no action needed
        break;
    }
  }

  private handleStatusChange(status: ConnectionStatus): void {
    if (status === 'disconnected') {
      this.onDisconnect('connection lost');
    } else if (status === 'connected' && this._playerId) {
      this.onReconnect();
    }
  }
}
