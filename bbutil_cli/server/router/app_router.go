/*
Copyright © 2023 LiuHailong
*/
package router

import (
	"bbutil_cli/common"
	"bbutil_cli/server/middleware"
	"bbutil_cli/server/service"
	"bbutil_cli/server/util"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var mode string

func InitRouter(port int64) {
	defer func() {
		if err := recover(); err != nil {
			common.Logger.Error("异常错误：", err)
		}
	}()
	util.DatabaseInit()
	gin.ForceConsoleColor()
	common.Logger.Info("start gin server")
	mode = viper.GetString("server.active")
	common.Logger.Infof("start set mode:  %s", mode)
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	common.Logger.Info("end set mode")
	router := gin.New()
	//自定义日志格式
	router.Use(middleware.GinLogger())
	//	在路由组之前全局使用跨域中间件
	router.Use(middleware.Cors())
	router.Use(gin.Recovery())
	//注册接口
	defineRouter(router)
	//注册定时任务
	go scheduledTask()
	go scheduledcleanExecuteInfoTask()
	common.Logger.Info("success start")
	err := router.Run(fmt.Sprintf("%s:%d", "", port))
	if err != nil {
		common.Logger.Fatal("项目启动失败", err)
	}
}

// 自定义路由
func defineRouter(router *gin.Engine) {
	//组
	groupInfoGroup := router.Group("/bup/group")
	{
		groupService := &service.GroupService{}
		groupInfoGroup.GET("/list", groupService.List)
		groupInfoGroup.POST("/add", groupService.Add)
		groupInfoGroup.POST("/delete", groupService.Delete)
		groupInfoGroup.POST("/getWebhook", groupService.GetWebHook)
		groupInfoGroup.POST("/updateWebhook", groupService.UpdateWebHook)
	}
	//人员
	empInfoGroup := router.Group("/bup/emp")
	{
		empService := &service.EmpService{}
		empInfoGroup.GET("/list", empService.List)
		empInfoGroup.POST("/login", empService.Login)
		empInfoGroup.POST("/update", empService.Update)
		empInfoGroup.POST("/delete", empService.Delete)
	}
	//jenkins
	jenkinsGroup := router.Group("/bup/jenkins")
	{
		jenkinsService := &service.JenkinsService{}
		jenkinsGroup.POST("/build", jenkinsService.BuildJob)
		jenkinsGroup.POST("/stop", jenkinsService.StopJob)
		jenkinsGroup.GET("/ws/:empId/:buildId", jenkinsService.GetJobProgress)
	}
	// 构建任务
	taskGroup := router.Group("/bup/task")
	{
		taskService := &service.TaskService{}
		taskGroup.POST("/laststList", taskService.LatestTaskExecute)
		taskGroup.POST("/list", taskService.List)
		taskGroup.POST("/create", taskService.Create)
		taskGroup.POST("/delete", taskService.Delete)
		taskGroup.POST("/getDesc", taskService.GetDesc)
	}
	// jira
	jiraGroup := router.Group("/bup/jira")
	{
		jiraService := &service.JiraService{}
		jiraGroup.POST("/create", jiraService.CreateJira)
		jiraGroup.POST("/getProject", jiraService.GetProject)
		jiraGroup.POST("/getProjectInfo", jiraService.GetProjectInfo)
		jiraGroup.POST("/testerInfo", jiraService.GetTesterInfo)
	}
}

// 定时任务，清除连接资源
func scheduledTask() {
	//创建定时器
	threeHourTicker := time.NewTicker(3 * time.Hour)
	for {
		<-threeHourTicker.C
		toDeleteKey := []string{}
		toDeleteJiraKey := []string{}
		for k, v := range service.JenkinsConnMap {
			if time.Since(v.InitTime) > 2*time.Hour {
				toDeleteKey = append(toDeleteKey, k)
			}
		}
		for _, v := range toDeleteKey {
			delete(service.JenkinsConnMap, v)
		}
		common.Logger.Infof("清除%d条过期的Jenkins连接数据", len(toDeleteKey))
		for k, v := range service.UserJiraLinkMap {
			if time.Since(v.InitTime) > 1*time.Hour {
				toDeleteJiraKey = append(toDeleteJiraKey, k)
			}
		}
		for _, v := range toDeleteJiraKey {
			delete(service.UserJiraLinkMap, v)
		}
		common.Logger.Infof("清除%d条过期的Jira连接数据", len(toDeleteJiraKey))
	}
}

// 清除过往执行任务的资源
func scheduledcleanExecuteInfoTask() {
	taskService := &service.TaskService{}
	//创建定时器
	ticker := time.NewTicker(24 * time.Hour)
	for {
		<-ticker.C
		num := taskService.CleanOverDueExecuteInfo()
		common.Logger.Infof("清除%d条过期的任务执行数据", num)
	}
}
