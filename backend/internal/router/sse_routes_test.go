package router

import (
	"context"
	"strconv"
	"time"

	"qms/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type stubTicket struct{}

func (stubTicket) Join(c *gin.Context) {}
func (stubTicket) Next(c *gin.Context) {}

type stubUser struct{}

func (stubUser) Register(c *gin.Context) {}
func (stubUser) Login(c *gin.Context)    {}

type stubQueue struct{}

func (stubQueue) Create(c *gin.Context)      {}
func (stubQueue) Get(c *gin.Context)         {}
func (stubQueue) List(c *gin.Context)        {}
func (stubQueue) Update(c *gin.Context)      {}
func (stubQueue) UpdateAuthz(c *gin.Context) {}
func (stubQueue) Close(c *gin.Context)       {}

type stubValidator struct{}

func (stubValidator) GetQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, model.QueueAuthz, error) {
	return model.Queue{ID: queueID, OwnerID: requesterID}, model.QueueAuthz{}, nil
}

func issueTestToken(secret string, userID int64) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, _ := tok.SignedString([]byte(secret))
	return s
}
