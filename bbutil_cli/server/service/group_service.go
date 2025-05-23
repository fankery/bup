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

type GroupService struct{}

var (
	apiResponse = &models.ApiResponse{}
	groupDao    = &sql.GroupDao{}
)

// 查询正常状态的组列表
func (g *GroupService) List(c *gin.Context) {
	list, _ := groupDao.List()
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(list))
}

// 新增组，superAdmin
func (g *GroupService) Add(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("新建组接口参数：%v", param)
	if param["name"] == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组名不能为空"))
		return
	}
	if param["rootName"] == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组管理员名称不能为空"))
		return
	}
	id := util.GetRandInt64()
	group := models.GroupInfo{
		Id:       id,
		Status:   1,
		Name:     util.AnyConvertToString(param["name"]),
		RootName: util.AnyConvertToString(param["rootName"]),
		Chan:     "ALL",
	}
	if err := groupDao.Create(group); err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}

// 删除组
func (g *GroupService) Delete(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("删除组接口参数：%v", param)
	if param["id"] == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组id不能为空"))
		return
	}
	if err := groupDao.Delete(util.AnyConvertToString(param["id"])); err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}

// getWebHook
func (g *GroupService) GetWebHook(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("获取webhook接口参数：%v", param)
	if param["id"] == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组id不能为空"))
		return
	}
	group, _ := groupDao.SelectByPrimaryKey(util.AnyConvertToString(param["id"]))
	resMap := make(map[string]string, 2)
	resMap["all"] = group.AllWebHook
	resMap["replace"] = group.ReplaceWebHook
	resMap["channel"] = group.Chan
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(resMap))
}

// updateWebHook
func (g *GroupService) UpdateWebHook(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("更新webhook接口参数：%v", param)
	if param["id"] == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组id不能为空"))
		return
	}
	allWebHook := util.AnyConvertToString(param["allWebhook"])
	replaceWebHook := util.AnyConvertToString(param["replaceWebhook"])
	channel := util.AnyConvertToString(param["channel"])
	id, _ := strconv.ParseInt(util.AnyConvertToString(param["id"]), 10, 64)
	group := models.GroupInfo{
		Id:             id,
		AllWebHook:     allWebHook,
		ReplaceWebHook: replaceWebHook,
		Chan:           channel,
	}
	if err := groupDao.Update(group); err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}
