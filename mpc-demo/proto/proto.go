package proto

const (
	StatusOK       = "ok"
	StatusWaiting  = "waiting"
	StatusUnlocked = "unlocked"
	StatusRejected = "rejected"
	StatusTimeout  = "timeout"
	StatusError    = "error"
)

// AuthRequest is sent from client → server when a party wants to authenticate.
type AuthRequest struct {
	Party string `json:"party"`
	PIN   string `json:"pin"`
}

// ServerMessage is sent from server → client for every state update.
type ServerMessage struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Secret  string `json:"secret,omitempty"`
}
