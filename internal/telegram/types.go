package telegram

type Message struct {
	ID         int64
	ChatID     int64
	Text       string
	SenderID   int64
	Timestamp  int64
	IsOutgoing bool
	ChatTitle  string
	ChatType   string
}

type User struct {
	ID        int64
	FirstName string
	LastName  string
}

type Chat struct {
	ID       int64
	Title    string
	Type     string
	Username string
}

type HandlerFunc func(*Message)
