package models

type GroupInfo struct {
	Id             int64  `json:"id,string"`
	Status         int8   `json:"status,string"`
	Name           string `json:"name"`
	AllWebHook     string `json:"awh,omitempty"`
	ReplaceWebHook string `json:"rwh,omitempty"`
	RootName       string `json:"rn,omitempty"`
	Chan           string `json:"channel,omitempty"`
}
