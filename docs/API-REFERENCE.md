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

## Response Format

### Success Response (HTTP 200)

**HTTP Status:** `200 OK`

**Headers:**
```
Content-Type: application/json
```

**Body:**
```json
{
    "user_id": 1000001,
    "username": "player1",
    "nickname": "Player One",
    ...
}
```

### Error Response (Non-200)

**HTTP Status:** `Error Code` (e.g., 400, 401, 404, 500, etc.)

**Headers:**
```
X-Error-Message: Invalid parameter: username is required
```

**Body:** 空 或 可选的调试信息

```
(空body)
```

或包含调试信息（开发环境）：

```json
{
    "field": "username",
    "reason": "required",
    "request_id": "req_123456"
}
```

### Design Principles

1. **HTTP 200**: 请求成功，Body包含业务数据
2. **Non-200**: 请求失败，HTTP状态码即错误码
3. **X-Error-Message Header**: 失败时返回可读的错误信息（客户端主要读取这个）
4. **Body**: 成功时返回业务数据，失败时为空或可选的调试信息

---

## HTTP Status Codes (Error Codes)

| Code | Description | Example |
|------|-------------|---------|
| **200** | Success | Request completed successfully |
| **400** | Bad Request | Invalid parameter, malformed request |
| **401** | Unauthorized | Missing or invalid token |
| **403** | Forbidden | Insufficient permissions |
| **404** | Not Found | Resource does not exist |
| **409** | Conflict | Resource already exists, state conflict |
| **429** | Too Many Requests | Rate limit exceeded |
| **500** | Internal Server Error | Unexpected server error |
| **502** | Bad Gateway | Upstream service error |
| **503** | Service Unavailable | Service temporarily unavailable |
| **504** | Gateway Timeout | Upstream service timeout |

### Common Error Scenarios

| Scenario | Status Code | X-Error-Message Example |
|----------|-------------|-------------------------|
| 参数缺失 | 400 | `Required parameter 'username' is missing` |
| 参数格式错误 | 400 | `Invalid email format` |
| 用户不存在 | 404 | `User not found: user_id=1000001` |
| Token过期 | 401 | `Token expired or invalid` |
| 权限不足 | 403 | `Insufficient permission to access this resource` |
| 资源已存在 | 409 | `Username already exists: player1` |
| 超出限流 | 429 | `Rate limit exceeded: 100 requests per minute` |
| 服务异常 | 500 | `Internal server error, please try again later` |

---

## Health Check

### GET /health

Check service health status.

**Response (200 OK):**
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

**Success Response (200 OK):**
```json
{
    "user_id": 1000001,
    "username": "player1",
    "nickname": "Player One",
    "created_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (409 Conflict):**
```
HTTP/1.1 409 Conflict
X-Error-Message: Username already exists: player1
```

---

### POST /users/login

User login.

**Request Body:**
```json
{
    "username": "player1",
    "password": "securepassword"
}
```

**Success Response (200 OK):**
```json
{
    "user_id": 1000001,
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2026-03-24T12:00:00Z"
}
```

**Error Response (401 Unauthorized):**
```
HTTP/1.1 401 Unauthorized
X-Error-Message: Invalid username or password
```

---

### GET /users/profile

Get user profile.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "user_id": 1000001,
    "username": "player1",
    "nickname": "Player One",
    "avatar": "https://cdn.example.com/avatars/1000001.png",
    "level": 10,
    "exp": 15000
}
```

**Error Response (401 Unauthorized):**
```
HTTP/1.1 401 Unauthorized
X-Error-Message: Token expired or invalid
```

---

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

**Success Response (200 OK):**
```json
{
    "user_id": 1000001,
    "nickname": "New Nickname",
    "avatar": "https://cdn.example.com/new-avatar.png",
    "bio": "I love gaming!",
    "updated_at": "2026-03-23T12:00:00Z"
}
```

---

### GET /users/friends

Get friends list.

**Headers:**
```
Authorization: Bearer <token>
```

**Query Parameters:**
- `page` (int): Page number (default: 1)
- `limit` (int): Items per page (default: 20)

**Success Response (200 OK):**
```json
{
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
```

---

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

**Success Response (200 OK):**
```json
{
    "request_id": "req_1234567890",
    "from_user_id": 1000001,
    "to_user_id": 1000002,
    "status": "pending",
    "created_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (404 Not Found):**
```
HTTP/1.1 404 Not Found
X-Error-Message: User not found: user_id=1000002
```

**Error Response (409 Conflict):**
```
HTTP/1.1 409 Conflict
X-Error-Message: Friend request already sent
```

---

## Game Service (Port 8002)

### GET /games

List available games.

**Query Parameters:**
- `page` (int): Page number
- `limit` (int): Items per page
- `type` (string): Filter by game type

**Success Response (200 OK):**
```json
{
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
```

---

### GET /games/{game_id}

Get game details.

**Success Response (200 OK):**
```json
{
    "game_id": 1,
    "game_code": "texas-holdem",
    "game_name": "Texas Hold'em",
    "description": "Classic poker game",
    "rules": { ... },
    "config": { ... }
}
```

**Error Response (404 Not Found):**
```
HTTP/1.1 404 Not Found
X-Error-Message: Game not found: game_id=999
```

---

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

**Success Response (200 OK):**
```json
{
    "room_id": "room_1234567890",
    "game_id": 1,
    "room_name": "My Poker Room",
    "host_id": 1000001,
    "player_count": 1,
    "max_players": 6,
    "status": "waiting",
    "created_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (400 Bad Request):**
```
HTTP/1.1 400 Bad Request
X-Error-Message: Invalid max_players: must be between 2 and 10
```

---

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

**Success Response (200 OK):**
```json
{
    "room_id": "room_1234567890",
    "player_id": 1000001,
    "joined_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (403 Forbidden):**
```
HTTP/1.1 403 Forbidden
X-Error-Message: Room is full
```

**Error Response (401 Unauthorized):**
```
HTTP/1.1 401 Unauthorized
X-Error-Message: Invalid room password
```

---

### POST /games/rooms/{room_id}/leave

Leave a game room.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "room_id": "room_1234567890",
    "player_id": 1000001,
    "left_at": "2026-03-23T12:00:00Z"
}
```

---

### POST /games/rooms/{room_id}/start

Start a game (host only).

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "room_id": "room_1234567890",
    "game_id": 1,
    "status": "playing",
    "started_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (403 Forbidden):**
```
HTTP/1.1 403 Forbidden
X-Error-Message: Only host can start the game
```

---

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

**Success Response (200 OK):**
```json
{
    "action_id": "act_1234567890",
    "player_id": 1000001,
    "action": "bet",
    "amount": 100,
    "processed_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (400 Bad Request):**
```
HTTP/1.1 400 Bad Request
X-Error-Message: Invalid action: not your turn
```

---

## Payment Service (Port 8003)

### GET /scores

Get user score balance.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "user_id": 1000001,
    "balance": 50000,
    "currency": "coins"
}
```

---

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

**Success Response (200 OK):**
```json
{
    "order_no": "ORD20260323120000ABC",
    "amount": 9900,
    "currency": "USD",
    "status": "pending",
    "payment_url": "https://payment.example.com/..."
}
```

---

### GET /orders/{order_no}

Get order status.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "order_no": "ORD20260323120000ABC",
    "product_type": "coin_package",
    "product_id": "pkg_1000",
    "amount": 9900,
    "status": "completed",
    "created_at": "2026-03-23T12:00:00Z",
    "updated_at": "2026-03-23T12:01:00Z"
}
```

**Error Response (404 Not Found):**
```
HTTP/1.1 404 Not Found
X-Error-Message: Order not found: order_no=ORD123
```

---

## Player Service (Port 8004)

### GET /players/{player_id}

Get player statistics.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
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
```

---

### GET /players/{player_id}/history

Get player game history.

**Query Parameters:**
- `game_id` (int): Filter by game
- `page` (int): Page number
- `limit` (int): Items per page

**Success Response (200 OK):**
```json
{
    "total": 500,
    "page": 1,
    "limit": 20,
    "history": [
        {
            "game_id": 1,
            "room_id": "room_123",
            "played_at": "2026-03-23T12:00:00Z",
            "result": "win",
            "score_change": 500
        }
    ]
}
```

---

## Guild Service (Port 8006)

### GET /guilds

List guilds.

**Query Parameters:**
- `page` (int): Page number
- `limit` (int): Items per page

**Success Response (200 OK):**
```json
{
    "total": 100,
    "page": 1,
    "limit": 20,
    "guilds": [
        {
            "guild_id": 1,
            "guild_name": "Dragon Slayers",
            "member_count": 50,
            "level": 5
        }
    ]
}
```

---

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
    "emblem": "https://cdn.example.com/guilds/dragon-slayers.png"
}
```

**Success Response (200 OK):**
```json
{
    "guild_id": 101,
    "guild_name": "Dragon Slayers",
    "leader_id": 1000001,
    "created_at": "2026-03-23T12:00:00Z"
}
```

---

### GET /guilds/{guild_id}

Get guild details.

**Success Response (200 OK):**
```json
{
    "guild_id": 101,
    "guild_name": "Dragon Slayers",
    "description": "We hunt dragons!",
    "emblem": "https://cdn.example.com/guilds/dragon-slayers.png",
    "leader_id": 1000001,
    "member_count": 50,
    "level": 5,
    "created_at": "2026-03-01T12:00:00Z"
}
```

---

### POST /guilds/{guild_id}/join

Request to join a guild.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "request_id": "guild_req_123",
    "guild_id": 101,
    "user_id": 1000001,
    "status": "pending",
    "created_at": "2026-03-23T12:00:00Z"
}
```

---

### POST /guilds/{guild_id}/members/{user_id}/kick

Kick a member (guild leader only).

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "guild_id": 101,
    "user_id": 1000002,
    "kicked_at": "2026-03-23T12:00:00Z"
}
```

**Error Response (403 Forbidden):**
```
HTTP/1.1 403 Forbidden
X-Error-Message: Only guild leader can kick members
```

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

**Success Response (200 OK):**
```json
{
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
```

---

### PUT /notifications/{id}/read

Mark notification as read.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "id": 1,
    "read": true,
    "read_at": "2026-03-23T12:00:00Z"
}
```

---

### PUT /notifications/read-all

Mark all notifications as read.

**Headers:**
```
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
    "count": 5,
    "read_at": "2026-03-23T12:00:00Z"
}
```

---

## ID Service (Port 8011)

### GET /id/generate

Generate a unique ID.

**Success Response (200 OK):**
```json
{
    "id": 1000000000000000001
}
```

---

### POST /id/batch

Batch generate IDs.

**Request Body:**
```json
{
    "count": 100
}
```

**Success Response (200 OK):**
```json
{
    "ids": [1000000000000000001, 1000000000000000002, ...]
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

**Server -> Client (Error):**
```json
{
    "type": "error",
    "code": 4003,
    "message": "Room not found",
    "time": "2026-03-23T12:00:00Z"
}
```

### Message Types

- `room_join`: Join a room
- `room_leave`: Leave a room
- `room_msg`: Send message to room
- `ping`: Ping server
- `pong`: Pong response
- `error`: Error message

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

**Rate Limit Exceeded (429):**
```
HTTP/1.1 429 Too Many Requests
X-Error-Message: Rate limit exceeded: 100 requests per minute
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1648000000
```

---

## Pagination

List endpoints support pagination:

**Query Parameters:**
- `page` (int): Page number (default: 1)
- `limit` (int): Items per page (default: 20, max: 100)

**Response Format:**
```json
{
    "total": 250,
    "page": 1,
    "limit": 20,
    "has_more": true,
    "items": [...]
}
```
