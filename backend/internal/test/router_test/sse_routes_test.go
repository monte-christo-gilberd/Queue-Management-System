package router_test

import (
	"context"

	"qms/internal/model"

	"github.com/gin-gonic/gin"
)

type stubTicket struct{}

func (stubTicket) Join(c *gin.Context)      {}
func (stubTicket) Next(c *gin.Context)      {}
func (stubTicket) Skip(c *gin.Context)      {}
func (stubTicket) Recall(c *gin.Context)    {}
func (stubTicket) List(c *gin.Context)      {}
func (stubTicket) Status(c *gin.Context)    {}
func (stubTicket) MyTickets(c *gin.Context) {}

type stubUser struct{}

func (stubUser) Register(c *gin.Context) {}
func (stubUser) Login(c *gin.Context)    {}
func (stubUser) Me(c *gin.Context)       {}

type stubQueue struct{}

func (stubQueue) Create(c *gin.Context) {}
func (stubQueue) Get(c *gin.Context)    {}
func (stubQueue) List(c *gin.Context)   {}
func (stubQueue) Update(c *gin.Context) {}
func (stubQueue) Close(c *gin.Context)  {}

type stubValidator struct{}

func (stubValidator) ViewQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, bool, error) {
	return model.Queue{ID: queueID, OwnerID: requesterID}, true, nil
}
