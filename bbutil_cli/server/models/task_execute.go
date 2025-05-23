package models

type TaskExecute struct {
	Id            int64  `json:"id,string,omitempty"`
	Status        int8   `json:"status,string,omitempty"`
	EmpId         int64  `json:"empId,string,omitempty"`
	StartTime     string `json:"startTime,omitempty"`
	ConsumeTime   int64  `json:"consumeTime,string,omitempty" gorm:"default:(-);"`
	TesterName    string `json:"testerName,omitempty" gorm:"default:(-);"`
	TesterCode    string `json:"testerCode,omitempty" gorm:"default:(-);"`
	TaskId        int64  `json:"taskId,string,omitempty"`
	Date          string `json:"date,omitempty"`
	ExecuteStatus int8   `json:"executeStatus,string,omitempty"`
	Jira          string `json:"address,omitempty" gorm:"default:(-);"`
	Desc          string `json:"desc,omitempty" gorm:"default:(-);"`
	BuildId       int64  `json:"buildId,string,omitempty"`
	TaskCode      string `json:"taskCode,omitempty" gorm:"<-:false"`
	TaskValue     string `json:"taskValue,omitempty" gorm:"<-:false"`
	EmpName       string `json:"empName,omitempty" gorm:"<-:false"`
}
