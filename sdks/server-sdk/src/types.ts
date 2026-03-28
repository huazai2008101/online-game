/**
 * Type definitions for @gameplatform/server-sdk
 */

/** Game context passed to onInit */
export interface GameContext {
  gameId: string;
  roomId: string;
  gameCode: string;
  version: string;
  config: Record<string, any>;
  minPlayers: number;
  maxPlayers: number;
}

/** Player information */
export interface PlayerInfo {
  playerId: string;
  nickname: string;
  avatar: string;
  metadata: Record<string, any>;
}

/** Reason for player leaving */
export type LeaveReason = 'disconnect' | 'voluntary' | 'kicked' | 'timeout';

/** Game results returned by endGame() */
export interface GameResults {
  winners: string[];
  scores: Record<string, number>;
  stats: Record<string, any>;
  duration: number;
}

/** Room configuration from manifest */
export interface RoomConfig {
  maxPlayers: number;
  gameType: 'realtime' | 'turn-based';
  customConfig: Record<string, any>;
}
