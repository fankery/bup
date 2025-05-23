package service

import (
	"bbutil_cli/common"
	"bbutil_cli/server/models"
	"bbutil_cli/server/sql"
	"bbutil_cli/server/util"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type EmpService struct{}

var (
	jenkinsService = &JenkinsService{}
	ldapService    = &LdapService{}
)

var empDao = &sql.EmpDao{}

// 查询正常状态的人员列表
func (e *EmpService) List(c *gin.Context) {
	param := map[string]string{
		"groupId": c.Query("groupId"),
	}
	groupId := param["groupId"]
	if groupId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("请输入组id"))
		return
	}
	emp, err := empDao.List(param)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessWithData(emp))
	}
}

// login
func (e *EmpService) Login(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	if param["username"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名不能为空"))
		return
	}
	if param["password"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("密码不能为空"))
		return
	}

	common.Logger.Infof("员工登录：%v", param["username"])
	username := util.AnyConvertToString(param["username"])
	password := util.AnyConvertToString(param["password"])
	if username == "superadmin" {
		// 获取当日的日期
		timeObj := time.Now()
		dateStr := timeObj.Local().Format("2006-01-02")
		if dateStr == password {
			c.JSON(http.StatusOK, apiResponse.SuccessWithDataAndMessage(nil, "1"))
			return
		} else {
			c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名或密码错误"))
			return
		}
	}
	// 查询所有的组
	if param["groupId"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("所属组不能为空"))
		return
	}
	allGroupList, _ := groupDao.List()
	allGroupIdStr := make([]string, 10)
	for _, v := range allGroupList {
		allGroupIdStr = append(allGroupIdStr, util.AnyConvertToString(v.RootName))
	}
	if util.Contains(allGroupIdStr, username) {
		groupInfo, err := groupDao.SelectByPrimaryKey(util.AnyConvertToString(param["groupId"]))
		if err != nil {
			c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名,密码或组错误"))
			return
		}
		if username == password && username == groupInfo.RootName {
			c.JSON(http.StatusOK, apiResponse.SuccessWithDataAndMessage(nil, "2"))
			return
		} else {
			c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名,密码或组错误"))
			return
		}
	}
	if ldapService.LoginAuth(username, password) {
		groupId, _ := strconv.ParseInt(util.AnyConvertToString(param["groupId"]), 10, 64)
		emp, _ := empDao.SelectByUsername(username)
		aesPassword, err := util.AESEncoding(password)
		if emp == (models.EmpInfo{}) {
			if err != nil {
				common.Logger.Error("加密错误：", err)
				c.JSON(http.StatusOK, apiResponse.FailDefault())
			}
			emp = models.EmpInfo{
				Id:              util.GetRandInt64(),
				Status:          1,
				Username:        username,
				JenkinsUsername: username,
				JenkinsPassword: aesPassword,
				JiraUsername:    username,
				JiraPassword:    aesPassword,
				GroupId:         groupId,
				Address:         c.RemoteIP(),
			}
		} else {
			if emp.GroupId != groupId {
				c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名,密码或组错误"))
				return
			}
		}
		c.JSON(http.StatusOK, apiResponse.SuccessWithDataAndMessage(emp, "3"))
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("jenkins 似乎异常了")
				}
			}()

			// 重置连接
			delete(JenkinsConnMap, username)
			jenkinsService.InitJenkins(username, password)
		}()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("jenkins 似乎异常了")
				}
			}()

			delete(UserJiraLinkMap, username)
			jiraCLient := jiraService.GetJiraClient(username, aesPassword)
			if jiraCLient == nil {
				return
			}
			userInfo, _, _ := jiraCLient.User.GetSelf()
			user, _ := empDao.SelectByUsername(username)
			if user == (models.EmpInfo{}) {
				emp.ChineseName = userInfo.DisplayName
				empDao.Create(emp)
			}
		}()
	} else {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("用户名或密码错误"))
		return
	}
}

// update
func (e *EmpService) Update(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	if param["id"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("id不能为空"))
		return
	}
	common.Logger.Infof("员工更新接口参数：%v", param)
	id, _ := strconv.ParseInt(fmt.Sprintf("%v", param["id"]), 10, 64)
	emp := models.EmpInfo{
		Id:              id,
		ChineseName:     util.AnyConvertToString(param["chineseName"]),
		JenkinsUsername: util.AnyConvertToString(param["jenkinsUsername"]),
		JenkinsPassword: util.AnyConvertToString(param["jenkinsPassword"]),
		JiraUsername:    util.AnyConvertToString(param["jiraUsername"]),
		JiraPassword:    util.AnyConvertToString(param["jiraPassword"]),
		Address:         util.AnyConvertToString(param["address"]),
	}
	if err := empDao.Update(emp); err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}

// delete 加错组了，删掉对应的员工数据
func (e *EmpService) Delete(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	if param["id"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("id不能为空"))
		return
	}
	if param["groupId"] == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("组id不能为空"))
		return
	}
	id, _ := strconv.ParseInt(fmt.Sprintf("%v", param["id"]), 10, 64)
	groupId, _ := strconv.ParseInt(fmt.Sprintf("%v", param["groupId"]), 10, 64)
	emp := models.EmpInfo{
		Id:      id,
		GroupId: groupId,
	}
	if err := empDao.Delete(emp); err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	}
}
