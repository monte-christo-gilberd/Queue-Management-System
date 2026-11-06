package router

import "github.com/gin-gonic/gin"

type TicketHandler interface {
	Join(c *gin.Context)
	Preview(c *gin.Context)
	Next(c *gin.Context)
	Skip(c *gin.Context)
	Cancel(c *gin.Context)
	Remove(c *gin.Context)
	Insert(c *gin.Context)
	Recall(c *gin.Context)
	List(c *gin.Context)
	Status(c *gin.Context)
	MyTickets(c *gin.Context)
}

type UserHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Me(c *gin.Context)
	UpdateMe(c *gin.Context)
	UpdatePreferences(c *gin.Context)
	UploadAvatar(c *gin.Context)
	GetAvatar(c *gin.Context)
	DeleteAvatar(c *gin.Context)
}

type PasswordHandler interface {
	ForgotPassword(c *gin.Context)
	ResetPassword(c *gin.Context)
}

type EmailVerificationHandler interface {
	Verify(c *gin.Context)
	Resend(c *gin.Context)
}

type DisplayHandler interface {
	Get(c *gin.Context)
	Update(c *gin.Context)
	Rotate(c *gin.Context)
	Disable(c *gin.Context)
	Snapshot(c *gin.Context)
	Events(c *gin.Context)
}

type QueueHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Close(c *gin.Context)
	GetQR(c *gin.Context)
	SetQRType(c *gin.Context)
	ReplaceQR(c *gin.Context)
	GetSkipSettings(c *gin.Context)
	UpdateSkipSettings(c *gin.Context)
}

type PushHandler interface {
	PublicKey(c *gin.Context)
	Subscribe(c *gin.Context)
	Unsubscribe(c *gin.Context)
}

type SSEHandler interface {
	ServeSSE(c *gin.Context)
}

type CounterHandler interface {
	List(c *gin.Context)
	Create(c *gin.Context)
	Rename(c *gin.Context)
	Delete(c *gin.Context)
}

type NoteHandler interface {
	Get(c *gin.Context)
	Save(c *gin.Context)
}

type HistoryHandler interface {
	ListQueues(c *gin.Context)
	GetQueue(c *gin.Context)
	Export(c *gin.Context)
	ListMine(c *gin.Context)
}

type AppRouter struct {
	counterHandler  CounterHandler
	historyHandler  HistoryHandler
	noteHandler     NoteHandler
	displayHandler  DisplayHandler
	pushHandler     PushHandler
	passwordHandler PasswordHandler
	// verifyHandler: verifikasi email (opsional; tanpa ini rute /users/verify-email tidak ada).
	verifyHandler  EmailVerificationHandler
	ticketHandler  TicketHandler
	userHandler    UserHandler
	queueHandler   QueueHandler
	sseHandler     SSEHandler
	authMiddleware gin.HandlerFunc
	allowedOrigins []string
}
