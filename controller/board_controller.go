package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// BoardController handles HTTP requests for the /boards resource.
type BoardController struct {
	svc *service.BoardService
}

// NewBoardController creates a BoardController wired to the provided service.
func NewBoardController(svc *service.BoardService) *BoardController {
	return &BoardController{svc: svc}
}

// createBoardRequest is the expected JSON body for POST /boards.
type createBoardRequest struct {
	Title       string `json:"title"       binding:"required"`
	Description string `json:"description"`
}

// CreateBoard godoc
// POST /boards
// Creates a new board and returns the persisted model.
func (bc *BoardController) CreateBoard(c *gin.Context) {
	var req createBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(constants.LogTagBoard, "invalid request body", err)
		c.JSON(http.StatusBadRequest, data.Fail("invalid request body: "+err.Error()))
		return
	}

	board, err := bc.svc.CreateBoard(c.Request.Context(), req.Title, req.Description)
	if err != nil {
		utils.Error(constants.LogTagBoard, "CreateBoard failed", err)
		c.JSON(http.StatusInternalServerError, data.Fail(err.Error()))
		return
	}

	utils.Info(constants.LogTagBoard, "board created", board.ID)
	c.JSON(http.StatusCreated, data.OK(board))
}

// GetBoards godoc
// GET /boards
// Returns all boards.
func (bc *BoardController) GetBoards(c *gin.Context) {
	boards, err := bc.svc.GetBoards(c.Request.Context())
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoards failed", err)
		c.JSON(http.StatusInternalServerError, data.Fail(err.Error()))
		return
	}

	c.JSON(http.StatusOK, data.OK(boards))
}
