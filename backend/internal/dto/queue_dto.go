package dto

type CreateQueueRequest struct {
	QueueName        string `json:"queue_name" binding:"required"`
	QueueDescription string `json:"queue_description"`
}

type UpdateQueueRequest struct {
	QueueName        string `json:"queue_name"`
	QueueDescription string `json:"queue_description"`
}

type UpdateAuthzRequest struct {
	QueueID           int64 `json:"queue_id"`
	CanManageQueue    bool  `json:"can_manage_queue"`
	CanManageCounters bool  `json:"can_manage_counters"`
	CanManageQr       bool  `json:"can_manage_qr"`
	CanViewQueue      bool  `json:"can_view_queue"`
	CanManageDisplay  bool  `json:"can_manage_display"`
	CanContinueQueue  bool  `json:"can_continue_queue"`
	CanNotifyCustomer bool  `json:"can_notify_customer"`
	CanManageTickets  bool  `json:"can_manage_tickets"`
	CanManageNotes    bool  `json:"can_manage_notes"`
}
