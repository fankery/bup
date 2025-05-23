package service

import (
	"bbutil_cli/common"
	"bbutil_cli/server/models"
	"bbutil_cli/server/util"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/gin-gonic/gin"
	"github.com/trivago/tgo/tcontainer"
)

type JiraService struct{}

// jira 服务器地址
const JiraURL string = "http://172.31.3.252/"

// jira客户端连接信息
type userJiraLinkInfo struct {
	jiraClient *jira.Client
	InitTime   time.Time
	token      string
}

// 项目信息
type projectInfo struct {
	// projectSlice = make([]map[string]string, 8)
	projectSlice []map[string]string
	initTime     time.Time
}

// jira project
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

var (
	// jira连接map
	// 有效期1小时，被动重置
	UserJiraLinkMap = make(map[string]userJiraLinkInfo)
	//项目切片
	projectCache projectInfo
)

// 获取该用户的jira连接
func (j *JiraService) GetJiraClient(jiraUsername, jiraPassword string) *jira.Client {
	jiraPassword, _ = util.AESDecoding(jiraPassword)
	jiraClient, ok := UserJiraLinkMap[jiraUsername]
	if ok {
		//校验是否过期
		if time.Since(jiraClient.InitTime) > 30*time.Minute {

			j.InitJiraLinkCore(jiraUsername, jiraPassword)
		}
	} else {
		j.InitJiraLinkCore(jiraUsername, jiraPassword)
	}
	return UserJiraLinkMap[jiraUsername].jiraClient
}

// 创建用户jira连接核心逻辑
func (j *JiraService) InitJiraLinkCore(jiraUsername, jiraPassword string) {
	//模拟登录
	authURL := JiraURL + "login.jsp"
	sessionId, token := j.getSessionAndToken(jiraUsername, jiraPassword)
	if strings.HasSuffix(token, "out") || sessionId == "" {
		return
	}
	// 创建 CookieAuthTransport 实例
	sessionCookies := []*http.Cookie{
		{Name: "WQJIRASESSIONID", Value: sessionId},
		{Name: "atlassian.xsrf.token", Value: token},
		{Name: "jira.editor.user.mode", Value: "wysiwyg"},
		{Name: "confluence.browse.space.cookie", Value: "space-blogposts"},
		{Name: "mywork.tab.tasks", Value: "false"},
	}
	// 创建一个自定义的Transport
	// proxy := http.ProxyURL(&url.URL{
	// 	Scheme: "http",
	// 	Host:   "127.0.0.1:8888",
	// })

	transport := &jira.CookieAuthTransport{
		Username:      jiraUsername,
		Password:      jiraPassword,
		AuthURL:       authURL,
		SessionObject: sessionCookies,
		Transport:     &http.Transport{
			// Proxy: proxy,
		},
	}
	httpClient := transport.Client()
	jiraClient, err := jira.NewClient(httpClient, JiraURL)
	if err != nil {
		common.Logger.Error(err)
	}
	clientInfo := userJiraLinkInfo{
		jiraClient: jiraClient,
		InitTime:   time.Now(),
		token:      sessionId + token,
	}
	UserJiraLinkMap[jiraUsername] = clientInfo
	common.Logger.Infof("%s jira客户端初始化成功", jiraUsername)
}

// 添加注释
func (j *JiraService) AddCommand(issueKey, comment, jiraUsername, jiraPassword string) bool {
	// 替换为要添加注释的问题的 Key
	// issueKey := "TEST-18512"
	commentBody := jira.Comment{
		Body: comment,
	}
	// 创建注释
	client := j.GetJiraClient(jiraUsername, jiraPassword)
	_, _, err := client.Issue.AddComment(issueKey, &commentBody)
	if err != nil {
		common.Logger.Error("Error adding comment:", err)
		return false
	}
	return true
}

// 创建jira问题
func (j *JiraService) CreateJira(c *gin.Context) {
	// 解析表单数据
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB内存限制
	if err != nil {
		common.Logger.Error("create jira parse request err:", err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	empId := c.PostForm("empId")
	if empId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工id不能为空"))
		return
	}
	summary := c.PostForm("summary")
	if summary == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("概要不能为空"))
		return
	}
	fixVersion := c.PostForm("fixVersion")
	if fixVersion == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("修复版本不能为空"))
		return
	}
	projectId := c.PostForm("projectId")
	if projectId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("项目不能为空"))
		return
	}
	testerName := c.PostForm("testerName")
	if testerName == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("经办人不能为空"))
		return
	}
	component := c.PostForm("component")
	if testerName == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("模块不能为空"))
		return
	}
	deployOrder := c.PostForm("deployOrder")
	if testerName == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("部署顺序不能为空"))
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

	client := j.GetJiraClient(emp.JiraUsername, emp.JiraPassword)
	if client == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("jira登录失败"))
		return
	}
	newIssue := &jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				ID: projectId,
			},
			Type: jira.IssueType{
				ID: "3",
			},
			Summary: summary,

			Assignee: &jira.User{
				Name: testerName,
			},
		},
	}
	newIssue.Fields.FixVersions = append(newIssue.Fields.FixVersions, &jira.FixVersion{ID: fixVersion})
	newIssue.Fields.Components = append(newIssue.Fields.Components, &jira.Component{ID: component})
	fieldMap := make(map[string]string)
	// 允许的值为: 10000(先sql脚本后程序]10001[只有脚本] 10002先程序后脚本],10003只有程序)
	fieldMap["id"] = deployOrder
	newIssue.Fields.Unknowns = tcontainer.MarshalMap{
		"customfield_10300": fieldMap,
	}
	//创建
	createdIssue, _, err := client.Issue.Create(newIssue)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	user := &jira.User{
		Name: testerName,
	}
	client.Issue.UpdateAssignee(createdIssue.ID, user)
	if err != nil {
		common.Logger.Error("Error creating issue:", err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	// 获取所有文件，添加附件
	form := c.Request.MultipartForm
	files := form.File["files"]
	common.Logger.Debugf("检测到%d附件", len(files))
	//遍历所有文件
	for _, file := range files {
		multiFile, err := file.Open()
		if err != nil {
			common.Logger.Error("Error copy attachment,", err)
			c.JSON(http.StatusOK, apiResponse.FailDefault())
			return
		}
		defer multiFile.Close()
		//将文件读取到内存
		fileBytes, err := io.ReadAll(multiFile)
		if err != nil {
			common.Logger.Error("Error parse attachment,", err)
			c.JSON(http.StatusOK, apiResponse.FailDefault())
			return
		}
		buffer := bytes.NewBuffer(fileBytes)
		_, _, err = client.Issue.PostAttachment(createdIssue.ID, buffer, file.Filename)
		if err != nil {
			common.Logger.Errorf("添加附件：%s错误:", file.Filename, err)
			c.JSON(http.StatusOK, apiResponse.FailWithMessage("添加附件："+file.Filename+" 错误"))
			return
		}
	}

	c.JSON(http.StatusOK, apiResponse.SuccessWithData(createdIssue.Key))
}

// 获取项目列表等信息
func (j *JiraService) GetProject(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("获取项目列表等信息接口参数：%v", param)
	empId := util.AnyConvertToString(param["empId"])
	if empId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工id不能为空"))
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
	client := j.GetJiraClient(emp.JiraUsername, emp.JiraPassword)
	if client == nil {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("登录jira失败"))
		return
	}
	if j.dataExistsInTheCache() {
		c.JSON(http.StatusOK, apiResponse.SuccessWithData(projectCache.projectSlice))
		return
	}
	if err != nil {
		common.Logger.Error("request:", err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if client == nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	Projects, _, err := client.Project.GetList()
	if err != nil {
		common.Logger.Error("response:", err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	//返回列表
	var list []map[string]string
	for _, v := range *Projects {
		tempMap := make(map[string]string, 4)
		tempMap["key"] = v.Key
		tempMap["id"] = v.ID
		tempMap["name"] = v.Name
		list = append(list, tempMap)
	}
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(list))
	j.saveDataInTheCache(list)
}

// 获取项目的信息
func (j *JiraService) GetProjectInfo(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("获取项目的修复版本列表 接口参数：%v", param)
	empId := util.AnyConvertToString(param["empId"])
	if empId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工id不能为空"))
		return
	}
	projectId := util.AnyConvertToString(param["projectId"])
	if projectId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("项目id不能为空"))
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
	client := j.GetJiraClient(emp.JiraUsername, emp.JiraPassword)
	project, _, err := client.Project.Get(projectId)
	if err != nil {
		common.Logger.Error("Error getting project information: %s\n", err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	sort.Slice(project.Versions, func(i, j int) bool {
		return project.Versions[i].ID > project.Versions[j].ID
	})
	//返回列表
	var versionlist []map[string]string
	for _, v := range project.Versions {
		if !*v.Released {
			tempMap := make(map[string]string, 4)
			tempMap["id"] = v.ID
			tempMap["name"] = v.Name
			versionlist = append(versionlist, tempMap)
		}
	}

	var componentslist []map[string]string
	for _, v := range project.Components {
		tempMap := make(map[string]string, 4)
		tempMap["id"] = v.ID
		tempMap["name"] = v.Name
		componentslist = append(componentslist, tempMap)
	}

	resultMap := make(map[string]any, 4)
	resultMap["versions"] = versionlist
	resultMap["components"] = componentslist

	c.JSON(http.StatusOK, apiResponse.SuccessWithData(resultMap))
}

// 缓存项目列表逻辑
func (j *JiraService) dataExistsInTheCache() bool {
	if projectCache.initTime == (time.Time{}) {
		return false
	} else {
		if time.Since(projectCache.initTime) > 2*time.Hour {
			return false
		}
	}
	return true
}

// 刷新缓存逻辑
func (j *JiraService) saveDataInTheCache(projectData []map[string]string) {
	projectCache = projectInfo{
		initTime:     time.Now(),
		projectSlice: projectData,
	}
}

// get sessionId and token
func (j *JiraService) getSessionAndToken(jiraUsername, jiraPassword string) (string, string) {
	var sessionId, token string
	//模拟登录jira，拿到cookie和sessionId
	targetUrl := "http://172.31.3.252/login.jsp"
	req, err := http.NewRequest(http.MethodGet, targetUrl, nil)
	if err != nil {
		return sessionId, token
	}
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Add("Cache-Control", "max-age=0")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Host", "172.31.3.252")
	req.Header.Add("Origin", "http://172.31.3.252")
	req.Header.Add("Referer", "http://172.31.3.252/login.jsp")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return sessionId, token
	}
	defer resp.Body.Close()
	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "WQJIRASESSIONID" {
			sessionId = cookie.Value
		}
		if cookie.Name == "atlassian.xsrf.token" {
			token = cookie.Value
		}
	}
	cookieStr := fmt.Sprintf("WQJIRASESSIONID=%s; atlassian.xsrf.token=%s;", sessionId, token)
	//println(sessionId + token)
	//模拟登录jira，拿到cookie和sessionId
	payload := url.Values{"os_username": {jiraUsername},
		"os_password":    {jiraPassword},
		"os_destination": {""},
		"user_role":      {""},
		"atl_token":      {""},
		"login":          {"Log In"}}
	body := strings.NewReader(payload.Encode())
	req, err = http.NewRequest(http.MethodPost, targetUrl, body)
	if err != nil {
		return sessionId, token
	}
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Add("Cache-Control", "max-age=0")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Host", "172.31.3.252")
	req.Header.Add("Origin", "http://172.31.3.252")
	req.Header.Add("Referer", "http://172.31.3.252/login.jsp")
	req.Header.Add("Cookie", cookieStr)
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil {
		return sessionId, token
	}
	defer resp.Body.Close()
	cookies = resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "WQJIRASESSIONID" {
			sessionId = cookie.Value
		}
		if cookie.Name == "atlassian.xsrf.token" {
			token = cookie.Value
		}
	}

	cookieStr = fmt.Sprintf("WQJIRASESSIONID=%s; atlassian.xsrf.token=%s;", sessionId, token)
	//println(sessionId + token)
	//拿token
	getRequest, err := http.NewRequest(http.MethodGet, "http://172.31.3.252/", nil)
	if err != nil {
		common.Logger.Error("获取token失败")
		return sessionId, token
	}
	getRequest.Header.Add("Accept", " text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	getRequest.Header.Add("Accept-Encoding", "gzip, deflate")
	getRequest.Header.Add("Accept-Language", "zh-CN,zh;q=0.9")
	getRequest.Header.Add("Cache-Control", "max-age=0")
	getRequest.Header.Add("Connection", "keep-alive")
	getRequest.Header.Add("Cookie", cookieStr)
	getRequest.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	getRequest.Header.Add("Referer", "http://172.31.3.252/login.jsp")
	getRequest.Header.Add("Upgrade-Insecure-Requests", "1")
	getRequest.Header.Add("Host", "172.31.3.252")
	response, err := client.Do(getRequest)
	if err != nil {
		common.Logger.Error("获取token失败")
		return sessionId, token
	}
	defer response.Body.Close()
	cookies = response.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "WQJIRASESSIONID" {
			sessionId = cookie.Value
		}
		if cookie.Name == "atlassian.xsrf.token" {
			token = cookie.Value
		}
	}
	return sessionId, token
}

func (j *JiraService) GetTesterInfo(c *gin.Context) {
	data, _ := c.GetRawData()
	var param map[string]any
	_ = json.Unmarshal(data, &param)
	common.Logger.Infof("获取jira 接口参数：%v", param)
	empId := util.AnyConvertToString(param["empId"])
	if empId == "" {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("员工id不能为空"))
		return
	}
	tester := util.AnyConvertToString(param["tester"])
	emp, err := empDao.SelectByPrimaryKey(empId)
	if err != nil {
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	if emp == (models.EmpInfo{}) {
		c.JSON(http.StatusOK, apiResponse.FailWithMessage("该员工不存在或已删除"))
		return
	}
	client := j.GetJiraClient(emp.JenkinsUsername, emp.JenkinsPassword)

	withUserName := jira.WithUsername(tester)
	withMaxResult := jira.WithMaxResults(50)
	withActive := jira.WithActive(true)
	withStartAt := jira.WithStartAt(0)
	withProperty := jira.WithProperty("displayName")
	users, _, err := client.User.Find("username", withActive, withUserName, withMaxResult, withStartAt, withProperty)
	if err != nil {
		common.Logger.Error(err)
		c.JSON(http.StatusOK, apiResponse.FailDefault())
		return
	}
	var list []map[string]string
	for _, v := range users {
		tempMap := make(map[string]string, 2)
		tempMap["username"] = v.Name
		tempMap["displayName"] = v.DisplayName
		list = append(list, tempMap)
	}
	c.JSON(http.StatusOK, apiResponse.SuccessWithData(list))
}
