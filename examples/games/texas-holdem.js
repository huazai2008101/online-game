/**
 * Texas Hold'em Poker Game Script
 * Supports: JavaScript Engine
 */

// Game State
var gameState = {
    deck: [],
    players: [],
    pot: 0,
    currentBet: 0,
    communityCards: [],
    phase: 'waiting', // waiting, preflop, flop, turn, river, showdown
    dealerIndex: 0,
    currentPlayerIndex: 0
};

// Card ranks and suits
var ranks = ['2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A'];
var suits = ['hearts', 'diamonds', 'clubs', 'spades'];

/**
 * Initialize the game
 */
function init(config) {
    log('Texas Hold\'em initialized');
    gameState.phase = 'waiting';
    return {
        minPlayers: 2,
        maxPlayers: 10,
        buyIn: config.buyIn || 1000
    };
}

/**
 * Player joins the game
 */
function playerJoin(playerId, playerData) {
    if (gameState.players.length >= 10) {
        return { error: 'Game is full' };
    }

    gameState.players.push({
        id: playerId,
        name: playerData.name || 'Player ' + playerId,
        chips: playerData.chips || 1000,
        cards: [],
        bet: 0,
        folded: false,
        allIn: false
    });

    log('Player ' + playerId + ' joined');
    return { success: true };
}

/**
 * Player leaves the game
 */
function playerLeave(playerId) {
    var index = -1;
    for (var i = 0; i < gameState.players.length; i++) {
        if (gameState.players[i].id === playerId) {
            index = i;
            break;
        }
    }

    if (index >= 0) {
        gameState.players.splice(index, 1);
        log('Player ' + playerId + ' left');
        return { success: true };
    }

    return { error: 'Player not found' };
}

/**
 * Start the game
 */
function startGame() {
    if (gameState.players.length < 2) {
        return { error: 'Need at least 2 players' };
    }

    gameState.phase = 'preflop';
    gameState.deck = createDeck();
    shuffleDeck();
    dealHoleCards();
    gameState.currentPlayerIndex = (gameState.dealerIndex + 3) % gameState.players.length;

    log('Game started');
    broadcastGameState();

    return { success: true };
}

/**
 * Create a standard deck
 */
function createDeck() {
    var deck = [];
    for (var s = 0; s < suits.length; s++) {
        for (var r = 0; r < ranks.length; r++) {
            deck.push({
                rank: ranks[r],
                suit: suits[s],
                value: r + 2
            });
        }
    }
    return deck;
}

/**
 * Shuffle the deck
 */
function shuffleDeck() {
    for (var i = gameState.deck.length - 1; i > 0; i--) {
        var j = Math.floor(Math.random() * (i + 1));
        var temp = gameState.deck[i];
        gameState.deck[i] = gameState.deck[j];
        gameState.deck[j] = temp;
    }
}

/**
 * Deal hole cards to players
 */
function dealHoleCards() {
    for (var p = 0; p < gameState.players.length; p++) {
        gameState.players[p].cards = [
            gameState.deck.pop(),
            gameState.deck.pop()
        ];
    }
}

/**
 * Player action: fold, call, raise
 */
function playerAction(playerId, action, data) {
    var player = getCurrentPlayer();
    if (player.id !== playerId) {
        return { error: 'Not your turn' };
    }

    switch (action) {
        case 'fold':
            player.folded = true;
            break;
        case 'call':
            var toCall = gameState.currentBet - player.bet;
            if (player.chips < toCall) {
                // All-in
                player.allIn = true;
                player.chips = 0;
            } else {
                player.chips -= toCall;
                player.bet += toCall;
            }
            break;
        case 'raise':
            var amount = data.amount || gameState.currentBet;
            if (player.chips < amount) {
                return { error: 'Not enough chips' };
            }
            player.chips -= amount;
            player.bet += amount;
            gameState.currentBet = player.bet;
            break;
    }

    nextPlayer();
    return { success: true };
}

/**
 * Move to next player
 */
function nextPlayer() {
    var activePlayers = gameState.players.filter(function(p) { return !p.folded && !p.allIn; });

    if (activePlayers.length <= 1) {
        endHand();
        return;
    }

    do {
        gameState.currentPlayerIndex = (gameState.currentPlayerIndex + 1) % gameState.players.length;
    } while (gameState.players[gameState.currentPlayerIndex].folded);

    // Check if betting round is complete
    if (isBettingRoundComplete()) {
        nextPhase();
    }
}

/**
 * Check if betting round is complete
 */
function isBettingRoundComplete() {
    var activePlayers = gameState.players.filter(function(p) { return !p.folded; });
    var maxBet = Math.max.apply(null, gameState.players.map(function(p) { return p.bet; }));

    for (var i = 0; i < activePlayers.length; i++) {
        if (!activePlayers[i].allIn && activePlayers[i].bet !== maxBet) {
            return false;
        }
    }
    return true;
}

/**
 * Move to next phase
 */
function nextPhase() {
    // Reset bets for next round
    for (var i = 0; i < gameState.players.length; i++) {
        gameState.players[i].bet = 0;
    }
    gameState.currentBet = 0;

    switch (gameState.phase) {
        case 'preflop':
            gameState.phase = 'flop';
            dealCommunityCards(3);
            break;
        case 'flop':
            gameState.phase = 'turn';
            dealCommunityCards(1);
            break;
        case 'turn':
            gameState.phase = 'river';
            dealCommunityCards(1);
            break;
        case 'river':
            gameState.phase = 'showdown';
            determineWinner();
            break;
    }

    gameState.currentPlayerIndex = (gameState.dealerIndex + 1) % gameState.players.length;
    broadcastGameState();
}

/**
 * Deal community cards
 */
function dealCommunityCards(count) {
    for (var i = 0; i < count; i++) {
        gameState.communityCards.push(gameState.deck.pop());
    }
}

/**
 * Determine winner
 */
function determineWinner() {
    var activePlayers = gameState.players.filter(function(p) { return !p.folded; });

    if (activePlayers.length === 1) {
        // Everyone else folded
        awardPot(activePlayers[0]);
    } else {
        // Evaluate hands
        var bestPlayer = null;
        var bestScore = -1;

        for (var i = 0; i < activePlayers.length; i++) {
            var score = evaluateHand(activePlayers[i]);
            if (score > bestScore) {
                bestScore = score;
                bestPlayer = activePlayers[i];
            }
        }

        awardPot(bestPlayer);
    }

    // Prepare for next hand
    setTimeout(function() {
        startNewHand();
    }, 5000);
}

/**
 * Evaluate hand (simplified)
 */
function evaluateHand(player) {
    var allCards = player.cards.concat(gameState.communityCards);
    var score = 0;

    // Simple high card evaluation
    for (var i = 0; i < allCards.length; i++) {
        score += allCards[i].value;
    }

    return score;
}

/**
 * Award pot to winner
 */
function awardPot(winner) {
    winner.chips += gameState.pot;
    log(winner.name + ' wins ' + gameState.pot + ' chips!');
    gameState.pot = 0;
}

/**
 * Start new hand
 */
function startNewHand() {
    gameState.dealerIndex = (gameState.dealerIndex + 1) % gameState.players.length;
    gameState.phase = 'preflop';
    gameState.deck = createDeck();
    shuffleDeck();
    gameState.communityCards = [];
    gameState.currentBet = 0;

    for (var i = 0; i < gameState.players.length; i++) {
        gameState.players[i].cards = [];
        gameState.players[i].bet = 0;
        gameState.players[i].folded = false;
        gameState.players[i].allIn = false;
    }

    dealHoleCards();
    broadcastGameState();
}

/**
 * End the hand
 */
function endHand() {
    gameState.phase = 'showdown';
    determineWinner();
}

/**
 * Broadcast game state to all players
 */
function broadcastGameState() {
    // This would send the state to connected players
    log('State: ' + gameState.phase + ', Players: ' + gameState.players.length);
}

/**
 * Get current game state
 */
function getState() {
    return {
        phase: gameState.phase,
        playerCount: gameState.players.length,
        pot: gameState.pot,
        communityCards: gameState.communityCards,
        currentPlayer: gameState.players[gameState.currentPlayerIndex] ? gameState.players[gameState.currentPlayerIndex].id : null
    };
}

/**
 * Get current player
 */
function getCurrentPlayer() {
    return gameState.players[gameState.currentPlayerIndex];
}

/**
 * Logging function
 */
function log(msg) {
    console.log('[Texas Holdem] ' + msg);
}

// Export functions
exports.init = init;
exports.playerJoin = playerJoin;
exports.playerLeave = playerLeave;
exports.startGame = startGame;
exports.playerAction = playerAction;
exports.getState = getState;
