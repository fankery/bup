package models

type EmpInfo struct {
	Id              int64  `json:"id,string,omitempty"`
	Status          int8   `json:"status,string,omitempty"`
	ChineseName     string `json:"chineseName,omitempty" gorm:"default:(-);"`
	Username        string `json:"username,omitempty"`
	JenkinsUsername string `json:"jenkinsUsername,omitempty"  gorm:"default:(-);"`
	JenkinsPassword string `json:"-" gorm:"default:(-);"`
	JiraUsername    string `json:"jiraUsername,omitempty" gorm:"default:(-);"`
	JiraPassword    string `json:"-" gorm:"default:(-);"`
	GroupId         int64  `json:"groupId,string,omitempty"`
	GroupName       string `json:"groupName,omitempty" gorm:"<-:false"`
	Address         string `json:"address,omitempty"`
}
