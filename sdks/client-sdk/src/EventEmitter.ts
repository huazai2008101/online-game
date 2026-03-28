/**
 * Simple typed event emitter for the game client.
 */
export class EventEmitter {
  private listeners: Map<string, Function[]> = new Map();

  on(event: string, handler: Function): void {
    const list = this.listeners.get(event) || [];
    list.push(handler);
    this.listeners.set(event, list);
  }

  once(event: string, handler: Function): void {
    const wrapper: Function = (...args: any[]) => {
      this.off(event, wrapper);
      handler(...args);
    };
    this.on(event, wrapper);
  }

  off(event: string, handler: Function): void {
    const list = this.listeners.get(event);
    if (!list) return;
    const idx = list.indexOf(handler);
    if (idx >= 0) list.splice(idx, 1);
  }

  emit(event: string, ...args: any[]): void {
    const list = this.listeners.get(event);
    if (!list) return;
    // Iterate over a copy to allow handlers to remove themselves
    for (const fn of [...list]) {
      try {
        fn(...args);
      } catch (e) {
        console.error(`[GameClient] Error in "${event}" handler:`, e);
      }
    }
  }

  removeAllListeners(event?: string): void {
    if (event) {
      this.listeners.delete(event);
    } else {
      this.listeners.clear();
    }
  }
}
