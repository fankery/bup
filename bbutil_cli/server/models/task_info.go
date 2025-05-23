package models

type TaskInfo struct {
	Id      int64  `json:"id,string,omitempty"`
	Status  int8   `json:"status,string,omitempty"`
	Code    string `json:"code,omitempty" gorm:"default:(-);"`
	Value   string `json:"value,omitempty"`
	GroupId int64  `json:"groupId,string,omitempty"`
	Type    int64  `json:"type,omitempty"`
	Jira    string `json:"jira,omitempty" gorm:"default:(-);"`
	Pack    string `json:"pack,omitempty"`
}
