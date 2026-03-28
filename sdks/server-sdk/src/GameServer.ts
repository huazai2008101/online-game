/**
 * GameServer base class for game developers.
 *
 * The platform provides a global `GameServer` object at runtime (in goja).
 * Extend this class for TypeScript type safety, then call `register(YourClass)`.
 *
 * Usage:
 *   import { GameServer, register } from '@gameplatform/server-sdk'
 *   class MyGame extends GameServer {
 *     onInit(ctx) { this.state = { phase: 'waiting' } }
 *   }
 *   register(MyGame)
 *
 * Or assign directly (no class):
 *   GameServer.onInit = function(ctx) { this.state = {} }
 */

import type { GameContext, PlayerInfo, LeaveReason, GameResults, RoomConfig } from './types';

// Platform host functions injected by Go runtime
declare function __platform_broadcast(event: string, data: any): void;
declare function __platform_sendTo(playerId: string, event: string, data: any): void;
declare function __platform_sendExcept(playerId: string, event: string, data: any): void;
declare function __platform_endGame(results: any): void;
declare function __platform_getPlayers(): string[];
declare function __platform_getRoomConfig(): any;
declare function __platform_getRoomId(): string;
declare function __platform_getGameId(): string;
declare function __platform_setTimeout(fn: () => void, ms: number): number;
declare function __platform_setInterval(fn: () => void, ms: number): number;
declare function __platform_clearTimeout(id: number): void;
declare function __platform_clearInterval(id: number): void;
declare function __platform_randomInt(min: number, max: number): number;
declare function __platform_shuffle(arr: any[]): any[];
declare function __platform_randomChoice(arr: any[]): any;
declare function __platform_uuid(): string;
declare function __platform_log(...args: any[]): void;
declare function __platform_warn(...args: any[]): void;
declare function __platform_error(...args: any[]): void;
declare function __platform_getState(): any;
declare function __platform_setState(state: any): void;

export abstract class GameServer {

  // ========== State ==========

  state: Record<string, any> = {};

  getPublicState(): Record<string, any> {
    return this.state;
  }

  // ========== Communication ==========

  broadcast(event: string, data: any): void {
    __platform_broadcast(event, data);
  }

  sendTo(playerId: string, event: string, data: any): void {
    __platform_sendTo(playerId, event, data);
  }

  sendExcept(playerId: string, event: string, data: any): void {
    __platform_sendExcept(playerId, event, data);
  }

  // ========== Game Control ==========

  endGame(results: GameResults): void {
    __platform_endGame(results);
  }

  // ========== Room Info ==========

  getPlayers(): string[] {
    return __platform_getPlayers();
  }

  getPlayerCount(): number {
    var players = __platform_getPlayers();
    return players ? players.length : 0;
  }

  getRoomConfig(): RoomConfig {
    return __platform_getRoomConfig();
  }

  getRoomId(): string {
    return __platform_getRoomId();
  }

  getGameId(): string {
    return __platform_getGameId();
  }

  // ========== Player Data ==========

  private _playerData: Record<string, Record<string, any>> = {};

  getPlayerData(playerId: string): Record<string, any> {
    if (!this._playerData[playerId]) {
      this._playerData[playerId] = {};
    }
    return this._playerData[playerId];
  }

  setPlayerData(playerId: string, key: string, value: any): void {
    if (!this._playerData[playerId]) {
      this._playerData[playerId] = {};
    }
    this._playerData[playerId][key] = value;
  }

  // ========== Timers ==========

  setTimeout(callback: () => void, ms: number): number {
    return __platform_setTimeout(callback, ms);
  }

  setInterval(callback: () => void, ms: number): number {
    return __platform_setInterval(callback, ms);
  }

  clearTimeout(id: number): void {
    __platform_clearTimeout(id);
  }

  clearInterval(id: number): void {
    __platform_clearInterval(id);
  }

  // ========== Utilities ==========

  randomInt(min: number, max: number): number {
    return __platform_randomInt(min, max);
  }

  shuffle(arr: any[]): any[] {
    return __platform_shuffle(arr);
  }

  randomChoice(arr: any[]): any {
    return __platform_randomChoice(arr);
  }

  uuid(): string {
    return __platform_uuid();
  }

  // ========== Logging ==========

  log(...args: any[]): void {
    __platform_log.apply(null, args);
  }

  warn(...args: any[]): void {
    __platform_warn.apply(null, args);
  }

  error(...args: any[]): void {
    __platform_error.apply(null, args);
  }

  // ========== Lifecycle Hooks ==========
  // Override these in your subclass

  onInit(_ctx: GameContext): void {}
  onPlayerJoin(_playerId: string, _playerInfo: PlayerInfo): void {}
  onPlayerLeave(_playerId: string, _reason: LeaveReason): void {}
  onGameStart(): void {}
  onPlayerAction(_playerId: string, _action: string, _data: any): void {}
  onGameEnd(_results: GameResults): void {}
  onRestore(_state: any): void {}
  onTick(_deltaTimeMs: number): void {}
}

/**
 * Register a GameServer subclass with the platform runtime.
 * Call this at the end of your game script:
 *
 *   register(MyGame)
 *
 * This copies lifecycle hooks from your class to the global GameServer object,
 * so the engine can call them via the platform runtime.
 */
export function register(GameClass: new (...args: any[]) => GameServer): void {
  var gs: any = (typeof GameServer !== 'undefined') ? GameServer :
                (typeof globalThis !== 'undefined') ? (globalThis as any).GameServer : null;
  if (!gs) return;

  var proto = GameClass.prototype;
  var baseProto = GameServer.prototype;
  var hooks = [
    'onInit', 'onPlayerJoin', 'onPlayerLeave', 'onGameStart',
    'onPlayerAction', 'onGameEnd', 'onRestore', 'onTick', 'getPublicState'
  ];

  for (var i = 0; i < hooks.length; i++) {
    var hook = hooks[i];
    if (typeof proto[hook] === 'function' && proto[hook] !== baseProto[hook]) {
      gs[hook] = (function(method: Function) {
        return function() {
          return method.apply(gs, arguments);
        };
      })(proto[hook]);
    }
  }
}
