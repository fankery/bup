package service

import (
	"bbutil_cli/common"
	"bbutil_cli/server/models"
	"bbutil_cli/server/sql"
	"bbutil_cli/server/util"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bndr/gojenkins"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type JenkinsService struct{}

type JenkinsConn struct {
	jenConn  *gojenkins.Jenkins
	InitTime time.Time
	ctx      context.Context
}

const PACK_PATH = "http://172.31.3.252:8082/"

// Constants for model types to improve readability
const (
	ModelWholePack = 1
	ModelReplace   = 2
)

// Pre-compile regexes for performance
var (
	// Regexes for getPack
	execRegex        = regexp.MustCompile(`(\s+\[exec\]\s+)(appmodule-.*\.tar\.gz)`)
	buildingTarRegex = regexp.MustCompile(`(\[INFO\] Building tar .*target/)(.*\.tar\.gz)`)

	// Regexes for getAIAgentPack
	aiVersionRegex  = regexp.MustCompile(`\[INFO\] Building\s+\S+\s+([0-9]+\.[0-9]+\.[0-9]+)`)
	aiRevisionRegex = regexp.MustCompile(`At revision (\d+)`)
	aiUserRe        = regexp.MustCompile(`(?s)Started by user.*?in workspace ([^\s]+)`)
)

// Project specific configurations for getAIAgentPack
var (
	projectAlias = map[string]string{
		"qc-ai-agent":    "ai_agent",
		"statsvr-ai-dev": "answer_ai",
		// Add other mappings here
	}
	// For projects where SVN revision needs to be incremented for the build number
	// Using a map for efficient lookups if the list grows
	projectVersionPlusOne = map[string]struct{}{
		"logcenter": {},
		// Add other projects here
	}
)

var jiraService = &JiraService{}

var taskExecuteDao = &sql.TaskExecuteDao{}

// jenkins 连接
var JenkinsConnMap = make(map[string]JenkinsConn, 0)

// websocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在这里检查请求的来源，如果来源合法则返回 true，否则返回 false
		// 例如，允许所有来源可以使用下面的代码：
		return true
	},
}

// token
var token_str string

// token_init_time
var token_init_time time.Time

// token_lock
var mutex sync.Mutex

// 初始化jenkins连接，设置有效期为6个小时，使用时判断是否过期被动删除
func (j *JenkinsService) InitJenkins(jenkinsUsername, jenkinsPassword string) error {
	ctx := context.Background()
	jenkins := gojenkins.CreateJenkins(nil, "http://172.31.3.252:9080", jenkinsUsername, jenkinsPassword)
	_, err := jenkins.Init(ctx)
	if err != nil {
		common.Logger.Error("jenjkins conn err,", err)
		return err
	} else {
		jenCon := JenkinsConn{
			jenConn:  jenkins,
			InitTime: time.Now(),
			ctx:      ctx,
		}
		JenkinsConnMap[jenkinsUsername] = jenCon
		common.Logger.Infof("jenkins conn init, user is %s", jenkinsUsername)
		return nil
	}
}

// 打包
func (j *JenkinsService) BuildJob(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("打包接口参数：%v", param)
	empId := util.AnyConvertToString(param["empId"])
	taskId := util.AnyConvertToString(param["taskId"])
	desc := util.AnyConvertToString(param["desc"])
	jira := util.AnyConvertToString(param["jira"])
	testerName := util.AnyConvertToString(param["testerName"])
	testerCode := util.AnyConvertToString(param["testerCode"])
	date := util.AnyConvertToString(param["date"])
	fileOrNum := util.AnyConvertToString(param["mode"])

	emp, err := empDao.SelectByPrimaryKey(empId)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if emp == (models.EmpInfo{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该员工不存在或已删除"))
		return
	}

	task, err := taskDao.SelectByPrimaryKey(taskId)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if task == (models.TaskInfo{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该任务不存在或已删除"))
		return
	}
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	// 将最近使用的jira更新到任务表中
	task.Jira = jira
	taskDao.Update(task)
	buildId, err := j.build(emp.JenkinsUsername, emp.JenkinsPassword, task.Value, desc, fileOrNum, task.Type)
	id := util.GetRandInt64()
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	} else {
		var dateP *string = &date
		if date == "" {
			*dateP = time.Now().Format("2006-01-02")
		}

		taskExecute := models.TaskExecute{
			Id:            id,
			Status:        1,
			EmpId:         emp.Id,
			StartTime:     time.Now().Format("2006-01-02 15:04:05"),
			Jira:          jira,
			TesterName:    testerName,
			TesterCode:    testerCode,
			TaskId:        task.Id,
			Date:          *dateP,
			ExecuteStatus: 1,
			Desc:          desc,
			BuildId:       buildId,
		}
		if err := taskExecuteDao.Create(taskExecute); err != nil {
			common.Logger.Error(err)
			c.JSON(http.StatusOK, apiResponse.FailDefault())
			return
		} else {
			c.JSON(http.StatusOK, apiResponse.SuccessWithData(util.AnyConvertToString(buildId)))
			go func() {
				defer func() {
					if r := recover(); r != nil {
						common.Logger.Error("jenkins 似乎异常了")
					}
				}()

				connInfo := JenkinsConnMap[emp.JenkinsUsername]
				build, err := connInfo.jenConn.GetBuildFromQueueID(connInfo.ctx, buildId)
				if err != nil {
					common.Logger.Error(err)
					return
				}
				j.afterUpdate(build, connInfo.ctx, id, emp, jira, desc, testerName, testerCode)
			}()
			return
		}

	}
}

// 打包动作
func (j *JenkinsService) build(username, password, jobName, replaceList, fileOrNum string, mode int64) (int64, error) {
	err := j.resetConn(username, password)
	if err != nil {
		return 0, err
	}
	// 校验是否存在该job
	connInfo := JenkinsConnMap[username]
	job, err := connInfo.jenConn.GetJob(connInfo.ctx, jobName)
	if err != nil {
		common.Logger.Error(err)
		return 0, err
	}
	if job == nil {
		return 0, errors.New("不存在该job")
	}
	param := make(map[string]string, 4)
	if mode == 1 {
		param["FullOrReplacefile"] = "FullPackage"
	} else {
		param["FullOrReplacefile"] = "FileListReplacePackage"
		if fileOrNum == "num" {
			param["FullOrReplacefile"] = "BuildNumberReplacePackage"
		}

	}
	var replaceListArr []string
	if replaceList != "" {
		if fileOrNum == "file" {
			common.Logger.Debug("选择通过文件列表打替换包")
			replaceListArr = strings.Split(replaceList, "\n")
			replaceList = ""
			for _, v := range replaceListArr {
				v = strings.Trim(v, " ")
				isComment := strings.HasPrefix(v, "#")
				if !isComment {
					replaceList = v + "\n" + replaceList
				}
			}
			param["ReplaceFileList"] = replaceList
		} else if fileOrNum == "num" {
			common.Logger.Debug("选择通过build号打替换包")
			replaceListArr = strings.Split(replaceList, "\n")
			replaceList = ""
			for _, v := range replaceListArr {
				v = strings.Trim(v, " ")
				isComment := strings.HasPrefix(v, "#")
				if !isComment {
					replaceList = v + "," + replaceList
				}
			}
			param["ReplaceBuildNumber"] = replaceList
		}
	}
	common.Logger.Debugf("打包动作参数：%v", param)
	id, err := connInfo.jenConn.BuildJob(connInfo.ctx, jobName, param)
	if id != 0 {
		return id, nil
	}

	return 0, err
}

// 校验连接的有效性，重置连接
func (j *JenkinsService) resetConn(username, password string) error {
	password, err := util.AESDecoding(password)
	if err != nil {
		return err
	}
	// 判断连接map中是否已包含对应的连接，或者连接是否已经过期
	connInfo, ok := JenkinsConnMap[username]
	if ok {
		// 存在判断是否过期
		if time.Since(connInfo.InitTime) > 1*time.Hour {
			return j.InitJenkins(username, password)
		}
	} else {
		return j.InitJenkins(username, password)
	}
	return nil
}

// ws连接资源
var WsConnMap = make(map[string]*websocket.Conn)

// websocket 获取当前任务的构建进度
func (j *JenkinsService) GetJobProgress(c *gin.Context) {
	common.Logger.Infof("ip: %s, ws连接", c.RemoteIP())
	empId := c.Param("empId")
	buildId := c.Param("buildId")
	if empId == "" || buildId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工或任务id为空"))
	}
	oldConn, exist := WsConnMap[c.RemoteIP()+buildId]
	if exist {
		// 存在连接，立即将当前连接关闭
		common.Logger.Debugf("已存在ws连接资源%s-%s,先将当前资源关闭", c.RemoteIP(), buildId)
		err := oldConn.Close()
		if err != nil {
			common.Logger.Error("ws资源关闭错误", err)
		}
		common.Logger.Debug("ws连接资源关闭成功")
	}
	// 检查WebSocket握手连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.Logger.Errorf("Error upgrading to WebSocket:", err)
		return
	}
	WsConnMap[c.RemoteIP()+buildId] = conn
	defer func() {
		delete(WsConnMap, c.RemoteIP()+buildId)
		conn.Close()
		common.Logger.Debug("释放ws连接资源")
	}()
	emp, err := empDao.SelectByPrimaryKey(empId)
	if err != nil {
		conn.WriteJSON(apiResponse.FailDefault())
		return
	}
	if emp == (models.EmpInfo{}) {
		conn.WriteJSON(apiResponse.FailWithMessage("该员工不存在或已删除"))
		return
	}
	job, err := taskExecuteDao.SelectByBuildId(buildId)
	if err != nil {
		conn.WriteJSON(apiResponse.FailDefault())
		return
	}
	if job == (models.TaskExecute{}) {
		conn.WriteJSON(apiResponse.FailWithMessage("该任务不存在或已删除"))
		return
	}
	task, err := taskDao.SelectByPrimaryKey(util.AnyConvertToString(job.TaskId))
	if err != nil {
		conn.WriteJSON(apiResponse.FailDefault())
		return
	}
	if task == (models.TaskInfo{}) {
		conn.WriteJSON(apiResponse.FailWithMessage("该任务不存在或已删除"))
		return
	}
	// pwd, err := util.AESDecoding(emp.JenkinsPassword)
	if err != nil {
		conn.WriteJSON(apiResponse.FailDefault())
		return
	}
	j.resetConn(emp.JenkinsUsername, emp.JenkinsPassword)
	num, _ := strconv.ParseInt(buildId, 10, 64)
	channel := make(chan string)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				common.Logger.Error("jenkins 似乎异常了")
			}
		}()
		j.GetJobProgressCore(emp.JenkinsUsername, task.Value, job.StartTime, num, job.Id, channel)
	}()

	for {
		num, ok := <-channel
		if !ok {
			return
		}
		if _, err := strconv.ParseFloat(num, 64); err == nil {
			conn.WriteJSON(apiResponse.SuccessWithData(num))
		} else {
			conn.WriteJSON(apiResponse.FailWithMessage(num))
		}
		if err != nil {
			common.Logger.Error(err)
			conn.WriteJSON(apiResponse.FailDefault())
			return
		}

	}
}

// 获取该任务之前构建成功的时间，用来计算进度条
func (j *JenkinsService) getLastSuccessTime(jenkinsUsername, jobName string) float64 {
	connInfo := JenkinsConnMap[jenkinsUsername]
	job, err := connInfo.jenConn.GetJob(connInfo.ctx, jobName)
	if err != nil {
		common.Logger.Error(err)
		return -1
	}
	build, err := job.GetLastSuccessfulBuild(connInfo.ctx)
	if err != nil {
		common.Logger.Error(err)
		return -1
	}
	return build.GetDuration()
}

// 获取构建进度核心逻辑
func (j *JenkinsService) GetJobProgressCore(jenkinsUsername, taskName, startTimeStr string, buildNum, jobId int64, channel chan string) {
	defer func() {
		common.Logger.Debug("关闭channel")
		close(channel)
	}()
	connInfo := JenkinsConnMap[jenkinsUsername]
	build, err := connInfo.jenConn.GetBuildFromQueueID(connInfo.ctx, buildNum)
	if err != nil {
		common.Logger.Error(err)
		return
	}
	totalTime := j.getLastSuccessTime(jenkinsUsername, taskName)
	startTime, _ := time.Parse("2006-01-02 15:04:05", startTimeStr)
	for build.IsRunning(connInfo.ctx) {
		time.Sleep(3000 * time.Millisecond)
		// 获取当前job的任务状态，返回给前端,计算进度条
		build.Poll(connInfo.ctx)
		now := time.Now().Format("2006-01-02 15:04:05")
		nowTime, _ := time.Parse("2006-01-02 15:04:05", now)
		balance := nowTime.Sub(startTime).Milliseconds()
		progressValue := float64(float64(balance)/totalTime) * 100
		if progressValue >= 100 {
			progressValue = 99.99
		}
		if err != nil {
			common.Logger.Error(err)
			return
		}
		channel <- fmt.Sprintf("%.2f", progressValue)
	}
	if build.GetResult() == "SUCCESS" {
		common.Logger.Infof("emp=%s, job=%s, EXECUTE SUCCESS", jenkinsUsername, taskName)
		time.Sleep(500 * time.Millisecond)
		taskInfo, _ := taskExecuteDao.SelectByPrimaryKey(util.AnyConvertToString(jobId))
		if taskInfo.ExecuteStatus == 1 {
			taskExecute := models.TaskExecute{
				Id:            jobId,
				ExecuteStatus: 2,
			}
			taskExecuteDao.Update(taskExecute)
		}
		channel <- "100.00"
	} else if build.GetResult() == "ABORTED" {
		resultStr := build.GetConsoleOutput(connInfo.ctx)
		stopMan := j.getStopMan(resultStr)
		channel <- "该任务被 " + stopMan + " 停止"
	} else {
		errMsg := j.getErrorMsg(build.GetConsoleOutput(connInfo.ctx))
		channel <- errMsg
	}
}

// 获取构建成功后打包的资源路径
// getPack attempts to find a package path from Jenkins output log using predefined patterns.
func (j *JenkinsService) getPack(model int64, pack, outputLog string) string {
	// Helper struct to define patterns and their extraction logic
	type logPattern struct {
		regex *regexp.Regexp
		// Potentially add a custom extraction func if logic differs significantly
	}

	patterns := []logPattern{
		{regex: execRegex},
		{regex: buildingTarRegex},
	}

	basePath := PACK_PATH + pack + "/" // Assuming PACK_PATH and pack form a URL-like prefix

	for _, p := range patterns {
		match := p.regex.FindStringSubmatch(outputLog)
		if len(match) == 3 { // Expecting full match + 2 capturing groups
			filename := match[2] // The captured filename
			if model == ModelReplace {
				return basePath + "replacefile-" + filename
			}
			return basePath + filename
		}
	}

	// Fallback to AI Agent specific packing logic
	return j.getJarPack(outputLog, pack)
}

// getAIAgentPack constructs a package path for AI Agent type projects.
func (j *JenkinsService) getJarPack(outputLog string, pack string) string {
	// 1. Extract workspace path
	userMatch := aiUserRe.FindStringSubmatch(outputLog)
	if len(userMatch) < 2 {
		log.Printf("getAIAgentPack: Could not find user workspace path in log.")
		return ""
	}
	fullPath := userMatch[1]

	// 2. Extract project name from the workspace path
	// project := filepath.Base(fullPath) // More robust for actual file paths
	// If fullPath is more like a URL segment or always uses '/', strings.Split is fine
	parts := strings.Split(fullPath, "/")
	project := parts[len(parts)-1]
	project = strings.TrimPrefix(project, "platform-")

	// 3. Apply project alias
	if alias, exists := projectAlias[project]; exists {
		project = alias
	}

	// 4. Extract version
	versionMatch := aiVersionRegex.FindStringSubmatch(outputLog)
	if len(versionMatch) < 2 {
		log.Printf("getAIAgentPack: Could not find version for project %s in log.", project)
		return ""
	}
	version := versionMatch[1]

	// 5. Extract SVN revision
	revisionMatch := aiRevisionRegex.FindStringSubmatch(outputLog)
	if len(revisionMatch) < 2 {
		log.Printf("getAIAgentPack: Could not find SVN revision for project %s in log.", project)
		return ""
	}
	svnRevisionStr := revisionMatch[1]

	// 6. Increment SVN revision if project requires it
	if _, exists := projectVersionPlusOne[project]; exists {
		if verInt, err := strconv.Atoi(svnRevisionStr); err == nil {
			svnRevisionStr = strconv.Itoa(verInt + 1)
		} else {
			log.Printf("getAIAgentPack: Could not parse SVN revision '%s' as integer for project %s.", svnRevisionStr, project)
			// Decide if this is a fatal error for this function; current logic proceeds with original revision.
		}
	}

	// 7. Format build date
	buildDate := time.Now().Format("20060102") // YYYYMMDD

	// 8. Construct filename
	filename := fmt.Sprintf("%s-%s-build%s-%s.tar.gz", project, version, svnRevisionStr, buildDate)

	// 9. Join with pack path (assuming PACK_PATH and pack form a URL-like prefix)
	// If it were a filesystem path, filepath.Join would be better:
	// return filepath.Join(PACK_PATH, pack, filename)
	return PACK_PATH + pack + "/" + filename
}

// 任务是被人停止时，找到执行停止动作的人
func (j *JenkinsService) getStopMan(outputLog string) string {
	regex := regexp.MustCompile(`(\s*Aborted by )(.+)`)
	match := regex.FindStringSubmatch(outputLog)
	if len(match) == 3 {
		return match[2]
	} else {
		return "神秘人"
	}
}

// 任务是异常停止的，获取对应的错误信息
func (j *JenkinsService) getErrorMsg(outputLog string) string {
	regex := regexp.MustCompile(`(?m)^\[ERROR\].*$`)
	matchs := regex.FindAllString(outputLog, -1)
	errMsg := "\n"
	for _, match := range matchs {
		errMsg = errMsg + match + "\n"
	}
	return errMsg
}

// 异步去检测当前任务的状态
// 任务停止运行时，根据结果是否为SUCCESS更新任务执行表的任务状态
func (j *JenkinsService) afterUpdate(build *gojenkins.Build, ctx context.Context, jobId int64, emp models.EmpInfo, jira, desc, testerName, testerCode string) error {
	startTime := time.Now()
	for build.IsRunning(ctx) {
		time.Sleep(3000 * time.Millisecond)
		build.Poll(ctx)
	}
	consumeTime := time.Since(startTime).Milliseconds() / 1000
	if build.GetResult() == "SUCCESS" {
		taskExecute := models.TaskExecute{
			Id:            jobId,
			ExecuteStatus: 2,
			ConsumeTime:   consumeTime,
		}
		err := taskExecuteDao.Update(taskExecute)
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		taskExec, err := taskExecuteDao.SelectByPrimaryKey(util.AnyConvertToString(jobId))
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		task, err := taskDao.SelectByPrimaryKey(util.AnyConvertToString(taskExec.TaskId))
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		packUrl := j.getPack(task.Type, task.Pack, build.GetConsoleOutput(ctx))
		if strings.HasPrefix(jira, "http") {
			jira = strings.ReplaceAll(jira, JiraURL+"browse/", "")
			isSuccess := jiraService.AddCommand(jira, packUrl, emp.JiraUsername, emp.JiraPassword)
			if !isSuccess {
				common.Logger.Error("粘贴jira失败")
				return nil
			}
		} else {
			isSuccess := jiraService.AddCommand(jira, packUrl, emp.JiraUsername, emp.JiraPassword)
			if !isSuccess {
				common.Logger.Error("粘贴jira失败")
				return nil
			}
		}
		// webHook 后置通知
		group, err := groupDao.SelectByPrimaryKey(util.AnyConvertToString(emp.GroupId))
		if err != nil {
			common.Logger.Error(err)
			return nil
		}
		// 组装数据
		jira = JiraURL + "browse/" + jira
		requestParam := make(map[string]string)
		requestParam["module"] = task.Code
		requestParam["changeFiles"] = desc
		requestParam["jira"] = jira
		requestParam["receiverName"] = testerName
		requestParam["receiverCode"] = testerCode
		if task.Type == 1 {
			requestParam["message"] = "帅哥,整包打好咯,取一下"
		} else {
			requestParam["message"] = "帅哥,替换包打好咯,取一下"
		}
		// 消息通道
		// ALL：全部  WEBHOOK：webhook  REBOT：机器人
		if group.Chan == "REBOT" {
			// rebot
			j.sendMessage(requestParam)
		} else if group.Chan == "WEBHOOK" {
			// webhook
			var webHookStr string
			if task.Type == 1 {
				// all
				if group.AllWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.AllWebHook
			} else {
				// replace
				if group.ReplaceWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.ReplaceWebHook
			}
			requestParam["ceshi"] = testerName
			common.Logger.Debugf("webhook request param: %v", requestParam)
			requestParamJson, _ := json.Marshal(requestParam)
			param := bytes.NewBuffer(requestParamJson)
			req, err := http.NewRequest(http.MethodPost, webHookStr, param)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			client := common.HttpClient
			do, err := client.Do(req)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			defer do.Body.Close()
			resp, err := io.ReadAll(do.Body)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			if do.StatusCode >= 200 && do.StatusCode < 400 {
				var c map[string]interface{}
				err := json.Unmarshal(resp, &c)
				if err != nil {
					common.Logger.Error(err)
					return err
				}
				i := c["status"].(float64)
				if int(i) == 1 {
					common.Logger.Info("webhook success")
				} else {
					common.Logger.Errorf("webhook fail:%s", util.AnyConvertToString(c["errorMsg"]))
				}
			}
		} else {
			// all
			j.sendMessage(requestParam)
			var webHookStr string
			if task.Type == 1 {
				// all
				if group.AllWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.AllWebHook
			} else {
				// replace
				if group.ReplaceWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.ReplaceWebHook
			}
			requestParam["ceshi"] = testerName
			common.Logger.Debugf("webhook request param: %v", requestParam)
			requestParamJson, _ := json.Marshal(requestParam)
			param := bytes.NewBuffer(requestParamJson)
			req, err := http.NewRequest(http.MethodPost, webHookStr, param)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			client := common.HttpClient
			do, err := client.Do(req)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			defer do.Body.Close()
			resp, err := io.ReadAll(do.Body)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			if do.StatusCode >= 200 && do.StatusCode < 400 {
				var c map[string]interface{}
				err := json.Unmarshal(resp, &c)
				if err != nil {
					common.Logger.Error(err)
					return err
				}
				i := c["status"].(float64)
				if int(i) == 1 {
					common.Logger.Info("webhook success")
				} else {
					common.Logger.Errorf("webhook fail:%s", util.AnyConvertToString(c["errorMsg"]))
				}
			}
		}
	} else {
		requestParam := make(map[string]string)
		requestParam["jira"] = "http://172.31.3.206:10010/#/home"
		if build.GetResult() == "ABORTED" {
			// 被人中断
			stopMan := j.getStopMan(build.GetConsoleOutput(ctx))
			requestParam["error"] = "任务被 " + stopMan + " 停止了"
		} else {
			// 异常打包错误,获取对应的错误信息
			errMsg := j.getErrorMsg(build.GetConsoleOutput(ctx))
			// 获取打包错误时的
			requestParam["error"] = errMsg
		}
		taskExecute := models.TaskExecute{
			Id:            jobId,
			ExecuteStatus: 3,
			ConsumeTime:   consumeTime,
		}
		err := taskExecuteDao.Update(taskExecute)
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		taskExec, err := taskExecuteDao.SelectByPrimaryKey(util.AnyConvertToString(jobId))
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		task, err := taskDao.SelectByPrimaryKey(util.AnyConvertToString(taskExec.TaskId))
		if err != nil {
			common.Logger.Error(err)
			return err
		}
		// webHook 后置通知
		group, err := groupDao.SelectByPrimaryKey(util.AnyConvertToString(emp.GroupId))
		if err != nil {
			common.Logger.Error(err)
			return nil
		}

		requestParam["module"] = task.Code
		requestParam["changeFiles"] = desc
		requestParam["receiverName"] = emp.ChineseName
		requestParam["receiverCode"] = emp.Username
		if task.Type == 1 {
			requestParam["message"] = "整包打包失败"
		} else {
			requestParam["message"] = "替换打包失败"
		}
		// 消息通道
		// ALL：全部  WEBHOOK：webhook  REBOT：机器人
		if group.Chan == "REBOT" {
			j.sendMessage(requestParam)
		} else if group.Chan == "WEBHOOK" {
			requestParam["yanfa"] = emp.ChineseName
			var webHookStr string
			if task.Type == 1 {
				// all
				if group.AllWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.AllWebHook
			} else {
				// replace
				if group.ReplaceWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.ReplaceWebHook
			}
			common.Logger.Debugf("webhook request param: %v", requestParam)
			requestParamJson, _ := json.Marshal(requestParam)
			param := bytes.NewBuffer(requestParamJson)
			req, err := http.NewRequest(http.MethodPost, webHookStr, param)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			client := common.HttpClient
			do, err := client.Do(req)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			defer do.Body.Close()
			resp, err := io.ReadAll(do.Body)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			if do.StatusCode >= 200 && do.StatusCode < 400 {
				var c map[string]interface{}
				err := json.Unmarshal(resp, &c)
				if err != nil {
					common.Logger.Error(err)
					return err
				}
				i := c["status"].(float64)
				if int(i) == 1 {
					common.Logger.Info("webhook success")
				} else {
					common.Logger.Errorf("webhook fail:%s", util.AnyConvertToString(c["errorMsg"]))
				}
			}
		} else {
			j.sendMessage(requestParam)
			requestParam["yanfa"] = emp.ChineseName
			var webHookStr string
			if task.Type == 1 {
				// all
				if group.AllWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.AllWebHook
			} else {
				// replace
				if group.ReplaceWebHook == "" {
					common.Logger.Warn("未设置webhook")
					return nil
				}
				webHookStr = group.ReplaceWebHook
			}
			common.Logger.Debugf("webhook request param: %v", requestParam)
			requestParamJson, _ := json.Marshal(requestParam)
			param := bytes.NewBuffer(requestParamJson)
			req, err := http.NewRequest(http.MethodPost, webHookStr, param)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			client := common.HttpClient
			do, err := client.Do(req)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			defer do.Body.Close()
			resp, err := io.ReadAll(do.Body)
			if err != nil {
				common.Logger.Error(err)
				return err
			}
			if do.StatusCode >= 200 && do.StatusCode < 400 {
				var c map[string]interface{}
				err := json.Unmarshal(resp, &c)
				if err != nil {
					common.Logger.Error(err)
					return err
				}
				i := c["status"].(float64)
				if int(i) == 1 {
					common.Logger.Info("webhook success")
				} else {
					common.Logger.Errorf("webhook fail:%s", util.AnyConvertToString(c["errorMsg"]))
				}
			}
		}
		return nil
	}
	return nil
}

// 停止构建工作
func (j *JenkinsService) StopJob(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("停止构建接口参数：%v", param)
	empId := util.AnyConvertToString(param["empId"])
	buildId := util.AnyConvertToString(param["buildId"])
	if empId == "" || buildId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工或构建任务id为空"))
		return
	}
	emp, err := empDao.SelectByPrimaryKey(empId)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if emp == (models.EmpInfo{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该员工不存在或已删除"))
		return
	}
	job, err := taskExecuteDao.SelectByBuildId(buildId)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if job == (models.TaskExecute{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该任务不存在或已删除"))
		return
	}
	if job.ExecuteStatus == 2 || job.ExecuteStatus == 3 {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("当前任务已完结"))
		return
	}
	task, err := taskDao.SelectByPrimaryKey(util.AnyConvertToString(job.TaskId))
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if task == (models.TaskInfo{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该任务不存在或已删除"))
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	j.resetConn(emp.JenkinsUsername, emp.JenkinsPassword)
	num, _ := strconv.ParseInt(buildId, 10, 64)
	connInfo := JenkinsConnMap[emp.JenkinsUsername]
	build, err := connInfo.jenConn.GetBuildFromQueueID(connInfo.ctx, num)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	stop, err := build.Stop(connInfo.ctx)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if stop {
		taskExecute := models.TaskExecute{
			Id:            job.Id,
			ExecuteStatus: 3,
			ConsumeTime:   0,
		}
		err := taskExecuteDao.Update(taskExecute)
		if err != nil {
			common.Logger.Error(err)
			c.JSON(http.StatusOK, apiResponse.FailWithMessage("停止失败"))
			return
		}
		c.JSON(http.StatusOK, apiResponse.SuccessDefault())
		return
	} else {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("无法停止"))
		return
	}
}

// 获取企业微信token
// 通常情况下。获取到的token的有效期有2个小时
// 此处在本地设置2个小时的有效期，被动更新
// 设置3次错误重试机会，如果3次获取失败，则提示前端错误信息
// 企业微信可能会出于运营需要，提前使access_token失效，开发者应实现access_token失效时重新获取的逻辑, 所以在发送信息错误时，也会走此逻辑获取token重新发送
func (j *JenkinsService) reSetAccessToken() {
	// https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=ID&corpsecret=SECRET
	getTokenUrl := `https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=wwd503861d87792170&corpsecret=0ZsayyAbhqbMhi1iPgR1obyc-olxOzNV6hb6w5fNEt8`
	req, err := http.NewRequest(http.MethodGet, getTokenUrl, nil)
	if err != nil {
		common.Logger.Error("获取token request错误,", err)
	}
	for i := 0; i < 3; i++ {
		apiResponse, err := common.HttpClient.Do(req)
		if err != nil {
			common.Logger.Errorf("获取企业机器人token第%d次请求失败,", i+1, err)
			continue
		}
		resp, err := io.ReadAll(apiResponse.Body)
		if err != nil {
			common.Logger.Errorf("解析企业机器人token第%d次响应失败,", i+1, err)
			continue
		}
		var c map[string]interface{}
		err = json.Unmarshal(resp, &c)
		if err != nil {
			common.Logger.Errorf("转换企业机器人token第%d次响应失败,", i+1, err)
			continue
		}
		errCode := util.AnyConvertToString(c["errcode"])
		if errCode == "0" {
			// success
			token_str = util.AnyConvertToString(c["access_token"])
			token_init_time = time.Now()
			return
		} else {
			// fail
			common.Logger.Errorf("获取企业机器人token第%d次失败,提示：%s", i+1, util.AnyConvertToString(c["errmsg"]))
			continue
		}

	}
	token_str = ""
}

// 发送企业微信信息
// 避免遭到频率拦截， 此处加互斥锁
func (j *JenkinsService) sendMessage(param map[string]string) {
	// 本地校验token是否过期，如果未过期则尝试发送信息，发送失败会重新获取token补偿发送一次
	// token过期则重新获取token，然后再发送一次
	mutex.Lock()
	defer mutex.Unlock()
	// token通常会有两个小时的有效期，本地校验token是否过期
	if time.Since(token_init_time) > 2*time.Hour {
		j.reSetAccessToken()
		j.sendMessageCore(param)
	} else {
		if err := j.sendMessageCore(param); err != nil {
			j.reSetAccessToken()
			j.sendMessageCore(param)
		}
	}
}

// 发信息核心逻辑
// 发的内容和接收人
func (j *JenkinsService) sendMessageCore(param map[string]string) error {
	common.Logger.Debug("开始发送机器人信息")
	sendUrl := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + token_str
	// 获取接收人id
	toUser := param["receiverCode"]
	userId := ldapService.getUserId(toUser)
	if userId == "" {
		common.Logger.Errorf("获取 %s id失败", toUser)
		return fmt.Errorf("获取 %s id失败", toUser)
	}
	// 封装参数
	paramMap := make(map[string]any)
	paramMap["touser"] = userId
	paramMap["msgtype"] = "markdown"
	paramMap["agentid"] = "1000026"
	mdMap := make(map[string]string)
	mdMap["content"] = fmt.Sprintf(`
> 模块:  %s


> %s
> %s
> %s

[地址](%s)
`, param["module"], param["message"], param["changeFiles"], param["error"], param["jira"])
	paramMap["markdown"] = mdMap
	// 编码
	jsonData, err := json.Marshal(paramMap)
	if err != nil {
		common.Logger.Error("机器人发送信息参数编码失败", err)
		return fmt.Errorf("机器人发送信息参数编码失败")
	}
	req, err := http.NewRequest(http.MethodPost, sendUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		common.Logger.Error("机器人发送信息请求体失败", err)
		return fmt.Errorf("机器人发送信息请求体失败")
	}
	resp, err := common.HttpClient.Do(req)
	if err != nil {
		common.Logger.Error("机器人发送信息响应失败", err)
		return fmt.Errorf("机器人发送信息响应失败")
	}
	defer resp.Body.Close()
	// 解析响应
	var respJson map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&respJson); err != nil {
		common.Logger.Error("机器人解析响应信息失败", err)
		return fmt.Errorf("机器人解析响应信息失败")
	}

	if util.AnyConvertToString(respJson["errcode"]) != "0" {
		common.Logger.Errorf("%v", respJson)
		return fmt.Errorf(util.AnyConvertToString(respJson["errmsg"]))
	}
	return nil
}
