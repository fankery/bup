package service

import (
	"bbutil_cli/common"
	"bbutil_cli/server/models"
	"bbutil_cli/server/sql"
	"bbutil_cli/server/util"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TaskService struct{}

var taskDao = &sql.TaskDao{}

// 查询正常状态的任务列表
func (g *TaskService) List(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("打包接口参数：%v", param)
	groupId := util.AnyConvertToString(param["groupId"])
	taskType := util.AnyConvertToString(param["taskType"])
	if groupId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组为空"))
		return
	}
	list, err := taskDao.List(groupId, taskType)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(list))
}

// 查询当前最近的构建任务信息
func (t *TaskService) LatestTaskExecute(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("查询当前最近的构建任务信息接口参数：%v", param)
	groupId := util.AnyConvertToString(param["groupId"])
	taskType := util.AnyConvertToString(param["taskType"])
	if groupId == "" || taskType == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组或任务类型为空"))
		return
	}
	list, err := taskExecuteDao.List(groupId, taskType)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(list))
}

// 创建任务
func (t *TaskService) Create(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("创建任务接口参数：%v", param)
	code := util.AnyConvertToString(param["code"])
	value := util.AnyConvertToString(param["value"])
	groupId := util.AnyConvertToString(param["groupId"])
	taskType := util.AnyConvertToString(param["taskType"])
	pack := util.AnyConvertToString(param["pack"])
	num, _ := strconv.ParseInt(groupId, 10, 64)
	tt, _ := strconv.ParseInt(taskType, 10, 64)
	task := models.TaskInfo{
		Id:      util.GetRandInt64(),
		Status:  1,
		Code:    code,
		Value:   value,
		GroupId: num,
		Type:    tt,
		Pack:    pack,
	}

	if err := taskDao.Create(task); err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}

// 删除任务
func (t *TaskService) Delete(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("删除任务接口参数：%v", param)
	id := util.AnyConvertToString(param["id"])
	if err := taskDao.Delete(id); err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}

// 获取该日期，该替换任务打包的desc
func (t *TaskService) GetDesc(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("获取desc接口参数：%v", param)
	taskId := util.AnyConvertToString(param["taskId"])
	date := util.AnyConvertToString(param["date"])
	if taskId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("任务id不能为空"))
		return
	}
	if date == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("任务日期不能为空"))
		return
	}
	taskExec, err := taskExecuteDao.SelectByTaskIdAndDate(taskId, date)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(taskExec))
}

func (t *TaskService) CleanOverDueExecuteInfo() int64 {
	return taskExecuteDao.CleanOverDueExecuteInfo()
}
