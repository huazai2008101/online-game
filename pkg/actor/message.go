package actor

import "time"

// MsgType defines the type of message sent to actors.
type MsgType int

const (
	MsgPlayerJoin  MsgType = iota + 1 // 玩家加入房间
	MsgPlayerLeave                     // 玩家离开房间
	MsgPlayerReady                     // 玩家准备
	MsgPlayerAction                    // 玩家游戏操作
	MsgGameStart                       // 开始游戏
	MsgGameTick                        // 游戏帧（实时游戏用）
	MsgTimer                           // 定时器回调
	MsgRestore                         // 状态恢复（断线重连）
	MsgShutdown                        // 关闭 Actor
)

// Message is the unit of communication between actors.
// All messages are processed sequentially by the receiving actor's goroutine.
type Message struct {
	Type     MsgType
	PlayerID string
	Action   string    // for MsgPlayerAction: the action name (e.g. "bet", "fold")
	Data     any       // payload, type depends on MsgType
	Time     time.Time // when the message was created
}

// NewMessage creates a new message with the current timestamp.
func NewMessage(msgType MsgType, playerID string, data any) *Message {
	return &Message{
		Type:     msgType,
		PlayerID: playerID,
		Data:     data,
		Time:     time.Now(),
	}
}

// ActionMessage creates a player action message.
func ActionMessage(playerID, action string, data any) *Message {
	return &Message{
		Type:     MsgPlayerAction,
		PlayerID: playerID,
		Action:   action,
		Data:     data,
		Time:     time.Now(),
	}
}

// --- Payload types for specific message types ---

// PlayerJoinData carries player join information.
type PlayerJoinData struct {
	Nickname string
	Avatar   string
	Metadata map[string]any
}

// PlayerLeaveData carries player leave reason.
type PlayerLeaveData struct {
	Reason string // "disconnect", "voluntary", "kicked"
}

// TimerData carries timer callback information.
type TimerData struct {
	ID   int
	Fn   any // goja.Value — stored as any to avoid import cycle
	Once bool
}
