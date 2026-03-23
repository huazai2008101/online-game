# Game Platform API Reference

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Most endpoints require authentication via Bearer token:

```
Authorization: Bearer <token>
```

## Common Response Format

```json
{
    "code": 0,
    "message": "success",
    "data": { ... }
}
```

Error response:

```json
{
    "code": 40001,
    "message": "Invalid parameter",
    "data": null
}
```

---

## Health Check

### GET /health

Check service health status.

**Response:**
```json
{
    "status": "healthy",
    "timestamp": "2026-03-23T12:00:00Z",
    "checks": {
        "database": {
            "name": "database",
            "status": "healthy",
            "duration_ms": 5
        },
        "redis": {
            "name": "redis",
            "status": "healthy",
            "duration_ms": 2
        }
    }
}
```

---

## User Service (Port 8001)

### POST /users/register

Register a new user.

**Request Body:**
```json
{
    "username": "player1",
    "password": "securepassword",
    "email": "player1@example.com",
    "phone": "+1234567890",
    "nickname": "Player One"
}
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 1000001,
        "username": "player1",
        "nickname": "Player One",
        "created_at": "2026-03-23T12:00:00Z"
    }
}
```

### POST /users/login

User login.

**Request Body:**
```json
{
    "username": "player1",
    "password": "securepassword"
}
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 1000001,
        "token": "eyJhbGciOiJIUzI1NiIs...",
        "expires_at": "2026-03-24T12:00:00Z"
    }
}
```

### GET /users/profile

Get user profile.

**Headers:**
```
Authorization: Bearer <token>
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 1000001,
        "username": "player1",
        "nickname": "Player One",
        "avatar": "https://cdn.example.com/avatars/1000001.png",
        "level": 10,
        "exp": 15000
    }
}
```

### PUT /users/profile

Update user profile.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "nickname": "New Nickname",
    "avatar": "https://cdn.example.com/new-avatar.png",
    "bio": "I love gaming!"
}
```

### GET /users/friends

Get friends list.

**Headers:**
```
Authorization: Bearer <token>
```

**Query Parameters:**
- `page` (int): Page number (default: 1)
- `limit` (int): Items per page (default: 20)

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total": 50,
        "page": 1,
        "limit": 20,
        "friends": [
            {
                "user_id": 1000002,
                "username": "player2",
                "nickname": "Player Two",
                "status": "online",
                "remark": "My best friend"
            }
        ]
    }
}
```

### POST /users/friends/request

Send friend request.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "user_id": 1000002,
    "message": "Let's be friends!"
}
```

---

## Game Service (Port 8002)

### GET /games

List available games.

**Query Parameters:**
- `page` (int): Page number
- `limit` (int): Items per page
- `type` (string): Filter by game type

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total": 10,
        "games": [
            {
                "game_id": 1,
                "game_code": "texas-holdem",
                "game_name": "Texas Hold'em",
                "game_type": "card",
                "game_icon": "https://cdn.example.com/games/texas-holdem.png",
                "game_cover": "https://cdn.example.com/covers/texas-holdem.jpg",
                "min_players": 2,
                "max_players": 10,
                "status": "active"
            }
        ]
    }
}
```

### GET /games/{game_id}

Get game details.

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "game_id": 1,
        "game_code": "texas-holdem",
        "game_name": "Texas Hold'em",
        "description": "Classic poker game",
        "rules": { ... },
        "config": { ... }
    }
}
```

### POST /games/rooms

Create a game room.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "game_id": 1,
    "room_name": "My Poker Room",
    "max_players": 6,
    "password": "roompass",
    "config": {
        "buy_in": 1000,
        "blind_structure": "normal"
    }
}
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "room_id": "room_1234567890",
        "game_id": 1,
        "room_name": "My Poker Room",
        "host_id": 1000001,
        "player_count": 1,
        "max_players": 6,
        "status": "waiting",
        "created_at": "2026-03-23T12:00:00Z"
    }
}
```

### POST /games/rooms/{room_id}/join

Join a game room.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "password": "roompass"
}
```

### POST /games/rooms/{room_id}/leave

Leave a game room.

**Headers:**
```
Authorization: Bearer <token>
```

### POST /games/rooms/{room_id}/start

Start a game (host only).

**Headers:**
```
Authorization: Bearer <token>
```

### POST /games/rooms/{room_id}/action

Send player action during game.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "action": "bet",
    "data": {
        "amount": 100
    }
}
```

---

## Payment Service (Port 8003)

### GET /scores

Get user score balance.

**Headers:**
```
Authorization: Bearer <token>
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 1000001,
        "balance": 50000,
        "currency": "coins"
    }
}
```

### POST /orders

Create payment order.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "product_type": "coin_package",
    "product_id": "pkg_1000",
    "amount": 9900,
    "currency": "USD"
}
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "order_no": "ORD20260323120000ABC",
        "amount": 9900,
        "currency": "USD",
        "status": "pending",
        "payment_url": "https://payment.example.com/..."
    }
}
```

### GET /orders/{order_no}

Get order status.

**Headers:**
```
Authorization: Bearer <token>
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "order_no": "ORD20260323120000ABC",
        "product_type": "coin_package",
        "product_id": "pkg_1000",
        "amount": 9900,
        "status": "completed",
        "created_at": "2026-03-23T12:00:00Z",
        "updated_at": "2026-03-23T12:01:00Z"
    }
}
```

---

## Player Service (Port 8004)

### GET /players/{player_id}

Get player statistics.

**Headers:**
```
Authorization: Bearer <token>
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "player_id": 1000001,
        "nickname": "Player One",
        "level": 10,
        "exp": 15000,
        "score": 50000,
        "stats": {
            "games_played": 500,
            "games_won": 200,
            "win_rate": 0.4,
            "total_score": 500000
        }
    }
}
```

### GET /players/{player_id}/history

Get player game history.

**Query Parameters:**
- `game_id` (int): Filter by game
- `page` (int): Page number
- `limit` (int): Items per page

---

## Guild Service (Port 8006)

### GET /guilds

List guilds.

**Query Parameters:**
- `page` (int): Page number
- `limit` (int): Items per page

### POST /guilds

Create a guild.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
    "guild_name": "Dragon Slayers",
    "description": "We hunt dragons!",
    " emblem": "https://cdn.example.com/guilds/dragon-slayers.png"
}
```

### GET /guilds/{guild_id}

Get guild details.

### POST /guilds/{guild_id}/join

Request to join a guild.

### POST /guilds/{guild_id}/members/{user_id}/kick

Kick a member (guild leader only).

---

## Notification Service (Port 8008)

### GET /notifications

Get user notifications.

**Headers:**
```
Authorization: Bearer <token>
```

**Query Parameters:**
- `page` (int): Page number
- `limit` (int): Items per page
- `read` (bool): Filter by read status

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total": 100,
        "unread": 5,
        "notifications": [
            {
                "id": 1,
                "type": "friend_request",
                "title": "New Friend Request",
                "content": "Player Two wants to be your friend",
                "read": false,
                "created_at": "2026-03-23T12:00:00Z"
            }
        ]
    }
}
```

### PUT /notifications/{id}/read

Mark notification as read.

### PUT /notifications/read-all

Mark all notifications as read.

---

## ID Service (Port 8011)

### GET /id/generate

Generate a unique ID.

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "id": 1000000000000000001
    }
}
```

### POST /id/batch

Batch generate IDs.

**Request Body:**
```json
{
    "count": 100
}
```

**Response:**
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "ids": [1000000000000000001, 1000000000000000002, ...]
    }
}
```

---

## WebSocket API

### Connection

Connect to WebSocket:

```
ws://localhost:8080/ws?token=<jwt_token>
```

### Message Format

**Client -> Server:**
```json
{
    "type": "room_msg",
    "room": "room_1234567890",
    "content": {
        "action": "chat",
        "message": "Hello everyone!"
    }
}
```

**Server -> Client:**
```json
{
    "type": "room_msg",
    "from": "user_1000001",
    "room": "room_1234567890",
    "content": {
        "action": "chat",
        "message": "Hello everyone!"
    },
    "time": "2026-03-23T12:00:00Z"
}
```

### Message Types

- `room_join`: Join a room
- `room_leave`: Leave a room
- `room_msg`: Send message to room
- `ping`: Ping server
- `pong`: Pong response

---

## Error Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 40001 | Invalid parameter |
| 40101 | Unauthorized |
| 40301 | Forbidden |
| 40401 | Resource not found |
| 40901 | Resource conflict |
| 42901 | Rate limit exceeded |
| 50001 | Internal server error |
| 50002 | Database error |
| 50003 | Cache error |
| 50004 | External service error |

---

## Rate Limiting

API requests are rate limited:

- 1000 requests per hour per IP
- 100 requests per minute per user

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1648000000
```
