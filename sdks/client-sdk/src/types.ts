/**
 * Type definitions for @gameplatform/client-sdk
 */

/** Game configuration from the platform */
export interface GameConfig {
  gameId: string;
  gameCode: string;
  version: string;
  gameType: 'realtime' | 'turn-based';
  minPlayers: number;
  maxPlayers: number;
  customConfig: Record<string, any>;
}

/** Room information */
export interface RoomInfo {
  roomId: string;
  roomName: string;
  owner: string;
  players: PlayerInfo[];
  maxPlayers: number;
  status: 'waiting' | 'playing' | 'ended';
  config: Record<string, any>;
}

/** Player information */
export interface PlayerInfo {
  playerId: string;
  nickname: string;
  avatar: string;
  isOwner: boolean;
  isReady: boolean;
  metadata: Record<string, any>;
}

/** Data sent when game starts */
export interface GameStartData {
  state: Record<string, any>;
  yourRole?: string;
  yourData?: Record<string, any>;
}

/** Game end results */
export interface GameResults {
  winners: string[];
  scores: Record<string, number>;
  stats: Record<string, any>;
  duration: number;
}

/** Error from the platform */
export interface GameError {
  code: number;
  message: string;
  data?: Record<string, any>;
}

/** Connection status */
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

/** Options for GameClient constructor */
export interface GameClientOptions {
  serverUrl?: string;
  token?: string;
  gameId?: string;
  roomId?: string;
  autoConnect: boolean;
  reconnect: boolean;
  reconnectInterval: number;
  maxReconnectAttempts: number;
  heartbeatInterval: number;
}

/** Internal message format */
export interface WsMessage {
  type: string;
  data: any;
  seq: number;
  ts: number;
}
