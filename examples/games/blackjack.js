/**
 * Blackjack (21) Game Script
 * Simple card game example
 */

// Game state
var state = {
    deck: [],
    players: [],
    dealer: {
        cards: [],
        score: 0
    },
    phase: 'betting'
};

// Initialize game
function init(config) {
    log('Blackjack initialized');
    state.deck = createDeck();
    return {
        minPlayers: 1,
        maxPlayers: 7,
        deckCount: config.deckCount || 6
    };
}

// Create and shuffle deck
function createDeck() {
    var deck = [];
    var suits = ['H', 'D', 'C', 'S'];
    var values = ['2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A'];

    for (var d = 0; d < 6; d++) {
        for (var s = 0; s < suits.length; s++) {
            for (var v = 0; v < values.length; v++) {
                deck.push({
                    suit: suits[s],
                    value: values[v],
                    numeric: getCardValue(values[v])
                });
            }
        }
    }

    // Shuffle
    for (var i = deck.length - 1; i > 0; i--) {
        var j = Math.floor(Math.random() * (i + 1));
        var temp = deck[i];
        deck[i] = deck[j];
        deck[j] = temp;
    }

    return deck;
}

// Get card numeric value
function getCardValue(value) {
    if (value === 'A') return 11;
    if (['K', 'Q', 'J'].indexOf(value) >= 0) return 10;
    return parseInt(value);
}

// Player joins
function playerJoin(playerId, data) {
    state.players.push({
        id: playerId,
        name: data.name || 'Player ' + playerId,
        chips: data.chips || 1000,
        cards: [],
        bet: 0,
        standing: false
    });

    log('Player ' + playerId + ' joined with ' + (data.chips || 1000) + ' chips');
    return { success: true };
}

// Start round
function startRound() {
    if (state.players.length === 0) {
        return { error: 'No players' };
    }

    state.phase = 'dealing';
    state.dealer.cards = [];

    // Deal initial cards
    for (var i = 0; i < state.players.length; i++) {
        state.players[i].cards = [state.deck.pop(), state.deck.pop()];
        state.players[i].standing = false;
    }

    // Deal dealer cards
    state.dealer.cards = [state.deck.pop(), state.deck.pop()];

    state.phase = 'playing';
    broadcastState();

    return { success: true };
}

// Hit - get another card
function hit(playerId) {
    var player = getPlayer(playerId);
    if (!player) return { error: 'Player not found' };
    if (player.standing) return { error: 'Already standing' };

    player.cards.push(state.deck.pop());
    var score = calculateScore(player.cards);

    if (score > 21) {
        // Bust
        log('Player ' + player.name + ' busted with ' + score);
        player.busted = true;
    }

    broadcastState();
    return { success: true, score: score };
}

// Stand - stop drawing cards
function stand(playerId) {
    var player = getPlayer(playerId);
    if (!player) return { error: 'Player not found' };

    player.standing = true;
    log('Player ' + player.name + ' stands');

    // Check if all players are done
    checkRoundEnd();
    broadcastState();

    return { success: true };
}

// Calculate hand score
function calculateScore(cards) {
    var score = 0;
    var aces = 0;

    for (var i = 0; i < cards.length; i++) {
        score += cards[i].numeric;
        if (cards[i].value === 'A') aces++;
    }

    // Adjust for aces
    while (score > 21 && aces > 0) {
        score -= 10;
        aces--;
    }

    return score;
}

// Check if round should end
function checkRoundEnd() {
    var allDone = true;
    for (var i = 0; i < state.players.length; i++) {
        if (!state.players[i].standing && !state.players[i].busted) {
            allDone = false;
            break;
        }
    }

    if (allDone) {
        dealerPlay();
    }
}

// Dealer plays
function dealerPlay() {
    state.phase = 'dealer';

    // Dealer hits until 17
    while (calculateScore(state.dealer.cards) < 17) {
        state.dealer.cards.push(state.deck.pop());
    }

    determineWinners();
}

// Determine winners
function determineWinners() {
    var dealerScore = calculateScore(state.dealer.cards);
    log('Dealer has ' + dealerScore);

    for (var i = 0; i < state.players.length; i++) {
        var player = state.players[i];
        var playerScore = calculateScore(player.cards);

        if (player.busted) {
            log(player.name + ' busted - loses');
        } else if (dealerScore > 21) {
            log(player.name + ' wins with ' + playerScore);
            player.chips += player.bet * 2;
        } else if (playerScore > dealerScore) {
            log(player.name + ' wins with ' + playerScore + ' vs dealer ' + dealerScore);
            player.chips += player.bet * 2;
        } else if (playerScore === dealerScore) {
            log(player.name + ' pushes with ' + playerScore);
            player.chips += player.bet;
        } else {
            log(player.name + ' loses with ' + playerScore);
        }
    }

    state.phase = 'complete';
}

// Get player by ID
function getPlayer(playerId) {
    for (var i = 0; i < state.players.length; i++) {
        if (state.players[i].id === playerId) {
            return state.players[i];
        }
    }
    return null;
}

// Broadcast state to all players
function broadcastState() {
    var publicState = {
        phase: state.phase,
        dealerCards: state.dealer.cards.length,
        players: []
    };

    for (var i = 0; i < state.players.length; i++) {
        publicState.players.push({
            id: state.players[i].id,
            name: state.players[i].name,
            cardCount: state.players[i].cards.length,
            standing: state.players[i].standing,
            busted: state.players[i].busted
        });
    }

    log('State: ' + JSON.stringify(publicState));
}

// Place bet
function placeBet(playerId, amount) {
    var player = getPlayer(playerId);
    if (!player) return { error: 'Player not found' };
    if (amount > player.chips) return { error: 'Insufficient chips' };

    player.chips -= amount;
    player.bet = amount;

    return { success: true };
}

// Logging
function log(msg) {
    console.log('[Blackjack] ' + msg);
}

// Get game state
function getState() {
    return {
        phase: state.phase,
        playerCount: state.players.length,
        dealerCards: state.dealer.cards.length
    };
}

// Exports
exports.init = init;
exports.playerJoin = playerJoin;
exports.startRound = startRound;
exports.hit = hit;
exports.stand = stand;
exports.placeBet = placeBet;
exports.getState = getState;
