/**
 * Blackjack Server - using @gameplatform/server-sdk
 *
 * Compile: tsc --outDir ../dist/server
 * Output: main.js (ES5, runs in goja runtime)
 */

// In goja runtime, GameServer is a global object provided by the platform.
// The TypeScript import is for type-checking only during development.
// When compiled, the game script runs after the SDK runtime has been loaded.
//
// Usage pattern: assign lifecycle hooks to the global GameServer object.

// Type references (available via the platform global GameServer)
interface Card { suit: string; rank: string; value: number }
interface PlayerState {
  chips: number;
  hand: Card[];
  bet: number;
  stood: boolean;
  busted: boolean;
  result: string;
}
interface GameState {
  phase: 'waiting' | 'betting' | 'playing' | 'dealer' | 'ended';
  deck: Card[];
  players: Record<string, PlayerState>;
  dealerHand: Card[];
  currentPlayerIndex: number;
  turnOrder: string[];
  pot: number;
}

const SUITS = ['hearts', 'diamonds', 'clubs', 'spades'];
const RANKS = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];

function createDeck(): Card[] {
  const deck: Card[] = [];
  for (const suit of SUITS) {
    for (const rank of RANKS) {
      const value = rank === 'A' ? 11 : ['J', 'Q', 'K'].indexOf(rank) >= 0 ? 10 : parseInt(rank);
      deck.push({ suit, rank, value });
    }
  }
  return deck;
}

function handValue(hand: Card[]): number {
  let total = 0;
  let aces = 0;
  for (const card of hand) {
    total += card.value;
    if (card.rank === 'A') aces++;
  }
  while (total > 21 && aces > 0) {
    total -= 10;
    aces--;
  }
  return total;
}

function cardToString(card: Card): string {
  return card.rank + '_of_' + card.suit;
}

// ===== Game Logic =====

GameServer.onInit = function(ctx: any) {
  this.state = {
    phase: 'waiting',
    deck: [],
    players: {},
    dealerHand: [],
    currentPlayerIndex: 0,
    turnOrder: [],
    pot: 0,
  } as GameState;

  this.log('Blackjack initialized', ctx.gameId);
};

GameServer.onPlayerJoin = function(playerId: string, info: any) {
  const state = this.state as GameState;
  if (state.phase !== 'waiting') {
    this.sendTo(playerId, 'error', { code: 40001, message: '游戏已经开始' });
    return;
  }

  state.players[playerId] = {
    chips: 1000,
    hand: [],
    bet: 0,
    stood: false,
    busted: false,
    result: '',
  };

  state.turnOrder.push(playerId);

  this.broadcast('player_joined', {
    playerId,
    nickname: info.nickname,
    chips: 1000,
    playerCount: state.turnOrder.length,
  });

  this.log('Player joined: ' + playerId);
};

GameServer.onPlayerLeave = function(playerId: string, reason: string) {
  const state = this.state as GameState;
  delete state.players[playerId];
  const idx = state.turnOrder.indexOf(playerId);
  if (idx >= 0) state.turnOrder.splice(idx, 1);

  this.broadcast('player_left', { playerId, reason });

  // If game is in progress and this was the current player, advance
  if (state.phase === 'playing' && state.turnOrder.length > 0) {
    this.advanceTurn();
  }
};

GameServer.onGameStart = function() {
  const state = this.state as GameState;

  // Build deck (6 decks shuffled together)
  let deck: Card[] = [];
  for (let i = 0; i < 6; i++) {
    deck = deck.concat(createDeck());
  }
  state.deck = this.shuffle(deck);
  state.phase = 'betting';

  this.broadcast('betting_start', {
    players: state.turnOrder,
    betLimits: { min: 10, max: 500 },
  });

  this.log('Betting phase started');
};

GameServer.onPlayerAction = function(playerId: string, action: string, data: any) {
  const state = this.state as GameState;

  switch (action) {
    case 'bet':
      this.handleBet(playerId, data.amount);
      break;
    case 'hit':
      this.handleHit(playerId);
      break;
    case 'stand':
      this.handleStand(playerId);
      break;
    case 'double':
      this.handleDouble(playerId);
      break;
    default:
      this.sendTo(playerId, 'error', { code: 40002, message: '未知操作: ' + action });
  }
};

GameServer.handleBet = function(playerId: string, amount: number) {
  const state = this.state as GameState;
  if (state.phase !== 'betting') {
    this.sendTo(playerId, 'error', { code: 40003, message: '当前不是下注阶段' });
    return;
  }

  const player = state.players[playerId];
  if (!player) return;

  if (amount < 10 || amount > 500) {
    this.sendTo(playerId, 'error', { code: 40004, message: '下注金额需在 10-500 之间' });
    return;
  }
  if (amount > player.chips) {
    this.sendTo(playerId, 'error', { code: 40005, message: '筹码不足' });
    return;
  }

  player.bet = amount;
  player.chips -= amount;
  state.pot += amount;

  this.broadcast('player_bet', { playerId, amount, chips: player.chips });

  // Check if all players have bet
  const allBet = state.turnOrder.every(function(pid: string) {
    return state.players[pid] && state.players[pid].bet > 0;
  });

  if (allBet) {
    this.dealCards();
  }
};

GameServer.dealCards = function() {
  const state = this.state as GameState;
  state.phase = 'playing';

  // Deal 2 cards to each player
  for (const pid of state.turnOrder) {
    const player = state.players[pid];
    player.hand = [state.deck.pop()!, state.deck.pop()!];
  }

  // Deal 2 cards to dealer (one hidden)
  state.dealerHand = [state.deck.pop()!, state.deck.pop()!];

  state.currentPlayerIndex = 0;

  // Send each player their hand privately
  for (const pid of state.turnOrder) {
    const player = state.players[pid];
    this.sendTo(pid, 'your_hand', {
      hand: player.hand.map(cardToString),
      value: handValue(player.hand),
    });
  }

  // Broadcast public state (hide card details)
  this.broadcastPublicState();

  // Notify first player it's their turn
  if (state.turnOrder.length > 0) {
    this.broadcast('your_turn', { playerId: state.turnOrder[0] });
  }

  this.log('Cards dealt, game playing');
};

GameServer.handleHit = function(playerId: string) {
  const state = this.state as GameState;
  if (state.phase !== 'playing') return;

  const currentPid = state.turnOrder[state.currentPlayerIndex];
  if (playerId !== currentPid) {
    this.sendTo(playerId, 'error', { code: 40006, message: '还没轮到你' });
    return;
  }

  const player = state.players[playerId];
  const card = state.deck.pop()!;
  player.hand.push(card);

  const value = handValue(player.hand);

  // Send the new card privately
  this.sendTo(playerId, 'card_dealt', {
    card: cardToString(card),
    handValue: value,
  });

  if (value > 21) {
    player.busted = true;
    this.broadcast('player_busted', { playerId, value });
    this.advanceTurn();
  } else if (value === 21) {
    player.stood = true;
    this.broadcast('player_blackjack', { playerId });
    this.advanceTurn();
  } else {
    this.broadcastPublicState();
  }
};

GameServer.handleStand = function(playerId: string) {
  const state = this.state as GameState;
  if (state.phase !== 'playing') return;

  const currentPid = state.turnOrder[state.currentPlayerIndex];
  if (playerId !== currentPid) {
    this.sendTo(playerId, 'error', { code: 40006, message: '还没轮到你' });
    return;
  }

  state.players[playerId].stood = true;
  this.broadcast('player_stood', { playerId, value: handValue(state.players[playerId].hand) });
  this.advanceTurn();
};

GameServer.handleDouble = function(playerId: string) {
  const state = this.state as GameState;
  if (state.phase !== 'playing') return;

  const currentPid = state.turnIndex < state.turnOrder.length ? state.turnOrder[state.currentPlayerIndex] : '';
  if (playerId !== currentPid) {
    this.sendTo(playerId, 'error', { code: 40006, message: '还没轮到你' });
    return;
  }

  const player = state.players[playerId];
  if (player.hand.length !== 2) {
    this.sendTo(playerId, 'error', { code: 40007, message: '只能在首次操作时加倍' });
    return;
  }
  if (player.chips < player.bet) {
    this.sendTo(playerId, 'error', { code: 40005, message: '筹码不足，无法加倍' });
    return;
  }

  // Double the bet
  player.chips -= player.bet;
  state.pot += player.bet;
  player.bet *= 2;

  // Deal exactly one more card
  const card = state.deck.pop()!;
  player.hand.push(card);
  player.stood = true;

  const value = handValue(player.hand);
  if (value > 21) {
    player.busted = true;
    this.broadcast('player_busted', { playerId, value });
  } else {
    this.broadcast('player_doubled', { playerId, bet: player.bet, value });
  }

  this.advanceTurn();
};

GameServer.advanceTurn = function() {
  const state = this.state as GameState;
  state.currentPlayerIndex++;

  if (state.currentPlayerIndex >= state.turnOrder.length) {
    // All players done, dealer's turn
    this.dealerPlay();
    return;
  }

  const nextPid = state.turnOrder[state.currentPlayerIndex];
  // Skip busted/standing players
  const player = state.players[nextPid];
  if (player && (player.busted || player.stood)) {
    this.advanceTurn();
    return;
  }

  this.broadcast('your_turn', { playerId: nextPid });
  this.broadcastPublicState();
};

GameServer.dealerPlay = function() {
  const state = this.state as GameState;
  state.phase = 'dealer';

  // Reveal dealer's hidden card
  this.broadcast('dealer_reveal', {
    hand: state.dealerHand.map(cardToString),
    value: handValue(state.dealerHand),
  });

  // Dealer draws until 17+
  const self = this;
  function draw() {
    const dv = handValue(state.dealerHand);
    if (dv < 17) {
      const card = state.deck.pop()!;
      state.dealerHand.push(card);
      self.broadcast('dealer_hit', {
        card: cardToString(card),
        value: handValue(state.dealerHand),
      });
      self.setTimeout(draw, 800);
    } else {
      self.resolveGame();
    }
  }

  this.setTimeout(draw, 500);
};

GameServer.resolveGame = function() {
  const state = this.state as GameState;
  const dealerValue = handValue(state.dealerHand);
  const dealerBusted = dealerValue > 21;

  const scores: Record<string, number> = {};
  const winners: string[] = [];

  for (const pid of state.turnOrder) {
    const player = state.players[pid];
    const playerValue = handValue(player.hand);

    if (player.busted) {
      player.result = 'lose';
    } else if (dealerBusted) {
      player.result = 'win';
      player.chips += player.bet * 2;
      winners.push(pid);
    } else if (playerValue > dealerValue) {
      player.result = 'win';
      player.chips += player.bet * 2;
      winners.push(pid);
    } else if (playerValue === dealerValue) {
      player.result = 'push';
      player.chips += player.bet;
    } else {
      player.result = 'lose';
    }

    scores[pid] = player.chips;
  }

  state.phase = 'ended';

  this.broadcast('round_result', {
    winners,
    scores,
    dealerValue,
    dealerBusted,
    players: state.turnOrder.map(function(pid: string) {
      return {
        playerId: pid,
        result: state.players[pid].result,
        handValue: handValue(state.players[pid].hand),
        chips: state.players[pid].chips,
      };
    }),
  });

  this.log('Round ended. Dealer: ' + dealerValue + (dealerBusted ? ' (busted)' : ''));
};

GameServer.broadcastPublicState = function() {
  const state = this.state as GameState;
  this.broadcast('state_update', {
    phase: state.phase,
    pot: state.pot,
    currentPlayer: state.turnOrder[state.currentPlayerIndex] || null,
    playerCount: state.turnOrder.length,
    dealerUpCard: state.dealerHand.length > 0 ? cardToString(state.dealerHand[0]) : null,
    players: state.turnOrder.map(function(pid: string) {
      const p = state.players[pid];
      return {
        playerId: pid,
        bet: p.bet,
        cardCount: p.hand.length,
        stood: p.stood,
        busted: p.busted,
        chips: p.chips,
      };
    }),
  });
};
